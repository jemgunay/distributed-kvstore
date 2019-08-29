package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/jemgunay/distributed-kvstore/server/store"
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
	if err := server.Start(port); err != nil {
		fmt.Printf("server has shut down unexpectedly: %s", err)
		return
	}
	fmt.Printf("server has shut down")
}

// KVSyncServer is a gRPC KV synchronised server which satisfies both the KVServiceServer and SyncServiceServer interfaces.
type KVSyncServer struct {
	grpcServer *grpc.Server
	store      store.KVStorer
}

// NewKVSyncServer creates a new gRPC KV synchronised server.
func NewKVSyncServer(store store.KVStorer) *KVSyncServer {
	return &KVSyncServer{
		grpcServer: grpc.NewServer(),
		store:      store,
	}
}

// Start registers the gRPC handlers and starts the KV sync server.
func (s *KVSyncServer) Start(port int) error {
	if s.store == nil {
		return errors.New("server store is uninitialised")
	}

	// register gRPC handlers
	pb.RegisterKVStoreServer(s.grpcServer, s)
	pb.RegisterSyncServer(s.grpcServer, s)

	// bind to specified port over TCP
	l, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return fmt.Errorf("failed to listen: %s", err)
	}

	// start serving via gRPC
	return s.grpcServer.Serve(l)
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

// Sync is responsible for transmitting sync requests to the other distributed store nodes.
func (s *KVSyncServer) Sync(stream pb.Sync_SyncServer) error {
	return nil
}
