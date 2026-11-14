// Package variables provides variable storage for holding extracted values per-worker.
package variables

import (
	"context"
)

// Store defines the interface for variable storage.
type Store interface {
	// Set stores a variable with the given key and value.
	Set(key, value string)

	// Get retrieves a variable by key. Returns (value, true) if found,
	// or ("", false) if the key is not present.
	Get(key string) (string, bool)
}

// MemoryStore is a simple map-based implementation of the Store interface.
// It is designed for per-worker use and does not require mutex protection.
type MemoryStore struct {
	variables map[string]string
}

// NewStore creates and returns a new MemoryStore instance.
func NewStore() Store {
	return &MemoryStore{
		variables: make(map[string]string),
	}
}

// Set stores a variable with the given key and value.
func (m *MemoryStore) Set(key, value string) {
	m.variables[key] = value
}

// Get retrieves a variable by key. Returns (value, true) if found,
// or ("", false) if the key is not present.
func (m *MemoryStore) Get(key string) (string, bool) {
	value, ok := m.variables[key]
	return value, ok
}

type contextKey struct{}

var storeKey = contextKey{}

// FromContext retrieves the variable store from the context.
// Returns nil if not found.
func FromContext(ctx context.Context) Store {
	if ctx == nil {
		return nil
	}
	if s, ok := ctx.Value(storeKey).(Store); ok {
		return s
	}
	return nil
}

// NewContext returns a new context with the variable store attached.
func NewContext(ctx context.Context, store Store) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, storeKey, store)
}
