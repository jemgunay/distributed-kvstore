package core

import (
	"context"

	"github.com/jemgunay/distributed-kvstore/pkg/nodes/identity"
	pb "github.com/jemgunay/distributed-kvstore/pkg/proto"
)

// Storer defines the store methods required by a replicated KV store.
type Storer interface {
	Get(key string) (value []byte, timestamp int64, err error)
	Put(key string, value []byte, timestamp int64) error
	Delete(key string, timestamp int64) error
	Subscribe(ctx context.Context, key string) (chan *pb.FetchResponse, error)

	// GetSyncStream returns a SyncMessage stream for polling messages to sync
	// to other nodes.
	GetSyncStream() <-chan *pb.SyncMessage
	// SyncIn receives a SyncMessage and attempts to insert the corresponding
	// operation into the store.
	SyncIn(*pb.SyncMessage) error
}

// Noder defines a KV node manager.
type Noder interface {
	FanOut() chan *pb.SyncMessage
	Nodes() []identity.Identity
}

// Client defines a KV Client.
type Client interface {
	Publish(key string, value any) error
	Fetch(key string, value any) (int64, error)
	Delete(key string) error
	Subscribe(key string) (chan *pb.FetchResponse, context.CancelFunc, error)
}
