// Package main implements a command line client tool for making requests to a KV server in order to modify or fetch
// records.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jemgunay/distributed-kvstore/pkg/client"
)

var addr = "localhost:7000"

func main() {
	// parse flags
	flag.StringVar(&addr, "addr", addr, "the target server's address")
	debugLogsEnabled := flag.Bool("logs_enabled", false, "whether debug logs should be enabled")
	flag.Parse()

	// connect to gRPC server
	fmt.Printf("connecting to server on %s\n", addr)
	c, err := client.NewKVClient(addr)
	if err != nil {
		fmt.Printf("failed to create client: %s", err)
		return
	}
	defer c.Close()

	c.DebugLog = *debugLogsEnabled

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> publish, fetch, delete or subscribe (e.g. publish key value): ")
		text, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("failed to read from stdin: %s\n", err)
			return
		}

		items := strings.Split(strings.TrimSpace(text), " ")
		if len(items) < 2 {
			continue
		}

		// remove prefixed/suffixed white space for each component
		for i := range items {
			items[i] = strings.TrimSpace(items[i])
		}

		switch items[0] {
		case "fetch", "f":
			var value string
			ts, err := c.Fetch(items[1], &value)
			if err != nil {
				fmt.Printf("failed to fetch from server: %s\n", err)
				continue
			}
			fmt.Printf("successfully fetched %s (%d)\n", value, ts)

		case "publish", "p":
			if len(items) != 3 {
				continue
			}
			if err := c.Publish(items[1], items[2]); err != nil {
				fmt.Printf("failed to publish to server: %s\n", err)
				continue
			}
			fmt.Println("successfully published")

		case "delete", "d":
			if err := c.Delete(items[1]); err != nil {
				fmt.Printf("failed to delete from server: %s\n", err)
				continue
			}
			fmt.Println("successfully deleted")

		case "subscribe", "s":
			ch, _, err := c.Subscribe(items[1])
			if err != nil {
				fmt.Printf("failed to subscribe to server: %s\n", err)
				continue
			}
			for {
				resp, ok := <-ch
				if !ok {
					break
				}
				val := strings.TrimSpace(string(resp.Value))
				fmt.Printf("subscription read for %s: %s @ %d\n", items[1], val, resp.Timestamp)
			}
			fmt.Println("subscription ended")
		}
	}
}
