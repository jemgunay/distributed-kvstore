// Package client implements a gRPC KV store client.
package client

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"

	pb "github.com/jemgunay/distributed-kvstore/proto"
)

// KVClient is a gRPC KV client which satisfies the KVServiceClient interface.
type KVClient struct {
	*grpc.ClientConn
	ServiceClient pb.KVStoreClient
	Timeout       time.Duration
	DebugLog      bool
}

// NewKVClient creates a new gRPC KV client.
func NewKVClient(address string) (*KVClient, error) {
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %s", err)
	}

	return &KVClient{
		ClientConn:    conn,
		ServiceClient: pb.NewKVStoreClient(conn),
		Timeout:       time.Second * 10,
	}, nil
}

// Publish performs a publish request over gRPC in order to publish a key/value pair.
func (c *KVClient) Publish(key string, value interface{}) error {
	c.Printf("[publish] %s -> %+v", key, value)

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
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
	c.Printf("[fetch] %s", key)

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	req := pb.FetchRequest{
		Key: key,
	}

	// perform fetch request
	resp, err := c.ServiceClient.Fetch(ctx, &req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch: %s", err)
	}

	// gob decode bytes into specified type
	buf := bytes.NewReader(resp.Value)
	decoder := gob.NewDecoder(buf)
	if err := decoder.Decode(value); err != nil {
		return 0, fmt.Errorf("failed to gob decode value: %s", err)
	}

	return resp.Timestamp, nil
}

// Delete performs a delete request over gRPC in order to delete the record that corresponds with the specified key.
func (c *KVClient) Delete(key string) error {
	c.Printf("[delete] %s", key)

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	req := pb.DeleteRequest{
		Key: key,
	}

	// perform fetch request
	if _, err := c.ServiceClient.Delete(ctx, &req); err != nil {
		return fmt.Errorf("failed to delete: %s", err)
	}

	return nil
}

func (c *KVClient) Subscribe(key string) (chan *pb.FetchResponse, error) {
	c.Printf("[subscribe] %s", key)

	stream, err := c.ServiceClient.Subscribe(context.Background(), &pb.FetchRequest{Key: key})
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to %s: %s", key, err)
	}

	ch := make(chan *pb.FetchResponse)
	go func() {
		defer c.Printf("subscription to %s closed", key)

		for {
			item, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				c.Printf("failed to read from %s subscription: %s", key, err)
				close(ch)
				return
			}

			ch <- item
		}
	}()
	return ch, nil
}

// Printf wraps log.Printf() to only write logs if logging is enabled.
func (c *KVClient) Printf(format string, v ...interface{}) {
	if !c.DebugLog {
		return
	}
	log.Printf(format, v...)
}
