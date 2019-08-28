package main

import (
	"context"
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

var port = 6000

func main() {
	// parse flags
	flag.IntVar(&port, "port", port, "the target server's port")
	flag.Parse()

	// create a server
	server := NewKVSyncServer()

	// start serving over gRPC
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
	store      *store.Store
}

// NewKVSyncServer creates a new gRPC KV synchronised server.
func NewKVSyncServer() *KVSyncServer {
	return &KVSyncServer{
		grpcServer: grpc.NewServer(),
		store:      store.NewStore(),
	}
}

// Start registers the gRPC handlers and starts the KV sync server.
func (s *KVSyncServer) Start(port int) error {
	// start the store's operation poller
	s.store.StartPoller()

	// register gRPC handlers
	pb.RegisterKVStoreServer(s.grpcServer, s)
	//pb.RegisterSyncServer(s.grpcServer, s)

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
