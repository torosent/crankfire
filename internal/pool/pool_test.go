package pool

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

type mockClient struct {
	connected   bool
	closed      bool
	failConnect bool
}

func (m *mockClient) Connect(ctx context.Context) error {
	if m.failConnect {
		return fmt.Errorf("connection failed")
	}
	m.connected = true
	return nil
}

func (m *mockClient) Close() error {
	m.closed = true
	m.connected = false
	return nil
}

func TestConnectionPool_GetPut(t *testing.T) {
	pool := NewConnectionPool(5)

	factory := func() Poolable {
		return &mockClient{}
	}

	// Get a new client
	client1, reused1 := pool.Get("key1", factory)
	if reused1 {
		t.Error("Expected new client, got reused")
	}
	if client1 == nil {
		t.Fatal("Expected client, got nil")
	}

	// Put it back
	if err := pool.Put("key1", client1); err != nil {
		t.Errorf("Put failed: %v", err)
	}

	// Get it again, should be reused
	client2, reused2 := pool.Get("key1", factory)
	if !reused2 {
		t.Error("Expected reused client, got new")
	}
	if client2 != client1 {
		t.Error("Expected same client instance")
	}
}

func TestConnectionPool_Close(t *testing.T) {
	pool := NewConnectionPool(5)

	client := &mockClient{}
	pool.Put("key1", client)

	if err := pool.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if !client.closed {
		t.Error("Expected client to be closed")
	}
}

func TestMakePoolKey(t *testing.T) {
	headers1 := make(http.Header)
	headers1.Set("Authorization", "Bearer token")
	headers1.Set("Content-Type", "application/json")

	headers2 := make(http.Header)
	headers2.Set("Content-Type", "application/json")
	headers2.Set("Authorization", "Bearer token")

	key1 := MakePoolKey("http://example.com", headers1)
	key2 := MakePoolKey("http://example.com", headers2)

	if key1 != key2 {
		t.Error("Expected same key for same headers in different order")
	}

	headers3 := make(http.Header)
	headers3.Set("Authorization", "Bearer different")
	key3 := MakePoolKey("http://example.com", headers3)

	if key1 == key3 {
		t.Error("Expected different keys for different headers")
	}
}

func TestConnectionPool_RetryStaleConnection(t *testing.T) {
	pool := NewConnectionPool(5)

	staleClient := &mockClient{connected: true}

	factory := func() Poolable {
		return &mockClient{}
	}

	newClient, ok := pool.RetryStaleConnection(context.Background(), staleClient, factory)
	if !ok {
		t.Error("Expected successful retry")
	}
	if newClient == nil {
		t.Error("Expected new client")
	}
	if !staleClient.closed {
		t.Error("Expected stale client to be closed")
	}

	mock := newClient.(*mockClient)
	if !mock.connected {
		t.Error("Expected new client to be connected")
	}
}

func TestConnectionPool_RetryStaleConnection_Failure(t *testing.T) {
	pool := NewConnectionPool(5)

	staleClient := &mockClient{connected: true}

	factory := func() Poolable {
		return &mockClient{failConnect: true}
	}

	newClient, ok := pool.RetryStaleConnection(context.Background(), staleClient, factory)
	if ok {
		t.Error("Expected failed retry")
	}
	if newClient != nil {
		t.Error("Expected nil client on failure")
	}
}
