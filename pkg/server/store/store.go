// Package store implements a serialised access KV store. Each record is composed of a list of latestOp so that
// synchronising across nodes can be achieved whilst maintaining the order of latestOp, preventing data loss.
package store

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/OneOfOne/xxhash"

	pb "github.com/jemgunay/distributed-kvstore/pkg/proto"
	"github.com/jemgunay/distributed-kvstore/pkg/server"
)

// represents a key/value record in the store
type record struct {
	key       string
	value     []byte
	timestamp int64
	operation pb.OperationVariant
}

type subscription struct {
	key    string
	id     uint64
	stream chan *pb.FetchResponse
	ctx    context.Context
}

var _ server.Storer = (*Store)(nil)

// Store is an operation-based KV store to facilitate a distributed server implementation.
type Store struct {
	// the key is the hashed key and the value is the data record
	store map[uint64]record

	// map[key]map[subscriber-id]subscription
	subscriptions map[string]map[uint64]subscription
	// nextSubscriptionID is atomically incremented
	nextSubscriptionID uint64

	// channels used to serialise access to the store via the store request poller
	requestChanBufSize   int64
	getReqQueue          chan getReq
	modifyReqQueue       chan modifyReq
	subscribeQueue       chan subscription
	syncRequestFeedQueue chan *pb.SyncMessage
}

// TODO: opt override for these
var (
	// requestChanBufSize is the size of each of the store poller request channel buffers.
	requestChanBufSize = int64(1 << 10) // 1024
	// syncRequestFeedChanBufSize is the size of sync request channel buffer.
	syncRequestFeedChanBufSize = int64(1 << 12) // 4096
)

// NewStore initialises and returns a new KV store.
func NewStore() *Store {
	s := &Store{
		store:                make(map[uint64]record, 100), // opt initial capacity
		requestChanBufSize:   requestChanBufSize,
		subscriptions:        make(map[string]map[uint64]subscription),
		getReqQueue:          make(chan getReq, requestChanBufSize),
		modifyReqQueue:       make(chan modifyReq, requestChanBufSize),
		subscribeQueue:       make(chan subscription, requestChanBufSize),
		syncRequestFeedQueue: make(chan *pb.SyncMessage, syncRequestFeedChanBufSize),
	}

	go s.startPoller()
	return s
}

var (
	// ErrNotFound indicates that the provided key does not exist in the store.
	ErrNotFound = errors.New("key not found in store")
	// ErrInvalidKey indicates that the provided key was empty.
	ErrInvalidKey = errors.New("invalid key provided")
	// ErrStoreBackpressure indicates that there is backpressure on the poller channel buffer, so it cannot consume new
	// requests.
	ErrStoreBackpressure = errors.New("request failed as there is backpressure on the store queue")
)

// Get retrieves a record's value from the store, as well as the timestamp the last update occurred at.
func (s *Store) Get(key string) ([]byte, int64, error) {
	req := getReq{
		key:    key,
		respCh: make(chan getResp, 1),
	}
	// if buffer is full, fail request with error
	select {
	case s.getReqQueue <- req:
		resp := <-req.respCh
		return resp.data, resp.timestamp, resp.err
	default:
		return nil, 0, ErrStoreBackpressure
	}
}

// called by the poller to serialise get request latestOp on the store map
func (s *Store) performGetOperation(key string) getResp {
	if key == "" {
		return getResp{err: ErrInvalidKey}
	}

	hash := hashKey(key)
	record, ok := s.store[hash]
	if !ok {
		return getResp{err: ErrNotFound}
	}

	// determine if record was deleted on last operation
	if record.operation == pb.OperationVariant_DELETE {
		return getResp{err: ErrNotFound}
	}

	return getResp{
		data:      record.value,
		timestamp: record.timestamp,
	}
}

