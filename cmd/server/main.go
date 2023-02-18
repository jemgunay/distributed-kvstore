// Package main creates an instance of a KV server.
package main

import (
	"os"

	"go.uber.org/zap"

	"github.com/jemgunay/distributed-kvstore/pkg/config"
	"github.com/jemgunay/distributed-kvstore/pkg/nodes"
	"github.com/jemgunay/distributed-kvstore/pkg/server"
	"github.com/jemgunay/distributed-kvstore/pkg/server/store"
)

func main() {
	cfg := config.New()
	logger := cfg.Logger

	// create a store and kvServer
	kvStore := store.NewStore()
	nodeManager, err := nodes.New(logger, cfg.Port, cfg.NodeAddresses)
	if err != nil {
		logger.Error("failed to init node manager", zap.Error(err))
		os.Exit(1)
	}
	kvServer := server.NewServer(logger, kvStore, nodeManager)

	// start serving
	logger.Info("starting HTTP server", zap.Int("pid", os.Getpid()), zap.Int("port", cfg.Port))
	if err := kvServer.Start(cfg.Port); err != nil {
		logger.Error("HTTP server shut down unexpectedly", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("server shut down gracefully")
}
