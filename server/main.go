package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

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

	if err = grpcServer.Serve(l); err != nil {
		fmt.Printf("server has shut down: %s", err)
	}
}

type server struct{}

func (s *server) Publish(ctx context.Context, r *pb.PublishRequest) (*pb.Empty, error) {
	if p, ok := peer.FromContext(ctx); ok {
		log.Printf("client %s requested fetch for %s", p.Addr, r.Key)
	}

	store.

	return nil, nil
}

func (s *server) Fetch(ctx context.Context, r *pb.FetchRequest) (*pb.FetchResponse, error) {
	if p, ok := peer.FromContext(ctx); ok {
		log.Printf("client %s requested fetch for %s", p.Addr, r.Key)
	}

	items, ok := topics[r.Name]
	if !ok {
		return fmt.Errorf("%s is not a supported topic", r.Name)
	}

	for _, item := range items {
		msg := &pb.Message{
			Msg:       item,
			Timestamp: time.Now().String(),
		}

		if err := stream.Send(msg); err != nil {
			return err
		}
		time.Sleep(time.Millisecond * 500)
	}
	return nil
}
