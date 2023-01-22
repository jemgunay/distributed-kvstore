// Package main implements a command line client tool for generating requests to a store server network. Use the
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/jemgunay/distributed-kvstore/client"
)

var (
	numServices        int
	numPublishRequests = 1000
)

func main() {
	// parse flags
	flag.IntVar(&numServices, "num_services", numServices, "the number of active services generate load for")
	flag.Parse()

	if numServices == 0 {
		log.Printf("num_services must be greater than 0")
		return
	}
	log.Printf("generating load for %d services", numServices)

	startTime := time.Now()
	eg := errgroup.Group{}

	for i := 0; i < numServices; i++ {
		i := i

		// create client
		c, err := client.NewKVClient(":" + strconv.Itoa(7001+i))
		if err != nil {
			log.Printf("failed to create client: %s", err)
		}
		defer c.Close()

		eg.Go(func() error {
			// generate load keys
			keys := make([]string, 0, numPublishRequests)
			fetchKeys := make([]string, 0, numPublishRequests/2)
			deleteKeys := make([]string, 0, numPublishRequests/2)
			const keySize = 15
			for j := 0; j < cap(fetchKeys); j++ {
				newKey := randStr(keySize)
				fetchKeys = append(fetchKeys, newKey)
				keys = append(keys, newKey)

				newKey = randStr(keySize)
				deleteKeys = append(deleteKeys, newKey)
				keys = append(keys, newKey)
			}

			// action load on nodes
			for j := 0; j < len(keys); j++ {
				if err := c.Publish(keys[j], randStr(100)); err != nil {
					return fmt.Errorf("failed to publish to server: %w", err)
				}
			}

			startFetching(c, &eg, fetchKeys)
			startFetching(c, &eg, fetchKeys)
			startFetching(c, &eg, fetchKeys)
			startDeleting(c, &eg, deleteKeys)

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		log.Printf("error interacting with kv server: %s", err)
		return
	}

	totalPublish := numPublishRequests * numServices
	totalFetch := (numPublishRequests / 2) * 3 * numServices
	totalDelete := (numPublishRequests / 2) * numServices
	totalRequests := totalPublish + totalFetch + totalDelete
	log.Printf("completed total=%d,publish=%d,fetch=%d,delete=%d, in %s", totalRequests, totalPublish,
		totalFetch, totalDelete, time.Since(startTime))
}

func startFetching(client *client.KVClient, eg *errgroup.Group, keys []string) {
	eg.Go(func() error {
		for i := 0; i < len(keys); i++ {
			var value string
			if _, err := client.Fetch(keys[i], &value); err != nil {
				return fmt.Errorf("failed to fetch from server: %w", err)
			}
		}

		return nil
	})
}

func startDeleting(client *client.KVClient, eg *errgroup.Group, keys []string) {
	eg.Go(func() error {
		for i := 0; i < len(keys); i++ {
			if err := client.Delete(keys[i]); err != nil {
				return fmt.Errorf("failed to delete from server: %w", err)
			}
		}

		return nil
	})
}

const charset = "abcdefghijklmnopqrstuvwxyz01234567890"

func init() {
	rand.Seed(time.Now().UnixNano())
}

func randStr(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}

	return string(b)
}
