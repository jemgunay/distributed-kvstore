// Package main is an example of passing storing more complex Go structures such as arrays via a client.
package main

import (
	"flag"
	"image/color"
	"log"
	"strconv"

	"github.com/jemgunay/distributed-kvstore/client"
)

var port = 6000

type vehicle struct {
	Wheels uint
	Doors  uint
	Colour color.RGBA
}

func main() {
	// parse flags
	flag.IntVar(&port, "port", port, "the target server's port")
	flag.Parse()

	// connect to gRPC server
	log.Printf("connecting to server on port %d", port)
	c, err := client.NewKVClient(":" + strconv.Itoa(port))
	if err != nil {
		log.Printf("failed to create client: %s", err)
		return
	}
	defer c.Close()

	// publish some records
	data := []struct {
		key   string
		value any
	}{
		{"animals", []string{"dog", "cat", "hippo"}},
		{"misc_data", "this is a string of random data"},
		{"animals", []string{"dog", "cat", "hippo", "tiger", "zebra"}}, // overwrite previous animals value
		{"vehicle", vehicle{
			Wheels: 4,
			Doors:  5,
			Colour: color.RGBA{100, 100, 100, 255},
		}},
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

	// retrieve an existing records
	var animals []string
	ts, err := c.Fetch("animals", &animals)
	if err != nil {
		log.Printf("failed to fetch: %s", err)
		return
	}
	log.Printf("fetched animals: %v (created at %d)", animals, ts)

	var car *vehicle
	ts, err = c.Fetch("vehicle", &car)
	if err != nil {
		log.Printf("failed to fetch: %s", err)
		return
	}
	log.Printf("fetched vehicle: %+v (created at %d)", *car, ts)
}
