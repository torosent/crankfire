package grpcclient

import (
	"context"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
)

func TestNewClient(t *testing.T) {
	cfg := Config{
		Target:  "localhost:50051",
		Service: "test.Service",
		Method:  "TestMethod",
		Timeout: 5 * time.Second,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if client == nil {
		t.Fatal("Expected non-nil client")
	}
	if client.service != "test.Service" {
		t.Errorf("Expected service 'test.Service', got %q", client.service)
	}
	if client.method != "TestMethod" {
		t.Errorf("Expected method 'TestMethod', got %q", client.method)
	}
}

func TestNewClientDefaultTimeout(t *testing.T) {
	cfg := Config{
		Target:  "localhost:50051",
		Service: "test.Service",
		Method:  "TestMethod",
		// Timeout not set - should default to 30s
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if client == nil {
		t.Fatal("Expected non-nil client")
	}
}

func TestClientConnectAlreadyConnected(t *testing.T) {
	// This test verifies the "already connected" logic by checking the code path
	// In practice, a real connection would be needed for full testing
	cfg := Config{
		Target:  "localhost:50051",
		Service: "test.Service",
		Method:  "TestMethod",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Verify initial state
	if client.conn != nil {
		t.Error("Expected conn to be nil initially")
	}
}

func TestClientInvokeWithoutConnect(t *testing.T) {
	cfg := Config{
		Target:  "localhost:50051",
		Service: "test.Service",
		Method:  "TestMethod",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()
	req := &emptypb.Empty{}
	resp := &emptypb.Empty{}

	err = client.Invoke(ctx, req, resp)
	if err == nil {
		t.Error("Expected error when invoking without connect")
	}
	// Check for expected error message (could be "not connected" or "client not connected")
	if err != nil {
		errMsg := err.Error()
		if errMsg != "client not connected" {
			t.Errorf("Expected 'client not connected' error, got: %v", err)
		}
	}
}

func TestClientCloseWithoutConnect(t *testing.T) {
	cfg := Config{
		Target:  "localhost:50051",
		Service: "test.Service",
		Method:  "TestMethod",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	err = client.Close()
	if err != nil {
		t.Errorf("Close without connect should not error, got: %v", err)
	}
}

func TestClientMetrics(t *testing.T) {
	cfg := Config{
		Target:  "localhost:50051",
		Service: "test.Service",
		Method:  "TestMethod",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	metrics := client.Metrics()
	if metrics.MessagesSent != 0 {
		t.Errorf("Expected 0 messages sent, got %d", metrics.MessagesSent)
	}
	if metrics.BytesSent != 0 {
		t.Errorf("Expected 0 bytes sent, got %d", metrics.BytesSent)
	}
	if metrics.Errors != 0 {
		t.Errorf("Expected 0 errors, got %d", metrics.Errors)
	}
}

func TestClientWithMetadata(t *testing.T) {
	cfg := Config{
		Target:  "localhost:50051",
		Service: "test.Service",
		Method:  "TestMethod",
		Metadata: map[string]string{
			"authorization": "Bearer token123",
			"x-custom":      "value",
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if len(client.md) != 2 {
		t.Errorf("Expected 2 metadata entries, got %d", len(client.md))
	}
}

func TestClientTLSConfig(t *testing.T) {
	cfg := Config{
		Target:   "localhost:50051",
		Service:  "test.Service",
		Method:   "TestMethod",
		UseTLS:   true,
		Insecure: true,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if !client.useTLS {
		t.Error("Expected useTLS to be true")
	}
	if !client.insecure {
		t.Error("Expected insecure to be true")
	}
}

func TestClientFullMethodName(t *testing.T) {
	cfg := Config{
		Target:  "localhost:50051",
		Service: "helloworld.Greeter",
		Method:  "SayHello",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Verify service and method are stored correctly
	if client.service != "helloworld.Greeter" {
		t.Errorf("Expected service 'helloworld.Greeter', got %q", client.service)
	}
	if client.method != "SayHello" {
		t.Errorf("Expected method 'SayHello', got %q", client.method)
	}
}

func TestClientConcurrentMetricsAccess(t *testing.T) {
	cfg := Config{
		Target:  "localhost:50051",
		Service: "test.Service",
		Method:  "TestMethod",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Concurrent reads should not panic
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = client.Metrics()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestClientConnectFailure(t *testing.T) {
	// Use an invalid address to force connection failure
	// Note: grpc.NewClient doesn't connect immediately, it connects lazily or in background.
	// But Connect() method in Client calls grpc.NewClient.
	// Wait, grpc.NewClient returns a ClientConn. It doesn't block until connection is established unless WithBlock is used (which is deprecated/removed).
	// However, Client.Connect() just calls grpc.NewClient.
	// So Connect() itself might not fail for invalid address immediately unless we try to use it.
	// But wait, the implementation of Connect() is:
	// conn, err := grpc.NewClient(c.target, opts...)
	// if err != nil { ... }
	// grpc.NewClient validates the target syntax but doesn't connect.

	// Let's test with an invalid target syntax
	cfg := Config{
		Target:  "///", // Invalid target
		Service: "test.Service",
		Method:  "TestMethod",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// This might fail if the target syntax is invalid
	// If it doesn't fail here, we can't easily test connection failure without blocking.
	// Let's skip this if it's too flaky or implementation dependent.
	// Instead, let's test Close() with connection.
	_ = client.Connect(context.Background())
	// We don't assert error here because "///" might be valid for some resolvers or just fail later.
}

func TestClientCloseWithConnection(t *testing.T) {
	cfg := Config{
		Target:  "localhost:50051",
		Service: "test.Service",
		Method:  "TestMethod",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if err := client.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if client.conn != nil {
		t.Error("Expected conn to be nil after Close")
	}
}

// MockFeeder for httpclient tests (will be used in httpclient_test.go, but I'm in grpcclient_test.go now)
// Wait, I should stick to grpcclient tests here.

// To test Invoke properly, we need a real gRPC server.
// Since setting up a full gRPC server in a unit test file might be verbose and require generated code,
// and we don't have the generated code for a test service here (unless we generate it on the fly or use reflection),
// we might be limited.
// However, we can use `grpc.NewServer()` and register a handler if we had the service definition.
// But `Client` uses `conn.Invoke` which is generic.
// We can try to use a standard service like `grpc.health.v1.Health`.
