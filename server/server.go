package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"sort"
	"strconv"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	pb "github.com/jemgunay/distributed-kvstore/proto"
	"github.com/jemgunay/distributed-kvstore/server/store"
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
	flag.IntVar(&port, "port", port, "the port this server should serve from")
	flag.Var(&nodesAddresses, "node_address", "list of node addresses that this node should attempt to synchronise with")
	flag.Parse()

	// create a store and server
	kvStore := store.NewStore()
	kvStore.StartPoller()
	server := NewKVSyncServer(kvStore, kvStore)

	// start serving
	log.Printf("server listening on port %d", port)
	if err := server.Start(":"+strconv.Itoa(port), nodesAddresses); err != nil {
		fmt.Printf("server has shut down unexpectedly: %s", err)
		return
	}
	fmt.Printf("server has shut down")
}

// Node represents a single service node in the distributed store network.
type Node struct {
	address   string
	startTime int64
	id        uint32

	*grpc.ClientConn
	Client          pb.SyncClient
	syncRequestChan chan *pb.SyncRequest
}

type SyncSourcer interface {
	SyncPoll() *pb.SyncRequest
}

// KVSyncServer is a gRPC KV synchronised server which satisfies both the KVServiceServer and SyncServiceServer
// interfaces.
type KVSyncServer struct {
	grpcServer  *grpc.Server
	store       store.KVStorer
	syncSourcer SyncSourcer
	nodes       map[uint32]*Node

	startTime           int64
	id                  uint32
	serverShutdownChan  chan error
	syncRequestChanSize int
}

// NewKVSyncServer creates a new gRPC KV synchronised server.
func NewKVSyncServer(store store.KVStorer, syncSource SyncSourcer) *KVSyncServer {
	return &KVSyncServer{
		grpcServer:          grpc.NewServer(),
		store:               store,
		syncSourcer:         syncSource,
		serverShutdownChan:  make(chan error),
		syncRequestChanSize: 1 << 10, // 1024
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

	for id, node := range s.nodes {
		log.Printf("[node %d on %s] node %d on \"%s\" started at %d", s.id, address, id, node.address, node.startTime)

		go s.syncPollNode(node)
	}

	// feed each node with new sync requests
	go func() {
		for {
			syncReq := s.syncSourcer.SyncPoll()
			for _, node := range s.nodes {
				node.syncRequestChan <- syncReq
			}
		}
	}()

	log.Println()

	//s.startPoller()

	return <-s.serverShutdownChan
}

func (s *KVSyncServer) identifyNodes(nodeAddresses []string) error {
	// create temporary slice of nodes, including self, in order to derive node IDs from order of startup
	discoveredNodes := make([]*Node, 0, len(nodeAddresses)+1)
	eg := errgroup.Group{}

	// TODO: ping until service is up (with timeout), then perform identification
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
			resp, err := client.Identify(ctx, &pb.IdentifyMessage{StartTime: s.startTime})
			if err != nil {
				return fmt.Errorf("failed to identify node: %s", err)
			}

			// create new node
			newNode := &Node{
				address:   addr,
				startTime: resp.StartTime,

				ClientConn:      conn,
				Client:          client,
				syncRequestChan: make(chan *pb.SyncRequest, s.syncRequestChanSize),
			}
			discoveredNodes = append(discoveredNodes, newNode)

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to identify all nodes: %s", err)
	}

	// add in self node in order to determine node IDs
	discoveredNodes = append(discoveredNodes, &Node{startTime: s.startTime})

	// sort by timestamp
	sort.Slice(discoveredNodes, func(i, j int) bool {
		return discoveredNodes[i].startTime < discoveredNodes[j].startTime
	})

	// use index in ordered slice as the node ID - each node will derive the same IDs from this process independently
	s.nodes = make(map[uint32]*Node, len(discoveredNodes)-1)
	for id, n := range discoveredNodes {
		if n.startTime == s.startTime {
			// we don't want to add self node to node store, but store its ID
			s.id = uint32(id)
			continue
		}
		n.id = uint32(id)
		s.nodes[n.id] = n
	}

	return nil
}

func (s *KVSyncServer) syncPollNode(node *Node) {
	// for each node, create long polling synchronised connection
	ctx, cancel := context.WithTimeout(context.Background(), clientTimeout)
	defer cancel()
	syncStream, err := node.Client.Sync(ctx)
	if err != nil {
		log.Printf("failed to open sync stream with node: %s", err)
		return
	}

	eg := errgroup.Group{}
	// sending poller
	eg.Go(func() error {
		for req := range node.syncRequestChan {
			if err := syncStream.Send(req); err != nil {
				return err
			}
		}
		return nil
	})

	// receiving poller
	/*eg.Go(func() error {

	})*/
	eg.Wait()
}

// Shutdown gracefully shuts down the gRPC server.
func (s *KVSyncServer) Shutdown() {
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

// Identify processes identify requests used to initially ID a node on startup based on the startup timestamps of all
// nodes.
func (s *KVSyncServer) Identify(ctx context.Context, r *pb.IdentifyMessage) (*pb.IdentifyMessage, error) {
	if p, ok := peer.FromContext(ctx); ok {
		log.Printf("[%s -> identify]", p.Addr)
	}
	// TODO: consume the r.StartTime here to create a node and reduce need to ping the requesting server again to
	// exponentially reduce number of pings required in identification stage

	return &pb.IdentifyMessage{StartTime: s.startTime}, nil
}

// Sync is responsible for transmitting sync requests to the other distributed store nodes.
func (s *KVSyncServer) Sync(stream pb.Sync_SyncServer) error {
	for {
		_, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		fmt.Println("sync request in")
	}
	fmt.Println("sync connection lost")
	return nil
}
