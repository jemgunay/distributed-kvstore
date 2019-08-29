package store

import (
	"errors"
	"sort"
	"time"

	"github.com/OneOfOne/xxhash"
)

// KVStorer is the interface that wraps the store methods required by a KV store.
type KVStorer interface {
	Get(key string) ([]byte, int64, error)
	Put(key string, value []byte) error
	Delete(key string) error
}

// represents a record in the store
type record struct {
	key        string
	data       []byte
	operations []*operation
}

// returns the latest operation stored in a record
func (r record) latestOperation() *operation {
	if len(r.operations) == 0 {
		return nil
	}
	return r.operations[len(r.operations)-1]
}

type operationType uint

const (
	updateOp operationType = iota
	deleteOp
)

// operation represents a request to update or delete a record - these are used to maintain the order of incoming
// operations to prevent data loss resulting from the processing of incorrectly ordered requests
type operation struct {
	opType       operationType
	data         []byte
	timestamp    int64
	nodeCoverage uint8
}

// Store is a operation based KV store to facilitate a distributed server implementation.
type Store struct {
	// a map of key/value pairs where the key is the hashed key and the value is the data record
	store map[uint64]*record

	// RequestChanBufSize is the size of each of the store poller request channel buffers.
	RequestChanBufSize int64

	getReqChan    chan getReq
	putReqChan    chan putReq
	deleteReqChan chan deleteReq
}

// NewStore initialises and returns a new KV store.
func NewStore() *Store {
	return &Store{
		store:              map[uint64]*record{},
		RequestChanBufSize: 1 << 10, // 1024
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
	if lastOp.opType == deleteOp {
		return getResp{err: ErrNotFound}
	}

	return getResp{data: record.data, timestamp: lastOp.timestamp}
}

// Put either creates a new record or amends the state of an existing record in the store.
func (s *Store) Put(key string, value []byte) error {
	req := putReq{
		key:    key,
		value:  value,
		respCh: make(chan error),
	}
	// if buffer is full, fail request with error
	select {
	case s.putReqChan <- req:
		return <-req.respCh
	default:
		return ErrPollerBufferFull
	}
}

// Delete performs a store record deletion.
func (s *Store) Delete(key string) error {
	req := deleteReq{
		key:    key,
		respCh: make(chan error),
	}
	// if buffer is full, fail request with error
	select {
	case s.deleteReqChan <- req:
		return <-req.respCh
	default:
		return ErrPollerBufferFull
	}
}

// called by the poller to serialise update and delete request operations on the store map
func (s *Store) performInsertOperation(key string, value []byte, opType operationType) error {
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
		opType:       opType,
		data:         value,
		timestamp:    time.Now().UTC().UnixNano(),
		nodeCoverage: 1,
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

	// update record's data to reflect most recent operation
	r.key = key
	r.data = r.latestOperation().data

	s.store[hash] = r
	return nil
}

// hash keys with very fast non-cryptographic hashing algorithm
func hashKey(key string) (uint64, error) {
	h := xxhash.New64()
	if _, err := h.WriteString(key); err != nil {
		return 0, err
	}
	return h.Sum64(), nil
}
