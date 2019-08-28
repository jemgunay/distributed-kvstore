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
	log.Printf("connecting to server on port %d", port)
	client, err := NewClient(port)
	if err != nil {
		log.Printf("failed to create client: %s", err)
		return
	}
	defer client.Close()

	// publish some records
	if err := client.Publish("animals", []string{"dog", "cat", "hippo"}); err != nil {
		log.Printf("failed to publish: %s", err)
		return
	}
	if err := client.Publish("misc_data", "this is a chunky piece of random data"); err != nil {
		log.Printf("failed to publish: %s", err)
		return
	}
	if err := client.Publish("animals", []string{"dog", "cat", "hippo", "tiger", "zebra"}); err != nil {
		log.Printf("failed to publish: %s", err)
		return
	}

	// retrieve an existing record
	var animals []string
	ts, err := client.Fetch("animals", &animals)
	if err != nil {
		log.Printf("failed to fetch: %s", err)
		return
	}
	log.Printf("fetched animals: %v (created at %d)", animals, ts)
}

// KVClient is a gRPC client.
type KVClient struct {
	*grpc.ClientConn
	ServiceClient pb.KVServiceClient
}

// NewClient creates a new gRPC client.
func NewClient(port int) (*KVClient, error) {
	conn, err := grpc.Dial(":"+strconv.Itoa(port), grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %s", err)
	}

	return &KVClient{
		ClientConn:    conn,
		ServiceClient: pb.NewKVServiceClient(conn),
	}, nil
}

// Publish performs a publish request over gRPC in order to publish a key/value pair.
func (c *KVClient) Publish(key string, value interface{}) error {
	log.Printf("[publish] %s -> %+v", key, value)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// gob encode value into bytes
	buf := &bytes.Buffer{}
	encoder := gob.NewEncoder(buf)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("failed to gob encode value: %s", err)
	}

	req := pb.PublishRequest{
		Key:   key,
		Value: buf.Bytes(),
	}

	// perform publish request
	if _, err := c.ServiceClient.Publish(ctx, &req); err != nil {
		return fmt.Errorf("failed to publish: %s", err)
	}

	return nil
}

// Fetch performs a fetch request over gRPC in order to retrieve the value that corresponds with the specified key.
func (c *KVClient) Fetch(key string, value interface{}) (int64, error) {
	log.Printf("[fetch] %s", key)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := pb.FetchRequest{
		Key: key,
	}

	// perform fetch request
	resp, err := c.ServiceClient.Fetch(ctx, &req)
	if err != nil {
		return 0, fmt.Errorf("failed to publish: %s", err)
	}

	// gob decode bytes into specified type
	buf := bytes.NewReader(resp.Value)
	decoder := gob.NewDecoder(buf)
	if err := decoder.Decode(value); err != nil {
		return 0, fmt.Errorf("failed to gob decode value: %s", err)
	}

	return resp.Timestamp, nil
}
