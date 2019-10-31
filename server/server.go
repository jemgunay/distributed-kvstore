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
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	pb "github.com/jemgunay/distributed-kvstore/proto"
	"github.com/jemgunay/distributed-kvstore/server/store"
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

// State represents the active state of the server.
type State uint

// The possible states a KV server can be in.
const (
	Uninitialised State = iota
	InitialIdentification
	Initialised
	Shutdown
)

var (
	errUninitialised     = errors.New("node is not yet initialised")
	errOutdatedTimestamp = errors.New("timestamp must be newer than the most recently started existing node in the network")
)

type nodeStore struct {
	// collection of nodes in the distributed store network, where the key is the node's startup timestamp
	timestampLookup map[int64]*Node
	ordered         []*Node
	wg              *sync.WaitGroup
	sync.RWMutex
}

// KVSyncServer is a gRPC KV synchronised server which satisfies both the KVServiceServer and SyncServiceServer
// interfaces.
type KVSyncServer struct {
	state      State
	grpcServer *grpc.Server
	// ClientTimeout is the timeout for the gRPC client requests. Set this before calling Start().
	ClientTimeout time.Duration
	store         Storer
	syncSourcer   SyncSourcer
	nodes         *nodeStore
	// IdentifyRetries is the number of attempts to retry a connection to a node client, at once per 200ms.
	IdentifyRetries int
	startTime       int64
	id              uint32
	address         string
	// the buffer size of the channel in each node's client used to queue sync requests to each node
	syncRequestChanBufSize int
	DebugLog               bool
	shutdownChan           chan struct{}
}

// NewKVSyncServer creates a new gRPC KV synchronised server.
func NewKVSyncServer(store Storer, syncSource SyncSourcer) *KVSyncServer {
	return &KVSyncServer{
		state:           Uninitialised,
		grpcServer:      grpc.NewServer(),
		ClientTimeout:   time.Second * 10,
		IdentifyRetries: 100,
		store:           store,
		syncSourcer:     syncSource,
		nodes: &nodeStore{
			wg: &sync.WaitGroup{},
		},
		syncRequestChanBufSize: 1 << 10, // 1024
	}
}

// State returns the server's state.
func (s *KVSyncServer) State() State {
	return s.state
}

// Start registers the gRPC handlers and starts the KV sync server.
func (s *KVSyncServer) Start(address string, nodeAddresses []string) error {
	if s.state != Uninitialised {
		return errors.New("server already started")
	}
	if s.store == nil {
		return errors.New("server store is uninitialised")
	}

	s.address = address
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
		// start serving store access via gRPC
		return s.grpcServer.Serve(l)
	})

	eg.Go(func() error {
		// identify the other nodes to initially sync with
		s.shutdownChan = make(chan struct{})
		if err := s.identifyInitialNodes(nodeAddresses); err != nil {
			return err
		}
		log.Printf("node identification complete for node %d", s.id)

		s.state = Initialised
		// main server poller
		for {
			select {
			case <-s.shutdownChan:
				return nil
			default:
				// pull sync request from store and fan out to each node, sending store operation sync request via
				// node's sync channel
				syncReq := s.syncSourcer.SyncOut()
				for _, node := range s.nodes.timestampLookup {
					node.syncRequestChan <- syncReq
				}
			}
		}
	})

	return eg.Wait()
}

// attempt to form the initial network with all nodes addressed in the startup flags
func (s *KVSyncServer) identifyInitialNodes(nodeAddresses []string) error {
	s.state = InitialIdentification
	s.nodes.timestampLookup = make(map[int64]*Node, len(nodeAddresses))
	// create temporary slice of nodes, including this node, in order to derive node IDs from order of startup
	s.nodes.ordered = make([]*Node, 0, len(nodeAddresses)+1)

	// perform identification for all addresses concurrently
	eg := errgroup.Group{}
	for _, addr := range nodeAddresses {
		addr := addr

		eg.Go(func() error {
			return s.identifyNode(addr)
		})
	}

	// wait until node identification stage has completed - fail if any identify request errors
	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to identify all nodes: %s", err)
	}

	// add in self node in order to determine node IDs
	s.nodes.ordered = append(s.nodes.ordered, &Node{startTime: s.startTime})

	// sort by timestamp
	sort.Slice(s.nodes.ordered, func(i, j int) bool {
		return s.nodes.ordered[i].startTime < s.nodes.ordered[j].startTime
	})

	// use index in ordered slice as the node ID - each node will derive the same IDs from this process independently
	for id, node := range s.nodes.ordered {
		if node.startTime == s.startTime {
			// we don't want to add self node to node store, but store its ID
			s.id = uint32(id)
			continue
		}
		node.id = uint32(id)

		// start sync poller for each node connection
		s.Printf("[node %d on %s] node %d on \"%s\" started at %d", s.id, s.address, node.id, node.address, node.startTime)
		go s.syncPollNode(node)
	}

	return nil
}

func (s *KVSyncServer) identifyNode(addr string) error {
	// create client to connect to the node
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("failed to connect to server on \"%s\": %s", addr, err)
	}

	client := pb.NewSyncClient(conn)

	var resp *pb.IdentifyMessage
	// attempt N number of times to fetch timestamp from node so that IDs can be determined from startup order
	for i := 0; i < s.IdentifyRetries; i++ {
		ctx, _ := context.WithTimeout(context.Background(), s.ClientTimeout)
		resp, err = client.Identify(ctx, &pb.IdentifyMessage{
			StartTime: s.startTime,
			Addr:      s.address,
		})
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
		address:         addr,
		startTime:       resp.StartTime,
		ClientConn:      conn,
		SyncClient:      client,
		syncRequestChan: make(chan *pb.SyncMessage, s.syncRequestChanBufSize),
	}

	s.nodes.Lock()
	s.nodes.timestampLookup[newNode.startTime] = newNode
	s.nodes.ordered = append(s.nodes.ordered, newNode)
	s.nodes.Unlock()
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

