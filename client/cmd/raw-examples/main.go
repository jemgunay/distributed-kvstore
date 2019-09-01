package main

import (
	"flag"
	"log"
	"strconv"

	"github.com/jemgunay/distributed-kvstore/client"
)

var port = 6000

func main() {
	// parse flags
	flag.IntVar(&port, "port", port, "the target server's port")
	flag.Parse()

	// connect to gRPC server
	log.Printf("connecting to server on port %d", port)
	c, err := client.NewKVClient(":" + strconv.Itoa(port))
	if err != nil {
		log.Printf("failed to create c: %s", err)
		return
	}
	defer c.Close()

	// publish some records
	data := []struct {
		key   string
		value interface{}
	}{
		{"animals", []string{"dog", "cat", "hippo"}},
		{"misc_data", "this is a chunky piece of random data"},
		{"animals", []string{"dog", "cat", "hippo", "tiger", "zebra"}},
	}

	for _, d := range data {
		if err := c.Publish(d.key, d.value); err != nil {
			log.Printf("failed to publish %s: %s", d.key, err)
			return
		}
	}

	// delete an existing record
	/*if err := c.Delete("animals"); err != nil {
		log.Printf("failed to delete: %s", err)
		return
	}*/

	// retrieve an existing record
	var animals []string
	ts, err := c.Fetch("animals", &animals)
	if err != nil {
		log.Printf("failed to fetch: %s", err)
		return
	}
	log.Printf("fetched animals: %v (created at %d)", animals, ts)
}
