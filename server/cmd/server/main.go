// Package main creates an instance of a KV server.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/jemgunay/distributed-kvstore/server"
	"github.com/jemgunay/distributed-kvstore/server/store"
)

var (
	port           = 6000
	nodesAddresses multiFlag
)

func main() {
	// parse flags
	flag.IntVar(&port, "port", port, "the port this server should serve from")
	flag.Var(&nodesAddresses, "node_address", "list of node addresses that this node should attempt to synchronise with")
	debugLogsEnabled := flag.Bool("logs_enabled", true, "whether debug logs should be enabled")
	flag.Parse()

	// create a store and kvServer
	kvStore := store.NewStore()
	kvServer := server.NewKVSyncServer(kvStore, kvStore)
	// TODO: replace with zap
	kvServer.DebugLog = *debugLogsEnabled

	// start serving
	log.Printf("KV server (pid %d) listening on port %d", os.Getpid(), port)
	if err := kvServer.Start(":"+strconv.Itoa(port), nodesAddresses); err != nil {
		log.Printf("KV server has shut down unexpectedly: %s", err)
		return
	}
	log.Printf("KV server has shut down")
}

// multiFlag satisfies the Value interface in order to parse multiple command line arguments of the same name into a
// slice.
type multiFlag []string

func (m *multiFlag) String() string {
	return fmt.Sprintf("%+v", *m)
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}
