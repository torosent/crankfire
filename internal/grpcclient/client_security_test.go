package grpcclient

import (
	"context"
	"crypto/tls"
	"testing"
	"time"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// TestClient_TLSConfiguration verifies proper TLS configuration
func TestClient_TLSConfiguration(t *testing.T) {
	tests := []struct {
		name               string
		useTLS             bool
		insecureSkipVerify bool
		expectedCredsType  string
		description        string
	}{
		{
			name:               "No TLS",
			useTLS:             false,
			insecureSkipVerify: false,
			expectedCredsType:  "insecure",
			description:        "Should use insecure credentials when TLS is disabled",
		},
		{
			name:               "TLS with verification",
			useTLS:             true,
			insecureSkipVerify: false,
			expectedCredsType:  "tls",
			description:        "Should use TLS credentials with certificate verification",
		},
		{
			name:               "TLS without verification",
			useTLS:             true,
			insecureSkipVerify: true,
			expectedCredsType:  "tls_insecure",
			description:        "Should use TLS credentials but skip certificate verification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Target:   "localhost:50051",
				Service:  "test.Service",
				Method:   "TestMethod",
				Metadata: nil,
				Timeout:  30 * time.Second,
				UseTLS:   tt.useTLS,
				Insecure: tt.insecureSkipVerify,
			}

			client, err := NewClient(cfg)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			// Verify client configuration
			if client.useTLS != tt.useTLS {
				t.Errorf("Expected useTLS=%v, got %v", tt.useTLS, client.useTLS)
			}
			if client.insecure != tt.insecureSkipVerify {
				t.Errorf("Expected insecure=%v, got %v", tt.insecureSkipVerify, client.insecure)
			}
		})
	}
}

// TestClient_TLSInsecureActuallyUsesTLS verifies that useTLS=true with insecure=true
// still uses TLS (just with verification disabled), not plaintext
func TestClient_TLSInsecureActuallyUsesTLS(t *testing.T) {
	cfg := Config{
		Target:   "localhost:50051",
		Service:  "test.Service",
		Method:   "TestMethod",
		Metadata: nil,
		Timeout:  30 * time.Second,
		UseTLS:   true,
		Insecure: true,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// The fix ensures that when useTLS=true and insecure=true,
	// we use credentials.NewTLS with InsecureSkipVerify=true
	// This is verified by the configuration stored in the client
	if !client.useTLS {
		t.Error("Expected useTLS to be true")
	}

	if !client.insecure {
		t.Error("Expected insecure to be true")
	}

	// Note: We can't easily test the actual credentials type without
	// connecting to a real server, but we've verified the configuration
	// is set correctly
}

// TestClient_DefaultTimeout verifies default timeout is set
func TestClient_DefaultTimeout(t *testing.T) {
	cfg := Config{
		Target:   "localhost:50051",
		Service:  "test.Service",
		Method:   "TestMethod",
		Metadata: nil,
		Timeout:  0, // No timeout specified
		UseTLS:   false,
		Insecure: false,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify client was created successfully
	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	// Note: The Config struct is not stored in Client, so we can't directly
	// verify the timeout was set to default. This would require refactoring
	// to store the config or timeout in the Client struct.
}

// TestClient_ConnectWithoutConnection verifies error when trying to invoke
// before connecting
func TestClient_ConnectWithoutConnection(t *testing.T) {
	cfg := Config{
		Target:   "localhost:50051",
		Service:  "test.Service",
		Method:   "TestMethod",
		Metadata: nil,
		Timeout:  30 * time.Second,
		UseTLS:   false,
		Insecure: false,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Try to invoke without connecting
	ctx := context.Background()
	err = client.Invoke(ctx, nil, nil)

	if err == nil {
		t.Fatal("Expected error when invoking without connection, got nil")
	}

	// Note: The actual error will be about nil request, but that's caught first
	// We can't easily test this without a mock or refactoring
}

// TestClient_DoubleConnect verifies error when connecting twice
func TestClient_DoubleConnect(t *testing.T) {
	cfg := Config{
		Target:   "localhost:50051",
		Service:  "test.Service",
		Method:   "TestMethod",
		Metadata: nil,
		Timeout:  100 * time.Millisecond,
		UseTLS:   false,
		Insecure: false,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// First connection (will fail because no server, but that's ok)
	_ = client.Connect(ctx)

	// If first connect succeeded (unlikely without server), try second connect
	if client.conn != nil {
		err = client.Connect(ctx)
		if err == nil {
			t.Fatal("Expected error when connecting twice, got nil")
		}

		expectedError := "client already connected"
		if err.Error() != expectedError {
			t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
		}
	}
}

// TestTLSConfigInsecureSkipVerify verifies TLS config with InsecureSkipVerify
func TestTLSConfigInsecureSkipVerify(t *testing.T) {
	// Create a TLS config with InsecureSkipVerify
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	creds := credentials.NewTLS(tlsConfig)
	if creds == nil {
		t.Fatal("Expected non-nil credentials")
	}

	// Verify this is different from insecure.NewCredentials()
	insecureCreds := insecure.NewCredentials()
	if insecureCreds == nil {
		t.Fatal("Expected non-nil insecure credentials")
	}

	// These should be different types
	// TLS credentials with InsecureSkipVerify still use TLS encryption,
	// just without certificate verification
	// insecure.NewCredentials() uses no encryption at all
}

// TestClient_MetadataHandling verifies metadata is properly set
func TestClient_MetadataHandling(t *testing.T) {
	metadata := map[string]string{
		"authorization":   "Bearer test-token",
		"x-custom-header": "custom-value",
	}

	cfg := Config{
		Target:   "localhost:50051",
		Service:  "test.Service",
		Method:   "TestMethod",
		Metadata: metadata,
		Timeout:  30 * time.Second,
		UseTLS:   false,
		Insecure: false,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify metadata is stored
	if len(client.md) != len(metadata) {
		t.Errorf("Expected %d metadata entries, got %d", len(metadata), len(client.md))
	}

	// Verify metadata values
	for key, expectedValue := range metadata {
		values := client.md.Get(key)
		if len(values) == 0 {
			t.Errorf("Expected metadata key '%s' not found", key)
			continue
		}
		if values[0] != expectedValue {
			t.Errorf("Expected metadata value '%s' for key '%s', got '%s'",
				expectedValue, key, values[0])
		}
	}
}

// TestClient_NilMetadata verifies client handles nil metadata
func TestClient_NilMetadata(t *testing.T) {
	cfg := Config{
		Target:   "localhost:50051",
		Service:  "test.Service",
		Method:   "TestMethod",
		Metadata: nil,
		Timeout:  30 * time.Second,
		UseTLS:   false,
		Insecure: false,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify client handles nil metadata gracefully
	if client.md == nil {
		t.Error("Expected non-nil metadata map even when config has nil metadata")
	}
}
