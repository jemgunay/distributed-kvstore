// Package server distributed implements a gRPC KV store server. It is capable of synchronising the store data across
// multiple node instances.
package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sort"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	pb "github.com/jemgunay/distributed-kvstore/proto"
)

// Node represents a single service node in the distributed store network.
type Node struct {
	address   string
	startTime int64
	id        uint32

	*grpc.ClientConn
	SyncClient      pb.SyncClient
	syncRequestChan chan *pb.SyncMessage
}

// Storer is the interface that wraps the store methods required by a KV store.
type Storer interface {
	Get(key string) (value []byte, timestamp int64, err error)
	Put(key string, value []byte, timestamp int64) error
	Delete(key string, timestamp int64) error
	Subscribe(ctx context.Context, key string) (chan *pb.FetchResponse, context.CancelFunc)
}

// SyncSourcer is the interface that wraps the methods required to sync a store operation across a KV store distributed
// network's nodes.
type SyncSourcer interface {
	// SyncOut is responsible for returning a SyncMessage to be distributed to the rest of the network's nodes.
	SyncOut() *pb.SyncMessage
	// SyncIn receives a SyncMessage and attempts to insert the corresponding operation into the store.
	SyncIn(*pb.SyncMessage) error
}

// KVSyncServer is a gRPC KV synchronised server which satisfies both the KVServiceServer and SyncServiceServer
// interfaces.
type KVSyncServer struct {
	grpcServer *grpc.Server
	// ClientTimeout is the timeout for the gRPC client requests. Set this before calling Start().
	ClientTimeout time.Duration
	// IdentifyRetries is the number of attempts to retry a connection to a node client, at once per 500ms.
	IdentifyRetries int
	store           Storer
	syncSourcer     SyncSourcer
	// collection of nodes in the distributed store network, where the key is the nodes ID (determined during the
	// identification stage)
	nodes map[uint32]*Node

	startTime int64
	id        uint32
	// the buffer size of the channel in each node's client used to queue sync requests to each node
	syncRequestChanBufSize int
	DebugLog               bool
	shutdownCh             chan struct{}
}

// NewKVSyncServer creates a new gRPC KV synchronised server.
func NewKVSyncServer(store Storer, syncSource SyncSourcer) *KVSyncServer {
	return &KVSyncServer{
		grpcServer:             grpc.NewServer(),
		ClientTimeout:          time.Second * 10,
		IdentifyRetries:        100,
		store:                  store,
		syncSourcer:            syncSource,
		syncRequestChanBufSize: 1 << 10, // 1024
	}
}

// Start registers the gRPC handlers and starts the KV sync server.
func (s *KVSyncServer) Start(address string, nodeAddresses []string) error {
	if s.store == nil {
		return errors.New("server store is uninitialised")
	}

	s.shutdownCh = make(chan struct{})
	s.startTime = time.Now().UTC().UnixNano()

	// register gRPC handlers
	pb.RegisterKVStoreServer(s.grpcServer, s)
	pb.RegisterSyncServer(s.grpcServer, s)

	// bind to specified port over TCP
	l, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen: %s", err)
	}

	eg := errgroup.Group{}
	eg.Go(func() error {
		// start serving via gRPC
		return s.grpcServer.Serve(l)
	})

	eg.Go(func() error {
		// identify the other nodes to sync with
		if err := s.identifyInitialNodes(nodeAddresses); err != nil {
			return err
		}
		log.Printf("node identification complete for node %d", s.id)

		// start sync poller for each node connection
		for id, node := range s.nodes {
			s.Printf("[node %d on %s] node %d on \"%s\" started at %d", s.id, address, id, node.address, node.startTime)
			go s.syncPollNode(node)
		}

		// fan out to each node, sending store operation sync request via node's sync channel
		for {
			select {
			case <-s.shutdownCh:
				return nil
			default:
			}

			syncReq := s.syncSourcer.SyncOut()
			for _, node := range s.nodes {
				node.syncRequestChan <- syncReq
			}
		}
	})

	return eg.Wait()
}

// attempt to form the initial network with all nodes addressed in the startup flags
func (s *KVSyncServer) identifyInitialNodes(nodeAddresses []string) error {
	// create temporary slice of nodes, including this node, in order to derive node IDs from order of startup
	discoveredNodes := make([]*Node, 0, len(nodeAddresses)+1)
	eg := errgroup.Group{}

	for _, addr := range nodeAddresses {
		addr := addr

		eg.Go(func() error {
			// create client to connect to the node
			conn, err := grpc.Dial(addr, grpc.WithInsecure())
			if err != nil {
				return fmt.Errorf("failed to connect to server on \"%s\": %s", addr, err)
			}

			client := pb.NewSyncClient(conn)

			var resp *pb.IdentifyMessage
			// attempt N number of times to fetch timestamp from node so that IDs can be determined from startup order
			for i := 0; i < s.IdentifyRetries; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), s.ClientTimeout)
				resp, err = client.Identify(ctx, &pb.IdentifyMessage{StartTime: s.startTime})
				cancel()

				if err == nil {
					break
				}
				time.Sleep(time.Millisecond * 200)
			}

			if err != nil {
				return fmt.Errorf("failed to identify node: %s", err)
			}

			// create new node
			newNode := &Node{
				address:   addr,
				startTime: resp.StartTime,

				ClientConn:      conn,
				SyncClient:      client,
				syncRequestChan: make(chan *pb.SyncMessage, s.syncRequestChanBufSize),
			}
			// TODO: protect array when appending (mutex)
			discoveredNodes = append(discoveredNodes, newNode)

			return nil
		})
	}

	// wait until node identification stage has completed - fail if any identify request errors
	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to identify all nodes: %s", err)
	}

	// add in self node in order to determine node IDs
	discoveredNodes = append(discoveredNodes, &Node{startTime: s.startTime})

	// sort by timestamp
	sort.Slice(discoveredNodes, func(i, j int) bool {
		return discoveredNodes[i].startTime < discoveredNodes[j].startTime
	})

	// use index in ordered slice as the node ID - each node will derive the same IDs from this process independently
	s.nodes = make(map[uint32]*Node, len(discoveredNodes)-1)
	for id, n := range discoveredNodes {
		if n.startTime == s.startTime {
			// we don't want to add self node to node store, but store its ID
			s.id = uint32(id)
			continue
		}
		n.id = uint32(id)
		s.nodes[n.id] = n
	}

	return nil
}

