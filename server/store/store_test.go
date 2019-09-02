package store

import (
	"bytes"
	"testing"
	"time"
)

func TestStore_Put(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value []byte
		err   error
	}{
		{"success","test", []byte("hello"), nil},
		{"success_empty_value","test", nil, nil},
		{"fail_empty_key","", []byte("hello"), ErrInvalidKey},
	}

	store := NewStore()
	store.StartPoller()
	defer store.Shutdown()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := store.Put(tt.key, tt.value, time.Now().UTC().UnixNano()); err != tt.err {
				t.Errorf("expected err of \"%s\", got err of \"%s\"", tt.err, err)
			}
		})
	}
}

func TestStore_Get(t *testing.T) {
	store := NewStore()
	store.StartPoller()
	defer store.Shutdown()

	// attempt to get something that doesn't exist in the store
	value, ts, err := store.Get("animal")
	if err != ErrNotFound {
		t.Fatalf("get unexpectedly succeeded, expected %s, got %s", ErrNotFound, err)
	}

	catValue := []byte("cat")
	catTimestamp := time.Now().UTC().UnixNano()
	if err := store.Put("animal", catValue, catTimestamp); err != nil {
		t.Fatalf("failed to put cat: %s", err)
	}

	// fetch something that does exist in the store
	value, ts, err = store.Get("animal")
	switch{
	case err != nil:
		t.Fatalf("failed to get cat, err: %s, expected nil", err)
	case !bytes.Equal(value, catValue):
		t.Fatalf("failed to get cat, value: %s, expected: %s", value, catValue)
	case ts != catTimestamp:
		t.Fatalf("failed to get cat, ts: %d, expected: %d", ts, catTimestamp)
	}
}
