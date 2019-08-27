package store

import (
	"errors"
	"sort"
	"time"

	"github.com/OneOfOne/xxhash"
)

type Record struct {
	key        string
	data       []byte
	operations []*Operation
}

func (r Record) latestOperation() *Operation {
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

type Operation struct {
	opType       operationType
	data         []byte
	timestamp    int64
	nodeCoverage uint8
}

var (
	// map key is hashed key
	store = map[uint64]*Record{}

	ErrNotFound   = errors.New("hash of key not found in store")
	ErrInvalidKey = errors.New("invalid key provided")
)

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

func Put(key string, value []byte) error {
	return insertOperation(key, value, updateOp)
}

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
	newOp := &Operation{
		opType:       opType,
		data:         value,
		timestamp:    time.Now().UTC().UnixNano(),
		nodeCoverage: 1,
	}

	// attempt to retrieve existing value from map
	r, ok := store[hash]
	if !ok {
		// if there is no existing record, create a new one
		r = &Record{
			operations: []*Operation{newOp},
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

func hashKey(key string) (uint64, error) {
	h := xxhash.New64()
	if _, err := h.WriteString(key); err != nil {
		return 0, err
	}
	return h.Sum64(), nil
}
