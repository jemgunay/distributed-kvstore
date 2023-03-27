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
	"google.golang.org/grpc/credentials/insecure"

	"github.com/jemgunay/distributed-kvstore/pkg/core"
	pb "github.com/jemgunay/distributed-kvstore/pkg/proto"
)

var _ core.Client = (*Client)(nil)

// Client is a gRPC KV client.
type Client struct {
	*grpc.ClientConn
	ServiceClient pb.KVStoreClient
	Timeout       time.Duration
	DebugLog      bool
	Logger        func(format string, v ...any)
}

// NewClient creates a new gRPC KV client.
func NewClient(address string) (*Client, error) {
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	return &Client{
		ClientConn:    conn,
		ServiceClient: pb.NewKVStoreClient(conn),
		Timeout:       time.Second * 10,
		Logger:        log.Printf,
	}, nil
}

// Publish performs a publish request over gRPC in order to publish a key/value pair.
func (c *Client) Publish(key string, value any) error {
	c.debugf("[publish] %s -> %#v", key, value)

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	// gob encode value into bytes
	buf := &bytes.Buffer{}
	encoder := gob.NewEncoder(buf)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("failed to gob encode value: %w", err)
	}

	req := pb.PublishRequest{
		Key:   key,
		Value: buf.Bytes(),
	}

	// perform publish request
	if _, err := c.ServiceClient.Publish(ctx, &req); err != nil {
		return fmt.Errorf("failed to publish: %w", err)
	}

	return nil
}

// Fetch performs a fetch request over gRPC in order to retrieve the value that corresponds with the specified key.
func (c *Client) Fetch(key string, value any) (int64, error) {
	c.debugf("[fetch] %s", key)

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	req := pb.FetchRequest{
		Key: key,
	}

	// perform fetch request
	resp, err := c.ServiceClient.Fetch(ctx, &req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch: %w", err)
	}

	// gob decode bytes into specified type
	buf := bytes.NewReader(resp.Value)
	decoder := gob.NewDecoder(buf)
	if err := decoder.Decode(value); err != nil {
		return 0, fmt.Errorf("failed to gob decode value: %w", err)
	}

	return resp.Timestamp, nil
}

// Delete performs a delete request over gRPC in order to delete the record that corresponds with the specified key.
func (c *Client) Delete(key string) error {
	c.debugf("[delete] %s", key)

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	req := pb.DeleteRequest{
		Key: key,
	}

	// perform fetch request
	if _, err := c.ServiceClient.Delete(ctx, &req); err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}

	return nil
}

// Subscribe performs a subscribe request over gRPC in order to stream changes consumed by the node to the client. This
// allows a client to receive changes to the specified key when they occur instead of repeatedly making a request for
// the corresponding value. The response channel will be closed when the stream EOFs. The cancel func can be used to
// prematurely cancel the subscription connection.
func (c *Client) Subscribe(key string) (chan *pb.FetchResponse, context.CancelFunc, error) {
	c.debugf("[subscribe] %s", key)

	ctx, cancel := context.WithCancel(context.Background())
	// subscribe to changes for the specified key
	stream, err := c.ServiceClient.Subscribe(ctx, &pb.FetchRequest{Key: key})
	if err != nil {
		return nil, cancel, fmt.Errorf("failed to subscribe to %s: %w", key, err)
	}

	ch := make(chan *pb.FetchResponse)
	go func() {
		// retrieve stream of responses - calling cancel will break out of this loop, triggering the cleanup below
		for {
			item, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				c.debugf("failed to read from %s subscription: %s", key, err)
				break
			}

			ch <- item
		}

		// clean up
		c.debugf("subscription to %s closed", key)
		cancel()
		close(ch)
		// nil channel to prevent panic on unexpected write
		ch = nil
	}()

	return ch, cancel, nil
}

// Printf wraps log.Printf() to only write logs if logging is enabled.
func (c *Client) debugf(format string, v ...any) {
	if !c.DebugLog {
		return
	}
	c.Logger(format, v...)
}
