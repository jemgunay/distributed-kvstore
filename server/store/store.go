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

type operationType uint

const (
	updateOp operationType = iota
	deleteOp
)

type operation struct {
	opType       operationType
	data         []byte
	timestamp    int64
	nodeCoverage uint8
}

var (
	// map key is hashed key
	store = map[uint64]*record{}

	// ErrNotFound indicates that the provided key does not exist in the store.
	ErrNotFound = errors.New("key not found in store")
	// ErrInvalidKey indicates that an invalid key (likely an empty string) was provided.
	ErrInvalidKey = errors.New("invalid key provided")
)

// Get retrieves a record from the store.
func Get(key string) ([]byte, int64, error) {
	if key == "" {
		return nil, 0, ErrInvalidKey
	}

	// get hash of key
	hash, err := hashKey(key)
	if err != nil {
		return nil, 0, err
	}

	// retrieve value from map
	record, ok := store[hash]
	if !ok {
		return nil, 0, ErrNotFound
	}
	// determine if record was deleted on last operation
	lastOp := record.latestOperation()
	if lastOp.opType == deleteOp {
		return nil, 0, ErrNotFound
	}

	return record.data, lastOp.timestamp, nil
}

// Put either creates a new record or amends the state of an existing record in the store.
func Put(key string, value []byte) error {
	return insertOperation(key, value, updateOp)
}

// Delete performs a store record deletion.
func Delete(key string) error {
	return insertOperation(key, nil, deleteOp)
}

func insertOperation(key string, value []byte, opType operationType) error {
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

// very fast non-cryptographic hashing algorithm used for hashing keys
func hashKey(key string) (uint64, error) {
	h := xxhash.New64()
	if _, err := h.WriteString(key); err != nil {
		return 0, err
	}
	return h.Sum64(), nil
}