func (s *KVSyncServer) syncPollNode(node *Node) {
	// for each node, create long polling synchronised connection - do not apply a timeout to the context as we want to
	// keep this connection open indefinitely
	syncStream, err := node.SyncClient.Sync(context.Background())
	if err != nil {
		s.Printf("failed to open sync stream with node: %s", err)
		return
	}

	// pull sync requests from node's queue and send to node via node's client
	for req := range node.syncRequestChan {
		if err := syncStream.Send(req); err != nil {
			s.Printf("failed to send sync req to node %d, key: %s, err %s", node.id, req.Key, err)
			// TODO: on error, retry x times. On retry failure, attempt to feed back into queue
		}
	}
}

// Shutdown gracefully shuts down the gRPC server.
func (s *KVSyncServer) Shutdown() {
	close(s.shutdownCh)
	s.grpcServer.GracefulStop()
	// TODO: shutdown syncs gracefully, i.e. stop accepting client requests then drain sync request pool channel before
	// shutting down
}

// Publish processes publish requests from a client and stores the provided key/value pair.
func (s *KVSyncServer) Publish(ctx context.Context, r *pb.PublishRequest) (*pb.Empty, error) {
	if p, ok := peer.FromContext(ctx); ok {
		s.Printf("[%s -> publish] %s", p.Addr, r.Key)
	}

	// insert record into store
	err := s.store.Put(r.GetKey(), r.GetValue(), time.Now().UTC().UnixNano())
	return &pb.Empty{}, err
}

// Fetch processes fetch requests from a client and returns the value and timestamp associated with the specified key.
func (s *KVSyncServer) Fetch(ctx context.Context, r *pb.FetchRequest) (*pb.FetchResponse, error) {
	if p, ok := peer.FromContext(ctx); ok {
		s.Printf("[%s -> fetch] %s", p.Addr, r.Key)
	}

	var (
		resp = &pb.FetchResponse{}
		err  error
	)

	// pull record from store
	resp.Value, resp.Timestamp, err = s.store.Get(r.GetKey())
	return resp, err
}

// Subscribe allows a client to long poll for changes to a key in the store, allowing the client to receive updates
// without continuously performing fetch requests. If a delete request is processed, the subscription will be ended.
func (s *KVSyncServer) Subscribe(request *pb.FetchRequest, stream pb.KVStore_SubscribeServer) error {
	p, ok := peer.FromContext(stream.Context())
	if ok {
		s.Printf("[%s -> subscribe] %s", p.Addr, request.Key)
	}

	ch, cancel := s.store.Subscribe(stream.Context(), request.Key)
	defer func() {
		s.Printf("subscribe connection from %s closed", p.Addr)
		cancel()
	}()

	for {
		select {
		case resp, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(resp); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		case <-s.shutdownCh:
			return nil
		}
	}
}

// Delete processes delete requests from a client returns the value and timestamp associated with the specified key.
func (s *KVSyncServer) Delete(ctx context.Context, r *pb.DeleteRequest) (*pb.Empty, error) {
	if p, ok := peer.FromContext(ctx); ok {
		s.Printf("[%s -> delete] %s", p.Addr, r.Key)
	}

	// pull record from store
	err := s.store.Delete(r.GetKey(), time.Now().UTC().UnixNano())
	return &pb.Empty{}, err
}

// Identify processes identify requests used to initially ID a node on startup based on the startup timestamps of all
// nodes.
func (s *KVSyncServer) Identify(ctx context.Context, r *pb.IdentifyMessage) (*pb.IdentifyMessage, error) {
	if p, ok := peer.FromContext(ctx); ok {
		s.Printf("[%s -> identify]", p.Addr)
	}
	// TODO: consume the r.StartTime here to create a node and reduce need to ping the requesting server again to
	// exponentially reduce number of pings required in identification stage. This will also provide support for
	// future joining nodes

	return &pb.IdentifyMessage{StartTime: s.startTime}, nil
}

// Sync is responsible for receiving sync requests from other nodes and applying them to the store.
func (s *KVSyncServer) Sync(stream pb.Sync_SyncServer) error {
	p, ok := peer.FromContext(stream.Context())
	if ok {
		s.Printf("[%s -> sync_init]", p.Addr)
	}

	for {
		// receive a stream of sync requests from another node
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		s.Printf("[%s -> sync_in] op_type: %s, key: %s", p.Addr, resp.OperationType, resp.Key)
		// insert sync'd operation into this node's store
		if err := s.syncSourcer.SyncIn(resp); err != nil {
			return fmt.Errorf("failed to store sync message: %s", err)
		}
	}

	s.Printf("sync connection to %s closed", p.Addr)
	return nil
}

// Printf wraps log.Printf() to only write logs if logging is enabled.
func (s *KVSyncServer) Printf(format string, v ...interface{}) {
	if !s.DebugLog {
		return
	}
	log.Printf(format, v...)
}
