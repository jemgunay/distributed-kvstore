package nodes

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/jemgunay/distributed-kvstore/pkg/config"
	"github.com/jemgunay/distributed-kvstore/pkg/nodes/identity"
	pb "github.com/jemgunay/distributed-kvstore/pkg/proto"
)

// Node represents a single service node in the distributed store network. It
// maps the identity to its sync channel.
type Node struct {
	identity.Identity
	syncRequestChan chan *pb.SyncMessage
}

// Manager manages node connection lifecycle and communications.
type Manager struct {
	logger   config.Logger
	identity identity.Identity

	initialNodes []string
	nodeList     []identity.Identity
	nodesListMu  *sync.Mutex

	registerStream chan Node
	fanOutStream   chan *pb.SyncMessage
}

// New initialises a new Manager which connects to the provided initial set of
// node addresses.
func New(logger config.Logger, port int, nodeAddresses []string) (*Manager, error) {
	id, err := identity.New(port)
	if err != nil {
		return nil, fmt.Errorf("attempting to identify self: %w", err)
	}

	m := &Manager{
		logger:   logger,
		identity: id,

		initialNodes: nodeAddresses,
		nodeList:     []identity.Identity{id},
		nodesListMu:  &sync.Mutex{},

		fanOutStream:   make(chan *pb.SyncMessage, 1<<12),
		registerStream: make(chan Node, 1<<5),
	}

	go func() {
		nodes := make(map[string]Node, len(nodeAddresses))

		for {
			select {
			case node := <-m.registerStream:
				// don't requeue known nodes
				if _, ok := nodes[node.ID]; ok {
					continue
				}

				go m.registerNode(node)
				m.updateNodeList(nodes)
				// only store node once successfully established connection
				nodes[node.ID] = node

			case msg := <-m.fanOutStream:
				for _, node := range nodes {
					node.syncRequestChan <- msg
				}

				// TODO: handle node disconnect - remove from nodes && updateNodeList
			}
		}
	}()

	go func() {
		// wait a second for servers to bind
		time.Sleep(time.Second)

		for {
			m.identifyNodes()
			timer := time.NewTimer(time.Second * 15)
			<-timer.C
		}
	}()

	return m, nil
}

// TODO: handle conn shutdown - remove node from stores
func (m *Manager) registerNode(node Node) {
	logger := m.logger.With(zap.Any("identity", node.Identity))
	logger.Info("establishing sync connection", zap.Any("from", m.identity))

	conn, err := grpc.Dial(node.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("failed to connect to node server for sync", zap.Error(err))
		return
	}
	defer conn.Close()

	client := pb.NewKVStoreClient(conn)

	// create long polling synchronised connection to the node - do not
	// apply a timeout to the context as we want to keep this connection
	// open indefinitely
	syncStream, err := client.Sync(context.Background())
	if err != nil {
		logger.Error("failed to open sync stream with node", zap.Error(err))
		return
	}

	// pull sync requests from node's queue and send to node via node's
	// client
	for req := range node.syncRequestChan {
		// TODO: batch up sync requests, flush every 100ms/every 100 messages (opt configurable)
		if err := syncStream.Send(req); err != nil {
			// TODO: on error, retry x times. On retry failure, attempt to feed back into queue, or create persistent in-memory backup queue to reprocess later?
			logger.Error("failed to send sync request to node", zap.Error(err),
				zap.String("key", req.Key))

			if err == io.EOF {
				logger.Warn("sync stream closed due to EOF")
				break
			}
		}
	}
}

// FanOut returns the channel for fanning out sync messages.
func (m *Manager) FanOut() chan *pb.SyncMessage {
	return m.fanOutStream
}

func (m *Manager) updateNodeList(nodes map[string]Node) {
	nodeList := make([]identity.Identity, 0, len(nodes))

	m.nodesListMu.Lock()
	defer m.nodesListMu.Unlock()

	for _, n := range nodes {
		nodeList = append(nodeList, n.Identity)
	}
	nodeList = append(nodeList, m.identity)
	m.nodeList = nodeList
}

// Nodes returns all connected node identities. It is concurrency safe.
func (m *Manager) Nodes() []identity.Identity {
	m.nodesListMu.Lock()
	defer m.nodesListMu.Unlock()
	return m.nodeList
}

// TODO: refactor all into a nodes.Discoverer
// attempt to form the initial network with all nodes addressed in the startup flags
func (m *Manager) identifyNodes() {
	// build unique a list of all possible nodes, including initially known
	// nodes and nodes discovered after startup
	totalNodes := make(map[string]struct{}, len(m.initialNodes))
	for _, n := range m.initialNodes {
		totalNodes[n] = struct{}{}
	}
	for _, n := range m.Nodes() {
		if n.ID == m.identity.ID {
			continue
		}
		totalNodes[n.Address] = struct{}{}
	}

	if len(m.initialNodes) == 0 {
		m.logger.Warn("no initial startup node addresses defined")
		return
	}

	const (
		timeout   = time.Second * 5
		retryWait = time.Second
	)

	discoveredNodes := make(map[string]Node, len(totalNodes))
	for addr := range totalNodes {
		logger := m.logger.With(zap.String("addr", addr))

		func() {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			for {
				err := m.aggregateClusterNodes(ctx, addr, discoveredNodes)
				if err == nil {
					break
				}
				logger.Error("failed to aggregate cluster nodes", zap.Error(err))

				timeout := time.NewTicker(retryWait)
				select {
				case <-ctx.Done():
					logger.Error("timed out attempting to aggregate cluster nodes")
					return
				case <-timeout.C:
				}
			}
		}()
	}

	// we now have a list of all nodes that all other nodes are aware of, register them
	for _, node := range discoveredNodes {
		// m.logger.Info("identified node", zap.String("instance", m.identity.Name), zap.String("name", node.Name), zap.String("id", node.ID))
		m.registerStream <- node
	}
}

func (m *Manager) aggregateClusterNodes(ctx context.Context, nodeAddress string, discoveredNodes map[string]Node) error {
	// create client to connect to the node for identification
	conn, err := grpc.DialContext(ctx, nodeAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to node server for identification: %w", err)
	}
	defer conn.Close()

	client := pb.NewKVStoreClient(conn)

	idPayload := &pb.IdentityRequest{
		Node: m.identity.ToProto(),
	}

	resp, err := client.Identify(ctx, idPayload)
	if err != nil {
		return fmt.Errorf("failed to identify node: %w", err)
	}

	for _, node := range resp.GetNodes() {
		id := node.GetId()

		if id == m.identity.ID {
			continue
		}

		if _, ok := discoveredNodes[id]; ok {
			continue
		}

		newNode := Node{
			Identity:        identity.FromProto(node),
			syncRequestChan: make(chan *pb.SyncMessage, 1<<12),
		}

		discoveredNodes[id] = newNode
	}

	return nil
}
