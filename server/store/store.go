package store

import (
	"errors"
	"sort"
	"time"

	"github.com/OneOfOne/xxhash"
)

type Record struct {
	data       []byte
	operations []*Operation
}

func (r Record) latestOperation() *Operation {
	if len(r.operations) == 0 {
		return nil
	}
	return r.operations[len(r.operations)-1]
}

type OperationType uint

const (
	UpdateOp OperationType = iota
	DeleteOp
)

type Operation struct {
	opType       OperationType
	data         []byte
	timestamp    int64
	nodeCoverage uint8
}

var (
	// key is hashed key
	store = map[uint64]*Record{}

	ErrNotFound = errors.New("hash of key not found in store")
)

func Get(key string) ([]byte, error) {
	// get hash of key
	hash, err := hashKey(key)
	if err != nil {
		return nil, err
	}

	// retrieve value from map
	r, ok := store[hash]
	if !ok {
		return nil, ErrNotFound
	}
	// determine if record was deleted on last operation
	if r.latestOperation().opType == DeleteOp {
		return nil, ErrNotFound
	}

	return r.data, nil
}

func Put(key string, value []byte) error {
	return insertOperation(key, value, UpdateOp)
}

func Delete(key string) error {
	return insertOperation(key, nil, DeleteOp)
}

func insertOperation(key string, value []byte, opType OperationType) error {
	// get hash of key
	hash, err := hashKey(key)
	if err != nil {
		return err
	}

	// attempt to retrieve existing value from map
	r, ok := store[hash]
	// TODO: insert in order instead of sorting for every operation
	newOp := &Operation{
		opType:       opType,
		data:         value,
		timestamp:    time.Now().UTC().UnixNano(),
		nodeCoverage: 1,
	}
	r.operations = append(r.operations, newOp)

	// order operations by timestamp (if there is more than one operation)
	if ok {
		sort.Slice(r.operations, func(i, j int) bool {
			return r.operations[i].timestamp < r.operations[j].timestamp
		})
	}

	// update record's data to reflect most recent operation
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
