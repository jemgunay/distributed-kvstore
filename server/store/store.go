package store

import "errors"

type Record struct {
	data       string
	operations []Operation
}

type OperationType uint

const (
	Create OperationType = iota
	Update
	Delete
)

type Operation struct {
	opType       OperationType
	delta        string
	timestamp    uint8
	nodeCoverage uint8
}

var (
	store = map[string]*Record{}

	ErrNotFound = errors.New("hash not found in store")
)

func Get(hash string) (Record, error) {
	r, ok := store[hash]
	if !ok {
		return r, ErrNotFound
	}
	return r, nil
}
