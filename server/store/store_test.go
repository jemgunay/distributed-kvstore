package store

import "testing"

func TestStore_PutSuccess(t *testing.T) {
	tests := []struct {
		key   string
		value interface{}
	}{
		{"string_key", ""},
	}

	store := NewStore()
	store.StartPoller()
	defer store.Shutdown()

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if err := store.Put(tt.key, tt.value, 0); err != nil {

			}
		})
	}
}
