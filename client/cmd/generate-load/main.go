// Package main implements a command line client tool for generating requests to a store server network. Use the
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"time"

	"github.com/jemgunay/distributed-kvstore/client"
	"golang.org/x/sync/errgroup"
)

var (
	numServices       int
	numFetchRequests  = 100
	numPutRequests    = 100
	numDeleteRequests = 50
	clients           []*client.KVClient
)

func main() {
	// parse flags
	flag.IntVar(&numServices, "num_services", numServices, "the number of active services generate load for")
	flag.IntVar(&numFetchRequests, "num_fetches", numFetchRequests, "the number of fetch requests to generate per client")
	flag.IntVar(&numPutRequests, "num_publishes", numPutRequests, "the number of publish requests to generate per client")
	flag.IntVar(&numDeleteRequests, "num_deletes", numDeleteRequests, "the number of delete requests to generate per client")
	flag.Parse()

	if numServices == 0 {
		log.Printf("num_services must be greater than 0")
		return
	}

	eg := errgroup.Group{}

	startTime := time.Now()

	for i := 0; i < numServices; i++ {
		// create clients
		c, err := client.NewKVClient(":" + strconv.Itoa(7001+i))
		if err != nil {
			fmt.Printf("failed to create client: %s", err)
			return
		}
		defer c.Close()

		//clients = append(clients, c)
		eg.Go(func() error {
			for j := 0; j < numFetchRequests; j++ {
				if err := c.Publish(randStr(10), randStr(100)); err != nil {
					return fmt.Errorf("failed to publish to server: %s\n", err)
				}

				if j > numFetchRequests/2 {
					startFetching(c, &eg)
					startDeleting(c, &eg)
				}
			}

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		log.Printf("error writing to server store: %s", err)
		return
	}

	log.Printf("finished in %s", time.Since(startTime))
}

func startFetching(client *client.KVClient, eg *errgroup.Group) {
	eg.Go(func() error {
		for k := 0; k < numFetchRequests; k++ {
			var value string
			if _, err := client.Fetch(randStr(10), &value); err != nil {
				return fmt.Errorf("failed to publish to server: %s\n", err)
			}
		}

		return nil
	})
}

func startDeleting(client *client.KVClient, eg *errgroup.Group) {
	eg.Go(func() error {
		for k := 0; k < numDeleteRequests; k++ {
			if err := client.Delete(randStr(10)); err != nil {
				return fmt.Errorf("failed to delete from server: %s\n", err)
			}
		}

		return nil
	})
}

const charset = "abcdefghijklmnopqrstuvwxyz01234567890"

func randStr(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}

	return string(b)
}
