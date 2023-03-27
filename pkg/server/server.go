// Package server distributed implements a gRPC KV store server. It is capable of synchronising the store data across
// multiple node instances.
package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/jemgunay/distributed-kvstore/pkg/config"
	"github.com/jemgunay/distributed-kvstore/pkg/core"
	"github.com/jemgunay/distributed-kvstore/pkg/nodes"
	pb "github.com/jemgunay/distributed-kvstore/pkg/proto"
)

// Server is a gRPC KV synchronised server which satisfies both the KVServiceServer and SyncServiceServer
// interfaces.
type Server struct {
	logger      config.Logger
	store       core.Storer
	nodeManager core.Noder
	grpcServer  *grpc.Server

	pb.UnimplementedKVStoreServer
}

// NewServer creates a new gRPC KV synchronised server.
func NewServer(logger config.Logger, store core.Storer, nodeManager *nodes.Manager) *Server {
	return &Server{
		logger:      logger,
		store:       store,
		nodeManager: nodeManager,
		grpcServer:  grpc.NewServer(),
	}
}

// Start registers the gRPC handlers and starts the KV sync server. It is a blocking operation and will return on error
// or shutdown.
func (s *Server) Start(port int) error {
	// register gRPC handlers
	pb.RegisterKVStoreServer(s.grpcServer, s)

	// bind to specified port over TCP
	address := ":" + strconv.Itoa(port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	syncOut := s.store.GetSyncStream()
	syncIn := s.nodeManager.FanOut()
	go func() {
		for req := range syncOut {
			syncIn <- req
		}
	}()

	return s.grpcServer.Serve(listener)
}

// Publish processes publish requests from a client and stores the provided key/value pair.
func (s *Server) Publish(ctx context.Context, req *pb.PublishRequest) (*pb.Empty, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, errors.New("failed to extract peer details from context")
	}
	logger := s.logger.With(zap.Stringer("addr", p.Addr), zap.String("key", req.Key))
	logger.Info("publish request received")

	// insert record into store
	err := s.store.Put(req.GetKey(), req.GetValue(), time.Now().UTC().UnixNano())
	if err != nil {
		return nil, fmt.Errorf("failed to process put: %w", err)
	}

	return &pb.Empty{}, nil
}

// Fetch processes fetch requests from a client and returns the value and timestamp associated with the specified key.
func (s *Server) Fetch(ctx context.Context, req *pb.FetchRequest) (*pb.FetchResponse, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, errors.New("failed to extract peer details from context")
	}
	logger := s.logger.With(zap.Stringer("addr", p.Addr), zap.String("key", req.Key))
	logger.Info("fetch request received")

	// pull record from store
	val, ts, err := s.store.Get(req.GetKey())
	if err != nil {
		return nil, fmt.Errorf("failed to process get: %w", err)
	}

	return &pb.FetchResponse{
		Value:     val,
		Timestamp: ts,
	}, nil
}

// Subscribe allows a client to long poll for changes to a key in the store, allowing the client to receive updates
// without continuously performing fetch requests. If a delete request is processed, the subscription will be ended.
func (s *Server) Subscribe(req *pb.FetchRequest, stream pb.KVStore_SubscribeServer) error {
	ctx := stream.Context()

	p, ok := peer.FromContext(ctx)
	if !ok {
		return errors.New("failed to extract peer details from context")
	}
	logger := s.logger.With(zap.Stringer("addr", p.Addr), zap.String("key", req.Key))
	logger.Info("subscribe request received")

	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		logger.Info("subscribe connection from %s closed")
	}()

	subStream, err := s.store.Subscribe(ctx, req.Key)
	if err != nil {
		return fmt.Errorf("failed to subscribe to store key: %w", err)
	}

	for {
		resp, ok := <-subStream
		if !ok {
			return nil
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

// Delete processes a delete request from a client.
func (s *Server) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.Empty, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, errors.New("failed to extract peer details from context")
	}
	s.logger.Info("delete request received", zap.Stringer("addr", p.Addr), zap.String("key", req.Key))

	if err := s.store.Delete(req.GetKey(), time.Now().UTC().UnixNano()); err != nil {
		return &pb.Empty{}, fmt.Errorf("failed to process delete: %w", err)
	}
	return &pb.Empty{}, nil
}

// Identify processes identify requests used to initially ID a node on startup
// based on the startup timestamps of all nodes.
func (s *Server) Identify(ctx context.Context, req *pb.IdentityRequest) (*pb.IdentityResponse, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, errors.New("failed to extract peer details from context")
	}
	s.logger.Info("identify request received", zap.String("addr", p.Addr.String()),
		zap.String("id", req.Node.GetId()))

	nodeIdentities := s.nodeManager.Nodes()

	resp := &pb.IdentityResponse{
		Nodes: make(map[string]*pb.Node, len(nodeIdentities)),
	}
	for _, n := range nodeIdentities {
		if n.ID == req.Node.GetId() {
			continue
		}

		resp.Nodes[n.ID] = n.ToProto()
	}

	return resp, nil
}

// Sync is responsible for receiving sync requests from other nodes and applying them to the store.
func (s *Server) Sync(stream pb.KVStore_SyncServer) error {
	for {
		// receive a stream of sync requests from another node
		msg, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				s.logger.Error("sync connection closed due to EOF", zap.Error(err))
				return nil
			}

			const msg = "sync connection closed due to unexpected error"
			s.logger.Error(msg, zap.Error(err))
			return fmt.Errorf(msg+": %w", err)
		}

		s.logger.Debug("sync_in", zap.Stringer("op", msg.Operation),
			zap.String("key", msg.Key), zap.Int64("ts", msg.GetTimestamp()))

		// TODO: create a SyncMessage with From & ToProto
		// insert sync operation into this node's store
		if err := s.store.SyncIn(msg); err != nil {
			const msg = "failed to store sync message"
			s.logger.Error(msg, zap.Error(err))
			return fmt.Errorf(msg+": %w", err)
		}
	}
}
