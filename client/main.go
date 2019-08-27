package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"strconv"
	"time"

	"google.golang.org/grpc"

	pb "github.com/jemgunay/distributed-kvstore/proto"
)

var port = 6000

func main() {
	// parse flags
	flag.IntVar(&port, "port", port, "the target server's port")
	flag.Parse()

	// connect to gRPC server
	conn, err := grpc.Dial(":"+strconv.Itoa(port), grpc.WithInsecure())
	if err != nil {
		fmt.Printf("failed to connect to server: %s", err)
		return
	}
	defer conn.Close()

	client := pb.NewKVServiceClient(conn)
	publish(client, "animals", []string{"dog", "cat", "hippo"})
	publish(client, "countries", "this is a chunky piece of country data")

	var animals []string
	fetch(client, "animals", &animals)
	fmt.Printf("fetched result for animals: %v", animals)
}

func publish(client pb.KVServiceClient, key string, value interface{}) error {
	log.Printf("Publishing key: %s, value: %+v", key, value)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	buf := &bytes.Buffer{}
	encoder := gob.NewEncoder(buf)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("failed to encode value: %s", err)
	}

	req := pb.PublishRequest{
		Key:       key,
		Value:     buf.Bytes(),
		Timestamp: time.Now().UTC().UnixNano(),
	}

	if _, err := client.Publish(ctx, &req); err != nil {
		return fmt.Errorf("failed to publish: %s", err)
	}

	return nil
}

func fetch(client pb.KVServiceClient, key string, data interface{}) (int64, error) {
	log.Printf("Fetching value for key: %s", key)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := pb.FetchRequest{
		Key: key,
	}

	resp, err := client.Fetch(ctx, &req)
	if err != nil {
		return 0, fmt.Errorf("failed to publish: %s", err)
	}

	buf := bytes.NewReader(resp.Value)
	decoder := gob.NewDecoder(buf)
	if err := decoder.Decode(data); err != nil {
		return 0, fmt.Errorf("failed to decode value: %s", err)
	}

	return resp.Timestamp, nil
}
