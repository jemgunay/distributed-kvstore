package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"time"

	"github.com/jemgunay/distributed-kvstore/server/store"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	pb "github.com/jemgunay/distributed-kvstore/proto"
)

// multiFlag satisfies the Value interface in order to parse multiple command line arguments of the same name into a
// slice.
type multiFlag []string

func (m *multiFlag) String() string {
	return fmt.Sprintf("%+v", *m)
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

var (
	port           = 6000
	clientTimeout  = time.Second * 10
	nodesAddresses multiFlag
)

func main() {
	// parse flags
	flag.IntVar(&port, "port", port, "the port this server should server from")
	flag.Var(&nodesAddresses, "node_address", "list of node addresses that this node should attempt to synchronise with")
	flag.Parse()

	// create a store and server
	kvStore := store.NewStore()
	kvStore.StartPoller()
	server := NewKVSyncServer(kvStore)

	// start serving
	log.Printf("server listening on port %d", port)
	if err := server.Start(":"+strconv.Itoa(port), nodesAddresses); err != nil {
		fmt.Printf("server has shut down unexpectedly: %s", err)
		return
	}
	fmt.Printf("server has shut down")
}

type Node struct {
	address   string
	startTime int64

	*grpc.ClientConn
	Client pb.SyncClient
}

// KVSyncServer is a gRPC KV synchronised server which satisfies both the KVServiceServer and SyncServiceServer
// interfaces.
type KVSyncServer struct {
	grpcServer *grpc.Server
	store      store.KVStorer
	nodes      []*Node

	startTime          int64
	serverShutdownChan chan error
}

// NewKVSyncServer creates a new gRPC KV synchronised server.
func NewKVSyncServer(store store.KVStorer) *KVSyncServer {
	return &KVSyncServer{
		grpcServer:         grpc.NewServer(),
		store:              store,
		serverShutdownChan: make(chan error),
	}
}

// Start registers the gRPC handlers and starts the KV sync server.
func (s *KVSyncServer) Start(address string, nodeAddresses []string) error {
	if s.store == nil {
		return errors.New("server store is uninitialised")
	}

	s.startTime = time.Now().UTC().UnixNano()

	// register gRPC handlers
	pb.RegisterKVStoreServer(s.grpcServer, s)
	pb.RegisterSyncServer(s.grpcServer, s)

	// bind to specified port over TCP
	l, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen: %s", err)
	}

	// start serving via gRPC
	go func() {
		// blocks and returns error on shutdown
		s.serverShutdownChan <- s.grpcServer.Serve(l)
	}()

	// identify the other nodes to sync with
	if err := s.identifyNodes(nodeAddresses); err != nil {
		return err
	}

	for _, v := range s.nodes {
		log.Printf("[%s] \"%s\" @ %d", address, v.address, v.startTime)
	}

	s.startPoller()

	return <-s.serverShutdownChan
}

func (s *KVSyncServer) identifyNodes(nodeAddresses []string) error {
	eg := errgroup.Group{}

	time.Sleep(time.Second * 2)

	for _, addr := range nodeAddresses {
		addr := addr

		eg.Go(func() error {
			// create client to connect to the node
			conn, err := grpc.Dial(addr, grpc.WithInsecure())
			if err != nil {
				return fmt.Errorf("failed to connect to server on \"%s\": %s", addr, err)
			}

			client := pb.NewSyncClient(conn)

			// fetch timestamp from node so that IDs can be determined from startup order
			ctx, cancel := context.WithTimeout(context.Background(), clientTimeout)
			defer cancel()
			resp, err := client.Identify(ctx, &pb.IdentifyMessage{Timestamp: s.startTime})
			if err != nil {
				return fmt.Errorf("failed to identify: %s", err)
			}

			// create new node
			newNode := &Node{
				address:   addr,
				startTime: resp.Timestamp,

				ClientConn: conn,
				Client:     client,
			}
			s.nodes = append(s.nodes, newNode)

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to identify all nodes: %s", err)
	}

	// sort by timestamp
	sort.Slice(s.nodes, func(i, j int) bool {
		return s.nodes[i].startTime < s.nodes[j].startTime
	})

	return nil
}

func (s *KVSyncServer) startPoller() {
	go func() {

	}()
}

// Shutdown gracefully shuts down the gRPC server.
func (s *KVSyncServer) Shutdown(port int) {
	s.grpcServer.GracefulStop()
}

// Publish processes publish requests from a client and stores the provided key/value pair.
func (s *KVSyncServer) Publish(ctx context.Context, r *pb.PublishRequest) (*pb.Empty, error) {
	if p, ok := peer.FromContext(ctx); ok {
		log.Printf("[%s -> publish] %s", p.Addr, r.Key)
	}

	// insert record into store
	err := s.store.Put(r.GetKey(), r.GetValue())
	return &pb.Empty{}, err
}

// Fetch processes fetch requests from a client and returns the value and timestamp associated with the specified key.
func (s *KVSyncServer) Fetch(ctx context.Context, r *pb.FetchRequest) (*pb.FetchResponse, error) {
	if p, ok := peer.FromContext(ctx); ok {
		log.Printf("[%s -> fetch] %s", p.Addr, r.Key)
	}

	var (
		resp = &pb.FetchResponse{}
		err  error
	)

	// pull record from store
	resp.Value, resp.Timestamp, err = s.store.Get(r.GetKey())
	return resp, err
}

// Delete processes delete requests from a client returns the value and timestamp associated with the specified key.
func (s *KVSyncServer) Delete(ctx context.Context, r *pb.DeleteRequest) (*pb.Empty, error) {
	if p, ok := peer.FromContext(ctx); ok {
		log.Printf("[%s -> delete] %s", p.Addr, r.Key)
	}

	// pull record from store
	err := s.store.Delete(r.GetKey())
	return &pb.Empty{}, err
}

func (s *KVSyncServer) Identify(ctx context.Context, r *pb.IdentifyMessage) (*pb.IdentifyMessage, error) {
	if p, ok := peer.FromContext(ctx); ok {
		log.Printf("[%s -> identify]", p.Addr)
	}

	return &pb.IdentifyMessage{Timestamp: s.startTime}, nil
}

// Sync is responsible for transmitting sync requests to the other distributed store nodes.
func (s *KVSyncServer) Sync(stream pb.Sync_SyncServer) error {

	return nil
}
