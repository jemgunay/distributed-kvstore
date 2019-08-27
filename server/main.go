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

	// create gRPC server
	grpcServer := grpc.NewServer()
	pb.RegisterKVServiceServer(grpcServer, &server{})

	l, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		fmt.Printf("failed to listen: %s", err)
		return
	}

	// start serving over gRPC
	log.Printf("server listening on port %d", port)
	if err = grpcServer.Serve(l); err != nil {
		fmt.Printf("server has shut down: %s", err)
	}
}

type server struct{}

func (s *server) Publish(ctx context.Context, r *pb.PublishRequest) (*pb.Empty, error) {
	if p, ok := peer.FromContext(ctx); ok {
		log.Printf("[%s -> publish] %s", p.Addr, r.Key)
	}

	// insert record into store
	err := store.Put(r.GetKey(), r.GetValue())
	return &pb.Empty{}, err
}

func (s *server) Fetch(ctx context.Context, r *pb.FetchRequest) (*pb.FetchResponse, error) {
	if p, ok := peer.FromContext(ctx); ok {
		log.Printf("[%s -> fetch] %s", p.Addr, r.Key)
	}

	var (
		resp = &pb.FetchResponse{}
		err  error
	)

	// pull record from store
	resp.Value, resp.Timestamp, err = store.Get(r.GetKey())
	return resp, err
}
