package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStore_Put(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value []byte
		err   error
	}{
		{"success", "test", []byte("hello"), nil},
		{"success_empty_value", "test", nil, nil},
		{"fail_empty_key", "", []byte("hello"), ErrInvalidKey},
	}

	store := NewStore()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Put(tt.key, tt.value, time.Now().UTC().UnixNano())
			require.ErrorIs(t, err, tt.err)
		})
	}
}

// TODO: restore this once we can inject override opts into the store creation
/*func TestStore_PutBufferFull(t *testing.T) {
	requestChanBufSize = 0
	store := NewStore()

	catValue := []byte("cat")
	catTimestamp := time.Now().UTC().UnixNano()

	err := store.Put("animal", catValue, catTimestamp)
	require.ErrorIs(t, err, ErrStoreBackpressure)
}*/

func TestStore_Get(t *testing.T) {
	store := NewStore()

	// attempt to get something that doesn't exist in the store
	value, ts, err := store.Get("animal")
	require.ErrorIs(t, err, ErrNotFound)

	catValue := []byte("cat")
	catTimestamp := time.Now().UTC().UnixNano()
	err = store.Put("animal", catValue, catTimestamp)
	require.NoError(t, err)

	// fetch something that does exist in the store
	value, ts, err = store.Get("animal")
	require.NoError(t, err)
	require.Equal(t, value, catValue)
	require.Equal(t, ts, catTimestamp)
}

func TestStore_Delete(t *testing.T) {
	store := NewStore()

	// attempt to get something that doesn't exist in the store
	if _, _, err := store.Get("animal"); err != ErrNotFound {
		t.Fatalf("get unexpectedly succeeded, expected %s, got %s", ErrNotFound, err)
	}

	// create element in store
	catValue := []byte("cat")
	catTimestamp := time.Now().UTC().UnixNano()
	err := store.Put("animal", catValue, catTimestamp)
	require.NoError(t, err)

	// delete from store
	err = store.Delete("animal", time.Now().UTC().UnixNano())
	require.NoError(t, err)

	// attempt to get something that doesn't exist in the store
	_, _, err = store.Get("animal")
	require.ErrorIs(t, err, ErrNotFound)
}
