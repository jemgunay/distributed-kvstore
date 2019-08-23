package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"

	pb "github.com/jemgunay/grpc-test/pubsub"
)

type server struct{}

func main() {
	grpcServer := grpc.NewServer()
	pb.RegisterPublisherServiceServer(grpcServer, &server{})

	l, err := net.Listen("tcp", ":6000")
	if err != nil {
		fmt.Printf("failed to listen: %s", err)
		return
	}

	if err = grpcServer.Serve(l); err != nil {
		fmt.Printf("server has shut down: %s", err)
	}
}

var topics = map[string][]string{
	"animals":   {"cat", "dog", "giraffe", "hippo", "monkey", "tiger"},
	"countries": {"Brazil", "Spain", "France", "Turkey", "Italy", "Japan"},
}

func (s *server) Subscribe(p *pb.Topic, stream pb.PublisherService_SubscribeServer) error {
	log.Printf("a client subscribed to %s", p.Name)

	items, ok := topics[p.Name]
	if !ok {
		return fmt.Errorf("%s is not a supported topic", p.Name)
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
