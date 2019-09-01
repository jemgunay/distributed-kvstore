package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jemgunay/distributed-kvstore/client"
)

var port = 6000

func main() {
	// parse flags
	flag.IntVar(&port, "port", port, "the target server's port")
	flag.Parse()

	// connect to gRPC server
	fmt.Printf("connecting to server on port %d\n", port)
	c, err := client.NewKVClient(":" + strconv.Itoa(port))
	if err != nil {
		fmt.Printf("failed to create client: %s", err)
		return
	}
	defer c.Close()

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> publish, fetch or delete (i.e. publish key value): ")
		text, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("failed to read from stdin: %s\n", err)
			return
		}

		items := strings.Split(text, " ")
		if len(items) < 2 {
			continue
		}

		for i := range items {
			items[i] = strings.TrimSpace(items[i])
		}

		switch items[0] {
		case "fetch":
			var value string
			ts, err := c.Fetch(items[1], &value)
			if err != nil {
				fmt.Printf("failed to fetch from server: %s\n", err)
				continue
			}
			fmt.Printf("successfully fetched %s (%d)\n", value, ts)

		case "publish":
			if len(items) != 3 {
				continue
			}
			if err := c.Publish(items[1], items[2]); err != nil {
				fmt.Printf("failed to publish to server: %s\n", err)
				continue
			}
			fmt.Printf("successfully published\n")

		case "delete":
			if err := c.Delete(items[1]); err != nil {
				fmt.Printf("failed to delete from server: %s\n", err)
				continue
			}
			fmt.Printf("successfully deleted\n")

		default:
			continue
		}
	}
}
