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
	conn, err := grpc.Dial(":"+strconv.Itoa(port), grpc.WithInsecure())
	if err != nil {
		fmt.Printf("failed to connect to server: %s", err)
		return
	}
	defer conn.Close()

	client := pb.NewKVServiceClient(conn)
	// publish some records
	if err := publish(client, "animals", []string{"dog", "cat", "hippo"}); err != nil {
		log.Printf("failed to publish: %s", err)
		return
	}
	if err := publish(client, "misc_data", "this is a chunky piece of random data"); err != nil {
		log.Printf("failed to publish: %s", err)
		return
	}
	if err := publish(client, "animals", []string{"dog", "cat", "hippo", "tiger", "zebra"}); err != nil {
		log.Printf("failed to publish: %s", err)
		return
	}

	// retrieve an existing record
	var animals []string
	ts, err := fetch(client, "animals", &animals)
	if err != nil {
		log.Printf("failed to fetch: %s", err)
		return
	}
	log.Printf("fetched animals: %v (created at %d)", animals, ts)
}

func publish(client pb.KVServiceClient, key string, value interface{}) error {
	log.Printf("[publish] %s -> %+v", key, value)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// gob encode value into bytes
	buf := &bytes.Buffer{}
	encoder := gob.NewEncoder(buf)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("failed to encode value: %s", err)
	}

	req := pb.PublishRequest{
		Key:   key,
		Value: buf.Bytes(),
	}

	if _, err := client.Publish(ctx, &req); err != nil {
		return fmt.Errorf("failed to publish: %s", err)
	}

	return nil
}

func fetch(client pb.KVServiceClient, key string, data interface{}) (int64, error) {
	log.Printf("[fetch] %s", key)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := pb.FetchRequest{
		Key: key,
	}

	resp, err := client.Fetch(ctx, &req)
	if err != nil {
		return 0, fmt.Errorf("failed to publish: %s", err)
	}

	// gob decode bytes into specified type
	buf := bytes.NewReader(resp.Value)
	decoder := gob.NewDecoder(buf)
	if err := decoder.Decode(data); err != nil {
		return 0, fmt.Errorf("failed to decode value: %s", err)
	}

	return resp.Timestamp, nil
}
