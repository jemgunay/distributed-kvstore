package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"

	pb "github.com/jemgunay/grpc-test/pubsub"
)

func main() {
	conn, err := grpc.Dial("localhost:6000", grpc.WithInsecure())
	if err != nil {
		fmt.Printf("failed to connect to server: %s", err)
		return
	}
	defer conn.Close()

	client := pb.NewPublisherServiceClient(conn)
	subscribeToTopic(client, "animals")
	subscribeToTopic(client, "countries")
}

func subscribeToTopic(client pb.PublisherServiceClient, topic string) {
	log.Printf("Subscribing to %s", topic)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := client.Subscribe(ctx, &pb.Topic{Name: topic})
	if err != nil {
		log.Printf("failed to subscribe to %s: %s,", topic, err)
		return
	}

	for {
		item, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("failed to read from %s subscription: %s", topic, err)
			return
		}
		log.Printf("[%s]: %s", topic, item)
	}
}
