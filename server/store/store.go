package store

import (
	"errors"
	"sort"
	"time"

	"github.com/OneOfOne/xxhash"
)

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

// operations types
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

var (
	// a map of key/value pairs where the key is the hashed key and the value is the data record
	store = map[uint64]*record{}

	// ErrNotFound indicates that the provided key does not exist in the store.
	ErrNotFound = errors.New("key not found in store")
	// ErrInvalidKey indicates that an invalid key (likely an empty string) was provided.
	ErrInvalidKey = errors.New("invalid key provided")
	// ErrPollerBufferFull indicates that the poller channel buffer is full and cannot consume new requests.
	ErrPollerBufferFull = errors.New("request failed as poller channel buffer is full")
)

// Get retrieves a record from the store.
func Get(key string) ([]byte, int64, error) {
	req := getReq{
		key:    key,
		respCh: make(chan getResp),
	}
	// if buffer is full, fail request with error
	select {
	case getReqChan <- req:
		resp := <-req.respCh
		return resp.data, resp.timestamp, resp.err
	default:
		return nil, 0, ErrPollerBufferFull
	}
}

// called by the poller to serialise get request operations on the store map
func performGet(key string) getResp {
	if key == "" {
		return getResp{err: ErrInvalidKey}
	}

	// get hash of key
	hash, err := hashKey(key)
	if err != nil {
		return getResp{err: err}
	}

	// retrieve value from map
	record, ok := store[hash]
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
func Put(key string, value []byte) error {
	req := putReq{
		key:    key,
		value:  value,
		respCh: make(chan error),
	}
	// if buffer is full, fail request with error
	select {
	case putReqChan <- req:
		return <-req.respCh
	default:
		return ErrPollerBufferFull
	}
}

// Delete performs a store record deletion.
func Delete(key string) error {
	req := deleteReq{
		key:    key,
		respCh: make(chan error),
	}
	// if buffer is full, fail request with error
	select {
	case deleteReqChan <- req:
		return <-req.respCh
	default:
		return ErrPollerBufferFull
	}
}

// called by the poller to serialise update and delete request operations on the store map
func performInsertOperation(key string, value []byte, opType operationType) error {
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
	r, ok := store[hash]
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

	store[hash] = r
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
