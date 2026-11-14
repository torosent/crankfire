package variables

import (
	"context"
	"testing"
)

func TestMemoryStore_SetGet(t *testing.T) {
	store := NewStore()
	store.Set("username", "john")
	store.Set("token", "abc123")

	value, ok := store.Get("username")
	if !ok {
		t.Fatal("expected to find 'username' key")
	}
	if value != "john" {
		t.Errorf("expected 'john', got %q", value)
	}

	value, ok = store.Get("token")
	if !ok {
		t.Fatal("expected to find 'token' key")
	}
	if value != "abc123" {
		t.Errorf("expected 'abc123', got %q", value)
	}
}

func TestMemoryStore_GetMissing(t *testing.T) {
	store := NewStore()
	store.Set("username", "john")

	value, ok := store.Get("missing_key")
	if ok {
		t.Errorf("expected ok=false for missing key, got ok=true with value %q", value)
	}
	if value != "" {
		t.Errorf("expected empty string for missing key, got %q", value)
	}
}

func TestContextWithVariableStore(t *testing.T) {
	store := NewStore()
	store.Set("key", "value")

	ctx := context.Background()
	ctxWithStore := NewContext(ctx, store)

	retrieved := FromContext(ctxWithStore)
	if retrieved == nil {
		t.Fatalf("FromContext() returned nil")
	}

	val, ok := retrieved.Get("key")
	if !ok || val != "value" {
		t.Errorf("retrieved store has wrong value: got %q, want %q", val, "value")
	}
}

func TestVariableStoreFromContext(t *testing.T) {
	// Test with context without store
	ctx := context.TODO()
	got := FromContext(ctx)
	if got != nil {
		t.Errorf("FromContext(no store) = %v, want nil", got)
	}

	// Test with context with store
	store := NewStore()
	ctx = NewContext(ctx, store)
	got = FromContext(ctx)
	if got != store {
		t.Errorf("FromContext(with store) returned wrong store")
	}
}