// Put either creates a new record or amends the state of an existing record in the store.
func (s *Store) Put(key string, value []byte, timestamp int64) error {
	req := modifyReq{
		key:         key,
		value:       value,
		timestamp:   timestamp,
		operation:   pb.OperationVariant_UPDATE,
		performSync: true,
		respCh:      make(chan error, 1),
	}
	// if buffer is full, fail request with error
	select {
	case s.modifyReqQueue <- req:
		return <-req.respCh
	default:
		return ErrStoreBackpressure
	}
}

// Delete performs a store record deletion and triggers a sync request upon successful deletion.
func (s *Store) Delete(key string, timestamp int64) error {
	req := modifyReq{
		key:         key,
		timestamp:   timestamp,
		operation:   pb.OperationVariant_DELETE,
		performSync: true,
		respCh:      make(chan error, 1),
	}
	// if buffer is full, fail request with error
	select {
	case s.modifyReqQueue <- req:
		return <-req.respCh
	default:
		return ErrStoreBackpressure
	}
}

// called by the poller to serialise update and delete request latestOp on the store map
func (s *Store) performModifyOperation(req modifyReq) error {
	if req.key == "" {
		return ErrInvalidKey
	}

	// attempt to retrieve existing value from map
	hash := hashKey(req.key)
	rec, ok := s.store[hash]
	if ok {
		if req.timestamp <= rec.timestamp {
			// we've received a stale message, we can ignore the request
			return nil
		}
	}

	// if there is no existing record, create a new one
	s.store[hash] = record{
		key:       req.key,
		value:     req.value,
		timestamp: req.timestamp,
		operation: req.operation,
	}

	if req.performSync {
		// place sync request into sync feed queue (for non-sync requests only) so that the server can propagate this
		// request to other nodes
		s.syncRequestFeedQueue <- &pb.SyncMessage{
			Key:       req.key,
			Value:     req.value,
			Timestamp: req.timestamp,
			Operation: req.operation,
		}
	}

	return nil
}

// Subscribe hooks into the store to listen for insert requests from other nodes or clients. These are then forwarded
// on to the consumer via the response channel.
func (s *Store) Subscribe(ctx context.Context, key string) (chan *pb.FetchResponse, error) {
	newID := atomic.AddUint64(&s.nextSubscriptionID, 1)
	newSub := subscription{
		key: key,
		id:  newID,
		ctx: ctx,
		// create the channel for pushing updates to the subscriber
		stream: make(chan *pb.FetchResponse, s.requestChanBufSize),
	}

	select {
	case s.subscribeQueue <- newSub:
		return newSub.stream, nil
	default:
		return nil, ErrStoreBackpressure
	}
}

func (s *Store) registerSubscription(sub subscription) {
	// ensure key has a subscription map before we append subscriber
	if s.subscriptions[sub.key] == nil {
		s.subscriptions[sub.key] = make(map[uint64]subscription, 1)
	}

	s.subscriptions[sub.key][sub.id] = sub
}

// GetSyncStream returns the store's sync stream channel.
func (s *Store) GetSyncStream() <-chan *pb.SyncMessage {
	return s.syncRequestFeedQueue
}

// SyncIn creates a store insert request which will be consumed by the store poller, serialising access to the store's
// map. Unlike Put and Delete, this operation will not produce a sync request to other nodes.
func (s *Store) SyncIn(syncMsg *pb.SyncMessage) error {
	req := modifyReq{
		key:         syncMsg.GetKey(),
		value:       syncMsg.GetValue(),
		timestamp:   syncMsg.GetTimestamp(),
		operation:   syncMsg.GetOperation(),
		performSync: false,
		respCh:      make(chan error, 1),
	}
	// if buffer is full, fail request with error
	select {
	case s.modifyReqQueue <- req:
		return <-req.respCh
	default:
		return ErrStoreBackpressure
	}
}

func hashKey(key string) uint64 {
	h := xxhash.New64()
	h.WriteString(key) // can't actually error
	return h.Sum64()
}
