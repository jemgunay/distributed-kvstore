package nodes

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	petname "github.com/dustinkirkland/golang-petname"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/jemgunay/distributed-kvstore/pkg/config"
	pb "github.com/jemgunay/distributed-kvstore/pkg/proto"
)

// Node represents a single service node in the distributed store network.
type Node struct {
	Identity

	syncRequestChan chan *pb.SyncMessage
}

type NodeManager struct {
	logger   config.Logger
	identity Identity

	nodeList    []Identity
	nodesListMu *sync.Mutex

	registerStream chan Node
	fanOutStream   chan *pb.SyncMessage
}

func New(logger config.Logger, port int, nodeAddresses []string) (*NodeManager, error) {
	address := ":" + strconv.Itoa(port)
	id := NewIdentity(time.Now().UTC().UnixNano(), address)

	m := &NodeManager{
		logger:   logger,
		identity: id,

		nodeList:    []Identity{id},
		nodesListMu: &sync.Mutex{},

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

				if err := m.registerNode(node); err != nil {
					m.logger.Error("registering node in store failed", zap.Any("id", node.ID), zap.Error(err))
					continue
				}

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

	go m.identifyInitialNodes(nodeAddresses)

	return m, nil
}

func (m *NodeManager) registerNode(node Node) error {
	logger := m.logger.With(zap.Any("identity", node.Identity))
	logger.Info("registering node in store")

	// TODO: handle conn shutdown - remove node from stores
	go func() {
		conn, err := grpc.Dial(node.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			// return fmt.Errorf("failed to connect to node server: %w", err)
			logger.Error("failed to connect to node server", zap.Error(err))
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
					break
				}
			}
		}

		logger.Warn("sync stream closed due to EOF")
	}()

	return nil
}

func (m *NodeManager) FanOut() chan *pb.SyncMessage {
	return m.fanOutStream
}

func (m *NodeManager) updateNodeList(nodes map[string]Node) {
	nodeList := make([]Identity, 0, len(nodes))

	m.nodesListMu.Lock()
	defer m.nodesListMu.Unlock()

	for _, n := range nodes {
		nodeList = append(nodeList, n.Identity)
	}
	nodeList = append(nodeList, m.identity)
	m.nodeList = nodeList
}

func (m *NodeManager) Nodes() []Identity {
	m.nodesListMu.Lock()
	defer m.nodesListMu.Unlock()
	return m.nodeList
}

type Identity struct {
	StartTime            int64
	Address              string
	Name                 string
	ID                   string
	LastMessageTimestamp int64
}

func (i Identity) ToProto() *pb.Node {
	return &pb.Node{
		StartTime:              i.StartTime,
		Address:                i.Address,
		Name:                   i.Name,
		Id:                     i.ID,
		LatestMessageTimestamp: i.LastMessageTimestamp,
	}
}

func FromProto(node *pb.Node) Identity {
	return Identity{
		StartTime:            node.GetStartTime(),
		Address:              node.GetAddress(),
		Name:                 node.GetName(),
		ID:                   node.GetId(),
		LastMessageTimestamp: node.GetLatestMessageTimestamp(),
	}
}

func init() {
	petname.NonDeterministicMode()
}

func NewIdentity(startTime int64, address string) Identity {
	name := petname.Generate(3, "-")

	return Identity{
		StartTime:            startTime,
		Address:              address,
		Name:                 name,
		LastMessageTimestamp: 0,
		ID:                   strconv.FormatInt(startTime, 10) + "-" + name,
	}
}

// attempt to form the initial network with all nodes addressed in the startup flags
// TODO: do this on a fixed interval to detect and connect to any new nodes
func (m *NodeManager) identifyInitialNodes(nodeAddresses []string) {
	if len(nodeAddresses) == 0 {
		m.logger.Warn("no initial startup node addresses defined")
		return
	}

	time.Sleep(time.Second)

	const attempts = 10
	discoveredNodes := make(map[string]Node, len(nodeAddresses))
	for _, addr := range nodeAddresses {
		logger := m.logger.With(zap.String("addr", addr))

		for i := 0; i < attempts; i++ {
			if err := m.aggregateClusterNodes(logger, addr, discoveredNodes); err != nil {
				logger.Error("failed to identify node", zap.Error(err), zap.Int("attempt", i))
				time.Sleep(time.Second)
				continue
			}

			break
		}
	}

	// we now have a list of all nodes that all other nodes are aware of, register them
	for _, node := range discoveredNodes {
		m.logger.Info("identified node", zap.String("name", node.Name), zap.String("id", node.ID))
		m.registerStream <- node
	}
}

func (m *NodeManager) aggregateClusterNodes(logger config.Logger, nodeAddress string, discoveredNodes map[string]Node) error {
	// create client to connect to the node
	conn, err := grpc.DialContext(context.Background(), nodeAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to node server: %w", err)
	}
	defer conn.Close()

	client := pb.NewKVStoreClient(conn)

	idPayload := &pb.IdentityRequest{
		Node: m.identity.ToProto(),
	}

	// TODO: timeout on ctx
	resp, err := client.Identify(context.Background(), idPayload)
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
		logger.Info("discovered new node", zap.String("id", id))

		newNode := Node{
			Identity:        FromProto(node),
			syncRequestChan: make(chan *pb.SyncMessage, 1<<12),
		}

		discoveredNodes[id] = newNode
	}

	return nil
}