// Shutdown gracefully shuts down the gRPC server. TODO: test this gracefully shuts down as expected.
func (s *KVSyncServer) Shutdown() {
	if s.state < InitialIdentification || s.state == Shutdown {
		return
	}
	s.state = Shutdown
	s.grpcServer.GracefulStop()
	close(s.shutdownChan)
}

// Publish processes publish requests from a client and stores the provided key/value pair.
func (s *KVSyncServer) Publish(ctx context.Context, r *pb.PublishRequest) (*pb.Empty, error) {
	if p, ok := peer.FromContext(ctx); ok {
		s.Printf("[%s -> publish] %s", p.Addr, r.Key)
	}

	if s.state < Initialised {
		return &pb.Empty{}, status.Error(codes.Unavailable, errUninitialised.Error())
	}

	// insert record into store
	err := s.store.Put(r.GetKey(), r.GetValue(), time.Now().UTC().UnixNano())
	return &pb.Empty{}, grpcWrapError(err)
}

// Fetch processes fetch requests from a client and returns the value and timestamp associated with the specified key.
func (s *KVSyncServer) Fetch(ctx context.Context, r *pb.FetchRequest) (*pb.FetchResponse, error) {
	if p, ok := peer.FromContext(ctx); ok {
		s.Printf("[%s -> fetch] %s", p.Addr, r.Key)
	}

	resp := &pb.FetchResponse{}
	if s.state < Initialised {
		return resp, status.Error(codes.Unavailable, errUninitialised.Error())
	}

	// pull record from store
	var err error
	resp.Value, resp.Timestamp, err = s.store.Get(r.GetKey())
	return resp, grpcWrapError(err)
}

// Subscribe allows a client to long poll for changes to a key in the store, allowing the client to receive updates
// without continuously performing fetch requests. If a delete request is processed, the subscription will be ended.
func (s *KVSyncServer) Subscribe(request *pb.FetchRequest, stream pb.KVStore_SubscribeServer) error {
	p, ok := peer.FromContext(stream.Context())
	if ok {
		s.Printf("[%s -> subscribe] %s", p.Addr, request.Key)
	}

	if s.state < Initialised {
		return status.Error(codes.Unavailable, errUninitialised.Error())
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
		case <-s.shutdownChan:
			return nil
		}
	}
}

// Delete processes delete requests from a client returns the value and timestamp associated with the specified key.
func (s *KVSyncServer) Delete(ctx context.Context, r *pb.DeleteRequest) (*pb.Empty, error) {
	if p, ok := peer.FromContext(ctx); ok {
		s.Printf("[%s -> delete] %s", p.Addr, r.Key)
	}

	if s.state < Initialised {
		return &pb.Empty{}, status.Error(codes.Unavailable, errUninitialised.Error())
	}

	// pull record from store
	err := s.store.Delete(r.GetKey(), time.Now().UTC().UnixNano())
	return &pb.Empty{}, grpcWrapError(err)
}

// Identify processes identify requests used to initially ID a node on startup based on the startup timestamps of all
// nodes.
func (s *KVSyncServer) Identify(ctx context.Context, r *pb.IdentifyMessage) (*pb.IdentifyMessage, error) {
	if p, ok := peer.FromContext(ctx); ok {
		s.Printf("[%s -> identify]", p.Addr)
	}

	// handle nodes identifying when joining the network after network initialisation
	/*if s.state == Initialised {
		// this must be a node connecting after the initial identification phase
		if r.StartTime < s.nodes.ordered[len(s.nodes.ordered)-1].startTime {
			// this newly connecting node is too old - trigger a timestamp refresh
			return &pb.IdentifyMessage{}, status.Error(codes.FailedPrecondition, errOutdatedTimestamp.Error())
		}

		// insert into nodes store
		s.Printf("[identifying post-initialisation -> %s]", r.Addr)
		if err := s.identifyNode(r.Addr); err != nil {
			return &pb.IdentifyMessage{}, status.Error(codes.Internal, err.Error())
		}
	}*/

	return &pb.IdentifyMessage{
		StartTime: s.startTime,
		Addr:      s.address,
	}, nil
}

// Sync is responsible for receiving sync requests from other nodes and applying them to the store.
func (s *KVSyncServer) Sync(stream pb.Sync_SyncServer) error {
	p, ok := peer.FromContext(stream.Context())
	if ok {
		s.Printf("[%s -> sync_init]", p.Addr)
	}

	if s.state < InitialIdentification {
		return status.Error(codes.Unavailable, errUninitialised.Error())
	}

	defer s.Printf("sync connection to %s closed", p.Addr)

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

	return nil
}

// Printf wraps log.Printf() to only write logs if logging is enabled.
func (s *KVSyncServer) Printf(format string, v ...interface{}) {
	if !s.DebugLog {
		return
	}
	log.Printf(format, v...)
}

// wraps a standard store error with the corresponding gRPC status context
func grpcWrapError(err error) error {
	switch err {
	case nil:
		return err
	case store.ErrInvalidKey:
		return status.Error(codes.InvalidArgument, err.Error())
	case store.ErrNotFound:
		return status.Error(codes.NotFound, err.Error())
	case store.ErrPollerBufferFull:
		return status.Error(codes.ResourceExhausted, err.Error())
	default:
		return status.Error(codes.Unknown, err.Error())
	}
}
