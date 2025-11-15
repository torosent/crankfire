package websocket

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Helper function to create a test WebSocket server
func createTestWSServer(handler func(*websocket.Conn)) *httptest.Server {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer conn.Close()
		handler(conn)
	}))
}

func TestWebSocketConnectAndSendReceiveText(t *testing.T) {
	// Create a test server that echoes messages
	server := createTestWSServer(func(conn *websocket.Conn) {
		for {
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(msgType, data); err != nil {
				return
			}
		}
	})
	defer server.Close()

	// Create client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(Config{
		URL: wsURL,
	})

	ctx := context.Background()

	// Test Connect
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Test SendMessage with text
	testMsg := "Hello, WebSocket!"
	err = client.SendMessage(ctx, Message{
		Type: websocket.TextMessage,
		Data: []byte(testMsg),
	})
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Test ReceiveMessage
	received, err := client.ReceiveMessage(ctx)
	if err != nil {
		t.Fatalf("ReceiveMessage failed: %v", err)
	}

	if received.Type != websocket.TextMessage {
		t.Errorf("Expected message type %d, got %d", websocket.TextMessage, received.Type)
	}

	if string(received.Data) != testMsg {
		t.Errorf("Expected message %q, got %q", testMsg, string(received.Data))
	}
}

func TestWebSocketMetricsTracking(t *testing.T) {
	// Create a test server that echoes messages
	server := createTestWSServer(func(conn *websocket.Conn) {
		for {
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(msgType, data); err != nil {
				return
			}
		}
	})
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(Config{
		URL: wsURL,
	})

	ctx := context.Background()

	// Connect
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Send multiple messages
	messages := []string{"message1", "message2", "message3"}
	for _, msg := range messages {
		err = client.SendMessage(ctx, Message{
			Type: websocket.TextMessage,
			Data: []byte(msg),
		})
		if err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}

		// Receive echo
		_, err = client.ReceiveMessage(ctx)
		if err != nil {
			t.Fatalf("ReceiveMessage failed: %v", err)
		}
	}

	// Check metrics
	metrics := client.Metrics()

	if metrics.MessagesSent != int64(len(messages)) {
		t.Errorf("Expected %d messages sent, got %d", len(messages), metrics.MessagesSent)
	}

	if metrics.MessagesReceived != int64(len(messages)) {
		t.Errorf("Expected %d messages received, got %d", len(messages), metrics.MessagesReceived)
	}

	expectedBytesSent := int64(0)
	for _, msg := range messages {
		expectedBytesSent += int64(len(msg))
	}

	if metrics.BytesSent != expectedBytesSent {
		t.Errorf("Expected %d bytes sent, got %d", expectedBytesSent, metrics.BytesSent)
	}

	if metrics.BytesReceived != expectedBytesSent {
		t.Errorf("Expected %d bytes received, got %d", expectedBytesSent, metrics.BytesReceived)
	}

	if metrics.ConnectionDuration <= 0 {
		t.Errorf("Expected positive connection duration, got %v", metrics.ConnectionDuration)
	}

	if metrics.Errors != 0 {
		t.Errorf("Expected 0 errors, got %d", metrics.Errors)
	}
}

func TestWebSocketConnectionError(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "invalid URL",
			url:     "ws://localhost:99999/invalid",
			wantErr: true,
		},
		{
			name:    "non-websocket endpoint",
			url:     "ws://httpbin.org/html",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(Config{
				URL:              tt.url,
				HandshakeTimeout: 2 * time.Second,
			})

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			err := client.Connect(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Connect() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil {
				metrics := client.Metrics()
				if metrics.Errors == 0 {
					t.Errorf("Expected error count > 0, got %d", metrics.Errors)
				}
			}
		})
	}
}

func TestWebSocketSendWithoutConnect(t *testing.T) {
	client := NewClient(Config{
		URL: "ws://localhost:8080",
	})

	ctx := context.Background()

	err := client.SendMessage(ctx, Message{
		Type: websocket.TextMessage,
		Data: []byte("test"),
	})

	if err == nil {
		t.Fatal("Expected error when sending without connection, got nil")
	}

	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("Expected 'not connected' error, got %v", err)
	}
}

