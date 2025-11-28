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

	// GetAll returns a copy of all stored variables.
	GetAll() map[string]string

	// Merge combines variables with a feeder record, where variables
	// take precedence over feeder record values. Returns the merged map.
	Merge(record map[string]string) map[string]string

	// Clear removes all stored variables.
	Clear()
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

// GetAll returns a copy of all stored variables.
func (m *MemoryStore) GetAll() map[string]string {
	result := make(map[string]string, len(m.variables))
	for key, value := range m.variables {
		result[key] = value
	}
	return result
}

// Merge combines variables with a feeder record, where variables
// take precedence over feeder record values. Returns the merged map.
func (m *MemoryStore) Merge(record map[string]string) map[string]string {
	result := make(map[string]string, len(record))

	// First, copy all feeder record values
	for key, value := range record {
		result[key] = value
	}

	// Then, override with variables (variables take precedence)
	for key, value := range m.variables {
		result[key] = value
	}

	return result
}

// Clear removes all stored variables.
func (m *MemoryStore) Clear() {
	m.variables = make(map[string]string)
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
