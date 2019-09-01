package store

import (
	"errors"
	"sort"
	"time"

	"github.com/OneOfOne/xxhash"

	pb "github.com/jemgunay/distributed-kvstore/proto"
)

// represents a record in the store
type record struct {
	key        string
	operations []*operation
}

// returns the latest operation stored in a record
func (r record) latestOperation() *operation {
	if len(r.operations) == 0 {
		return nil
	}
	return r.operations[len(r.operations)-1]
}

// operation represents a request to update or delete a record - these are used to maintain the order of incoming
// operations to prevent data loss resulting from the processing of incorrectly ordered requests
type operation struct {
	opType    pb.OperationType
	data      []byte
	timestamp int64
	// used to determine which nodes this operation has synchronised across
	nodeSYNCoverage uint64
	nodeACKCoverage uint64
}

// Store is a operation based KV store to facilitate a distributed server implementation.
type Store struct {
	// a map of key/value pairs where the key is the hashed key and the value is the data record
	store map[uint64]*record

	// RequestChanBufSize is the size of each of the store poller request channel buffers.
	RequestChanBufSize int64

	getReqChan    chan getReq
	insertReqChan chan insertReq
	//deleteReqChan chan deleteReq
	//syncReqChan   chan *pb.SyncMessage

	syncRequestFeedChan chan *pb.SyncMessage
}

// NewStore initialises and returns a new KV store.
func NewStore() *Store {
	return &Store{
		store:               map[uint64]*record{},
		RequestChanBufSize:  1 << 10, // 1024
		syncRequestFeedChan: make(chan *pb.SyncMessage, 1<<10),
	}
}

var (
	// ErrNotFound indicates that the provided key does not exist in the store.
	ErrNotFound = errors.New("key not found in store")
	// ErrInvalidKey indicates that an invalid key (likely an empty string) was provided.
	ErrInvalidKey = errors.New("invalid key provided")
	// ErrPollerBufferFull indicates that the poller channel buffer is full and cannot consume new requests.
	ErrPollerBufferFull = errors.New("request failed as poller channel buffer is full")
)

// Get retrieves a record from the store.
func (s *Store) Get(key string) ([]byte, int64, error) {
	req := getReq{
		key:    key,
		respCh: make(chan getResp),
	}
	// if buffer is full, fail request with error
	select {
	case s.getReqChan <- req:
		resp := <-req.respCh
		return resp.data, resp.timestamp, resp.err
	default:
		return nil, 0, ErrPollerBufferFull
	}
}

// called by the poller to serialise get request operations on the store map
func (s *Store) performGetOperation(key string) getResp {
	if key == "" {
		return getResp{err: ErrInvalidKey}
	}

	// get hash of key
	hash, err := hashKey(key)
	if err != nil {
		return getResp{err: err}
	}

	// retrieve value from map
	record, ok := s.store[hash]
	if !ok {
		return getResp{err: ErrNotFound}
	}

	// determine if record was deleted on last operation
	lastOp := record.latestOperation()
	if lastOp.opType == pb.OperationType_DELETE {
		return getResp{err: ErrNotFound}
	}

	return getResp{data: lastOp.data, timestamp: lastOp.timestamp}
}

// Put either creates a new record or amends the state of an existing record in the store.
func (s *Store) Put(key string, value []byte, timestamp int64) error {
	req := insertReq{
		key:           key,
		value:         value,
		timestamp:     timestamp,
		operationType: pb.OperationType_UPDATE,
		performSync:   true,
		respCh:        make(chan error),
	}
	// if buffer is full, fail request with error
	select {
	case s.insertReqChan <- req:
		return <-req.respCh
	default:
		return ErrPollerBufferFull
	}
}

// Delete performs a store record deletion.
func (s *Store) Delete(key string, timestamp int64) error {
	req := insertReq{
		key:           key,
		timestamp:     timestamp,
		operationType: pb.OperationType_UPDATE,
		performSync:   true,
		respCh:        make(chan error),
	}
	// if buffer is full, fail request with error
	select {
	case s.insertReqChan <- req:
		return <-req.respCh
	default:
		return ErrPollerBufferFull
	}
}

// called by the poller to serialise update and delete request operations on the store map
func (s *Store) performInsertOperation(key string, value []byte, timestamp int64, opType pb.OperationType, performSync bool) error {
	if key == "" {
		return ErrInvalidKey
	}

	// get hash of key
	hash, err := hashKey(key)
	if err != nil {
		return err
	}

	// construct new operation record
	newOp := &operation{
		opType:          opType,
		data:            value,
		timestamp:       time.Now().UTC().UnixNano(),
		nodeSYNCoverage: 1,
		nodeACKCoverage: 1,
	}

	// attempt to retrieve existing value from map
	r, ok := s.store[hash]
	if !ok {
		// if there is no existing record, create a new one
		r = &record{
			operations: []*operation{newOp},
		}
	} else {
		// if record exists, append new operation order operations by timestamp (if there is more than one operation)
		// TODO: insert in order instead of sorting for every operation
		r.operations = append(r.operations, newOp)

		sort.Slice(r.operations, func(i, j int) bool {
			return r.operations[i].timestamp < r.operations[j].timestamp
		})
	}

	r.key = key
	s.store[hash] = r

	if performSync {
		s.syncRequestFeedChan <- &pb.SyncMessage{
			Key:       key,
			Value:     value,
			Timestamp: timestamp,

			OperationType: pb.OperationType_UPDATE,
			SyncType:      pb.SyncType_SYN,
		}
	}

	return nil
}

func (s *Store) SyncOut() *pb.SyncMessage {
	return <-s.syncRequestFeedChan
}

func (s *Store) SyncIn(syncMsg *pb.SyncMessage) (err error) {
	req := insertReq{
		key:           syncMsg.Key,
		value:         syncMsg.Value,
		timestamp:     syncMsg.Timestamp,
		operationType: syncMsg.OperationType,
		performSync:   false,
		respCh:        make(chan error),
	}
	// if buffer is full, fail request with error
	select {
	case s.insertReqChan <- req:
		return <-req.respCh
	default:
		return ErrPollerBufferFull
	}
}

// hashes the specified key with very fast non-cryptographic hashing algorithm
func hashKey(key string) (uint64, error) {
	h := xxhash.New64()
	if _, err := h.WriteString(key); err != nil {
		return 0, err
	}
	return h.Sum64(), nil
}