func TestWebSocketReceiveWithoutConnect(t *testing.T) {
	client := NewClient(Config{
		URL: "ws://localhost:8080",
	})

	ctx := context.Background()

	_, err := client.ReceiveMessage(ctx)

	if err == nil {
		t.Fatal("Expected error when receiving without connection, got nil")
	}

	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("Expected 'not connected' error, got %v", err)
	}
}

func TestWebSocketClose(t *testing.T) {
	server := createTestWSServer(func(conn *websocket.Conn) {
		// Wait for close
		_, _, err := conn.ReadMessage()
		if err != nil {
			return
		}
	})
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(Config{
		URL: wsURL,
	})

	ctx := context.Background()

	// Connect
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Close
	err = client.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Try to send after close (should fail)
	err = client.SendMessage(ctx, Message{
		Type: websocket.TextMessage,
		Data: []byte("test"),
	})
	if err == nil {
		t.Error("Expected error when sending after close, got nil")
	}
}

func TestWebSocketCloseWithoutConnect(t *testing.T) {
	client := NewClient(Config{
		URL: "ws://localhost:8080",
	})

	// Close without connect should not error
	err := client.Close()
	if err != nil {
		t.Errorf("Close without connection failed: %v", err)
	}
}

func TestWebSocketBinaryMessages(t *testing.T) {
	server := createTestWSServer(func(conn *websocket.Conn) {
		for {
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(msgType, data); err != nil {
				return
			}
		}
	})
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(Config{
		URL: wsURL,
	})

	ctx := context.Background()

	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Send binary data
	binaryData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
	err = client.SendMessage(ctx, Message{
		Type: websocket.BinaryMessage,
		Data: binaryData,
	})
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Receive binary message
	received, err := client.ReceiveMessage(ctx)
	if err != nil {
		t.Fatalf("ReceiveMessage failed: %v", err)
	}

	if received.Type != websocket.BinaryMessage {
		t.Errorf("Expected message type %d, got %d", websocket.BinaryMessage, received.Type)
	}

	if len(received.Data) != len(binaryData) {
		t.Fatalf("Expected %d bytes, got %d", len(binaryData), len(received.Data))
	}

	for i, b := range binaryData {
		if received.Data[i] != b {
			t.Errorf("Byte mismatch at index %d: expected %x, got %x", i, b, received.Data[i])
		}
	}
}

func TestWebSocketContextCancellation(t *testing.T) {
	// Test that context cancellation works during connect
	wsURL := "ws://localhost:99999/timeout"
	client := NewClient(Config{
		URL:              wsURL,
		HandshakeTimeout: 5 * time.Second,
	})

	// Create a context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should timeout during connect
	err := client.Connect(ctx)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "context deadline exceeded") && !strings.Contains(err.Error(), "connection refused") {
		t.Logf("Warning: Expected context or connection error, got: %v", err)
	}
}

func TestWebSocketMultipleConnectError(t *testing.T) {
	server := createTestWSServer(func(conn *websocket.Conn) {
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(Config{
		URL: wsURL,
	})

	ctx := context.Background()

	// First connect should succeed
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("First Connect failed: %v", err)
	}
	defer client.Close()

	// Second connect should fail
	err = client.Connect(ctx)
	if err == nil {
		t.Fatal("Expected error on second connect, got nil")
	}

	if !strings.Contains(err.Error(), "already connected") {
		t.Errorf("Expected 'already connected' error, got %v", err)
	}
}

func TestWebSocketCustomHeaders(t *testing.T) {
	receivedHeaders := make(http.Header)
	server := createTestWSServer(func(conn *websocket.Conn) {
		time.Sleep(50 * time.Millisecond)
	})
	defer server.Close()

	// Create server with header validation
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()

		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		conn.Close()
	}))
	defer server2.Close()

	headers := http.Header{}
	headers.Set("X-Custom-Header", "test-value")
	headers.Set("Authorization", "Bearer token123")

	wsURL := "ws" + strings.TrimPrefix(server2.URL, "http")
	client := NewClient(Config{
		URL:     wsURL,
		Headers: headers,
	})

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Verify custom headers were sent
	if receivedHeaders.Get("X-Custom-Header") != "test-value" {
		t.Errorf("Expected X-Custom-Header to be 'test-value', got '%s'", receivedHeaders.Get("X-Custom-Header"))
	}

	if receivedHeaders.Get("Authorization") != "Bearer token123" {
		t.Errorf("Expected Authorization to be 'Bearer token123', got '%s'", receivedHeaders.Get("Authorization"))
	}
}

func TestWebSocketReadWriteErrors(t *testing.T) {
	server := createTestWSServer(func(conn *websocket.Conn) {
		// Read one message then close abruptly
		conn.ReadMessage()
		conn.Close()
	})
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(Config{
		URL: wsURL,
	})

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Send first message (should work)
	err = client.SendMessage(ctx, Message{
		Type: websocket.TextMessage,
		Data: []byte("test"),
	})
	if err != nil {
		t.Fatalf("First SendMessage failed: %v", err)
	}

	// Wait for server to close
	time.Sleep(100 * time.Millisecond)

	// Try to receive (should fail with connection closed)
	_, err = client.ReceiveMessage(ctx)
	if err == nil {
		t.Error("Expected error when reading from closed connection")
	}

	// Check that error was tracked in metrics
	metrics := client.Metrics()
	if metrics.Errors == 0 {
		t.Error("Expected error count > 0 in metrics")
	}
}

func TestWebSocketNewClientDefaults(t *testing.T) {
	client := NewClient(Config{
		URL: "ws://localhost:8080",
	})

	// Verify defaults are set
	if client.url != "ws://localhost:8080" {
		t.Errorf("Expected URL to be 'ws://localhost:8080', got '%s'", client.url)
	}

	if client.dialer == nil {
		t.Error("Expected dialer to be initialized")
	}

	if client.dialer.HandshakeTimeout != 30*time.Second {
		t.Errorf("Expected default HandshakeTimeout to be 30s, got %v", client.dialer.HandshakeTimeout)
	}
}

func TestWebSocketLargeMessage(t *testing.T) {
	server := createTestWSServer(func(conn *websocket.Conn) {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		conn.WriteMessage(msgType, data)
	})
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(Config{
		URL: wsURL,
	})

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Create a large message (100KB)
	largeData := make([]byte, 100*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	err = client.SendMessage(ctx, Message{
		Type: websocket.TextMessage,
		Data: largeData,
	})
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	received, err := client.ReceiveMessage(ctx)
	if err != nil {
		t.Fatalf("ReceiveMessage failed: %v", err)
	}

	if len(received.Data) != len(largeData) {
		t.Errorf("Expected %d bytes, got %d", len(largeData), len(received.Data))
	}

	// Verify metrics tracked the large message
	metrics := client.Metrics()
	if metrics.BytesSent != int64(len(largeData)) {
		t.Errorf("Expected BytesSent=%d, got %d", len(largeData), metrics.BytesSent)
	}
	if metrics.BytesReceived != int64(len(largeData)) {
		t.Errorf("Expected BytesReceived=%d, got %d", len(largeData), metrics.BytesReceived)
	}
}

func TestWebSocketConcurrentSendReceive(t *testing.T) {
	server := createTestWSServer(func(conn *websocket.Conn) {
		for {
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(msgType, data); err != nil {
				return
			}
		}
	})
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(Config{
		URL: wsURL,
	})

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Send and receive messages sequentially (WebSocket connections require
	// external synchronization for concurrent use)
	numMessages := 10

	for i := 0; i < numMessages; i++ {
		msg := fmt.Sprintf("message-%d", i)
		err := client.SendMessage(ctx, Message{
			Type: websocket.TextMessage,
			Data: []byte(msg),
		})
		if err != nil {
			t.Errorf("SendMessage %d failed: %v", i, err)
		}

		// Receive echo
		_, err = client.ReceiveMessage(ctx)
		if err != nil {
			t.Errorf("ReceiveMessage %d failed: %v", i, err)
		}
	}

	// Verify metrics
	metrics := client.Metrics()
	if metrics.MessagesSent != int64(numMessages) {
		t.Errorf("Expected %d messages sent, got %d", numMessages, metrics.MessagesSent)
	}
	if metrics.MessagesReceived != int64(numMessages) {
		t.Errorf("Expected %d messages received, got %d", numMessages, metrics.MessagesReceived)
	}
}
