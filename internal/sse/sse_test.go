package sse

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Helper function to create a test SSE server
func createTestSSEServer(handler func(w http.ResponseWriter)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		handler(w)
	}))
}

func TestSSEConnectAndReadEvents(t *testing.T) {
	server := createTestSSEServer(func(w http.ResponseWriter) {
		fmt.Fprintf(w, "event: message\n")
		fmt.Fprintf(w, "data: Hello, SSE!\n")
		fmt.Fprintf(w, "\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	client := NewClient(Config{
		URL: server.URL,
	})

	ctx := context.Background()

	// Test Connect
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Test ReadEvent
	event, err := client.ReadEvent(ctx)
	if err != nil {
		t.Fatalf("ReadEvent failed: %v", err)
	}

	if event.Event != "message" {
		t.Errorf("Expected event type 'message', got '%s'", event.Event)
	}

	if event.Data != "Hello, SSE!" {
		t.Errorf("Expected data 'Hello, SSE!', got '%s'", event.Data)
	}
}

func TestSSEMultilineData(t *testing.T) {
	server := createTestSSEServer(func(w http.ResponseWriter) {
		fmt.Fprintf(w, "event: multiline\n")
		fmt.Fprintf(w, "data: Line 1\n")
		fmt.Fprintf(w, "data: Line 2\n")
		fmt.Fprintf(w, "data: Line 3\n")
		fmt.Fprintf(w, "\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	client := NewClient(Config{
		URL: server.URL,
	})

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	event, err := client.ReadEvent(ctx)
	if err != nil {
		t.Fatalf("ReadEvent failed: %v", err)
	}

	expectedData := "Line 1\nLine 2\nLine 3"
	if event.Data != expectedData {
		t.Errorf("Expected data %q, got %q", expectedData, event.Data)
	}

	if event.Event != "multiline" {
		t.Errorf("Expected event type 'multiline', got '%s'", event.Event)
	}
}

func TestSSEMetricsTracking(t *testing.T) {
	numEvents := 5
	server := createTestSSEServer(func(w http.ResponseWriter) {
		for i := 0; i < numEvents; i++ {
			fmt.Fprintf(w, "id: %d\n", i)
			fmt.Fprintf(w, "event: test\n")
			fmt.Fprintf(w, "data: Event %d\n", i)
			fmt.Fprintf(w, "\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	client := NewClient(Config{
		URL: server.URL,
	})

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Read all events
	for i := 0; i < numEvents; i++ {
		event, err := client.ReadEvent(ctx)
		if err != nil {
			t.Fatalf("ReadEvent %d failed: %v", i, err)
		}

		expectedID := fmt.Sprintf("%d", i)
		if event.ID != expectedID {
			t.Errorf("Event %d: expected ID '%s', got '%s'", i, expectedID, event.ID)
		}

		expectedData := fmt.Sprintf("Event %d", i)
		if event.Data != expectedData {
			t.Errorf("Event %d: expected data '%s', got '%s'", i, expectedData, event.Data)
		}
	}

	// Check metrics
	metrics := client.Metrics()

	if metrics.EventsReceived != int64(numEvents) {
		t.Errorf("Expected %d events received, got %d", numEvents, metrics.EventsReceived)
	}

	if metrics.BytesReceived <= 0 {
		t.Errorf("Expected positive bytes received, got %d", metrics.BytesReceived)
	}

	if metrics.ConnectionDuration <= 0 {
		t.Errorf("Expected positive connection duration, got %v", metrics.ConnectionDuration)
	}

	if metrics.Errors != 0 {
		t.Errorf("Expected 0 errors, got %d", metrics.Errors)
	}
}

func TestSSEConnectionError(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "invalid URL",
			url:     "http://localhost:99999/invalid",
			wantErr: true,
		},
		{
			name:    "invalid host",
			url:     "http://invalid-host-that-does-not-exist-12345.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(Config{
				URL:     tt.url,
				Timeout: 2 * time.Second,
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

func TestSSENon200StatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
		{"403 Forbidden", http.StatusForbidden},
		{"401 Unauthorized", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := NewClient(Config{
				URL: server.URL,
			})

			ctx := context.Background()
			err := client.Connect(ctx)

			if err == nil {
				t.Fatal("Expected error for non-200 status code, got nil")
			}

			expectedErr := fmt.Sprintf("unexpected status code: %d", tt.statusCode)
			if !strings.Contains(err.Error(), expectedErr) {
				t.Errorf("Expected error to contain '%s', got '%v'", expectedErr, err)
			}

			metrics := client.Metrics()
			if metrics.Errors == 0 {
				t.Error("Expected error count > 0 in metrics")
			}
		})
	}
}

func TestSSEReadWithoutConnect(t *testing.T) {
	client := NewClient(Config{
		URL: "http://localhost:8080/events",
	})

	ctx := context.Background()

	_, err := client.ReadEvent(ctx)

	if err == nil {
		t.Fatal("Expected error when reading without connection, got nil")
	}

	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("Expected 'not connected' error, got %v", err)
	}
}

func TestSSEClose(t *testing.T) {
	server := createTestSSEServer(func(w http.ResponseWriter) {
		// Keep connection open
		time.Sleep(2 * time.Second)
	})
	defer server.Close()

	client := NewClient(Config{
		URL: server.URL,
	})

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Close
	err = client.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Try to read after close (should fail)
	_, err = client.ReadEvent(ctx)
	if err == nil {
		t.Error("Expected error when reading after close, got nil")
	}
}

func TestSSECloseWithoutConnect(t *testing.T) {
	client := NewClient(Config{
		URL: "http://localhost:8080/events",
	})

	// Close without connect should not error
	err := client.Close()
	if err != nil {
		t.Errorf("Close without connection failed: %v", err)
	}
}

func TestSSECommentLines(t *testing.T) {
	server := createTestSSEServer(func(w http.ResponseWriter) {
		fmt.Fprintf(w, ": This is a comment\n")
		fmt.Fprintf(w, ": Another comment\n")
		fmt.Fprintf(w, "event: test\n")
		fmt.Fprintf(w, "data: Real data\n")
		fmt.Fprintf(w, ": Comment in the middle\n")
		fmt.Fprintf(w, "\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	client := NewClient(Config{
		URL: server.URL,
	})

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	event, err := client.ReadEvent(ctx)
	if err != nil {
		t.Fatalf("ReadEvent failed: %v", err)
	}

	// Comments should be ignored
	if event.Event != "test" {
		t.Errorf("Expected event 'test', got '%s'", event.Event)
	}

	if event.Data != "Real data" {
		t.Errorf("Expected data 'Real data', got '%s'", event.Data)
	}
}

func TestSSEContextCancellation(t *testing.T) {
	server := createTestSSEServer(func(w http.ResponseWriter) {
		// Keep connection open without sending events
		time.Sleep(5 * time.Second)
	})
	defer server.Close()

	client := NewClient(Config{
		URL: server.URL,
	})

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Create a context with short timeout
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// This should timeout or return error when connection closes
	_, err = client.ReadEvent(ctxWithTimeout)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	// The error can be either context deadline exceeded or connection closed
	// depending on timing
	if err != context.DeadlineExceeded && !strings.Contains(err.Error(), "connection closed") {
		t.Errorf("Expected context.DeadlineExceeded or connection closed error, got %v", err)
	}
}

func TestSSEMultipleConnectError(t *testing.T) {
	server := createTestSSEServer(func(w http.ResponseWriter) {
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	client := NewClient(Config{
		URL: server.URL,
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

func TestSSECustomHeaders(t *testing.T) {
	receivedHeaders := make(http.Header)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "data: test\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer server.Close()

	headers := http.Header{}
	headers.Set("X-Custom-Header", "test-value")
	headers.Set("Authorization", "Bearer token123")

	client := NewClient(Config{
		URL:     server.URL,
		Headers: headers,
	})

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Verify SSE-specific headers
	if receivedHeaders.Get("Accept") != "text/event-stream" {
		t.Errorf("Expected Accept header 'text/event-stream', got '%s'", receivedHeaders.Get("Accept"))
	}

	if receivedHeaders.Get("Cache-Control") != "no-cache" {
		t.Errorf("Expected Cache-Control header 'no-cache', got '%s'", receivedHeaders.Get("Cache-Control"))
	}

	if receivedHeaders.Get("Connection") != "keep-alive" {
		t.Errorf("Expected Connection header 'keep-alive', got '%s'", receivedHeaders.Get("Connection"))
	}

	// Verify custom headers
	if receivedHeaders.Get("X-Custom-Header") != "test-value" {
		t.Errorf("Expected X-Custom-Header 'test-value', got '%s'", receivedHeaders.Get("X-Custom-Header"))
	}

	if receivedHeaders.Get("Authorization") != "Bearer token123" {
		t.Errorf("Expected Authorization 'Bearer token123', got '%s'", receivedHeaders.Get("Authorization"))
	}
}

func TestSSEEventWithID(t *testing.T) {
	server := createTestSSEServer(func(w http.ResponseWriter) {
		fmt.Fprintf(w, "id: 123\n")
		fmt.Fprintf(w, "event: update\n")
		fmt.Fprintf(w, "data: Event with ID\n")
		fmt.Fprintf(w, "\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	client := NewClient(Config{
		URL: server.URL,
	})

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	event, err := client.ReadEvent(ctx)
	if err != nil {
		t.Fatalf("ReadEvent failed: %v", err)
	}

	if event.ID != "123" {
		t.Errorf("Expected ID '123', got '%s'", event.ID)
	}

	if event.Event != "update" {
		t.Errorf("Expected event 'update', got '%s'", event.Event)
	}

	if event.Data != "Event with ID" {
		t.Errorf("Expected data 'Event with ID', got '%s'", event.Data)
	}
}

func TestSSEMultipleEvents(t *testing.T) {
	server := createTestSSEServer(func(w http.ResponseWriter) {
		events := []struct {
			id    string
			event string
			data  string
		}{
			{"1", "start", "First event"},
			{"2", "middle", "Second event"},
			{"3", "end", "Third event"},
		}

		for _, e := range events {
			fmt.Fprintf(w, "id: %s\n", e.id)
			fmt.Fprintf(w, "event: %s\n", e.event)
			fmt.Fprintf(w, "data: %s\n", e.data)
			fmt.Fprintf(w, "\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	client := NewClient(Config{
		URL: server.URL,
	})

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	expectedEvents := []struct {
		id    string
		event string
		data  string
	}{
		{"1", "start", "First event"},
		{"2", "middle", "Second event"},
		{"3", "end", "Third event"},
	}

	for i, expected := range expectedEvents {
		event, err := client.ReadEvent(ctx)
		if err != nil {
			t.Fatalf("ReadEvent %d failed: %v", i, err)
		}

		if event.ID != expected.id {
			t.Errorf("Event %d: expected ID '%s', got '%s'", i, expected.id, event.ID)
		}

		if event.Event != expected.event {
			t.Errorf("Event %d: expected event '%s', got '%s'", i, expected.event, event.Event)
		}

		if event.Data != expected.data {
			t.Errorf("Event %d: expected data '%s', got '%s'", i, expected.data, event.Data)
		}
	}
}

func TestSSEEmptyDataLines(t *testing.T) {
	server := createTestSSEServer(func(w http.ResponseWriter) {
		fmt.Fprintf(w, "event: test\n")
		fmt.Fprintf(w, "data:\n")
		fmt.Fprintf(w, "data:\n")
		fmt.Fprintf(w, "\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	client := NewClient(Config{
		URL: server.URL,
	})

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	event, err := client.ReadEvent(ctx)
	if err != nil {
		t.Fatalf("ReadEvent failed: %v", err)
	}

	// Empty data lines should result in a newline
	expectedData := "\n"
	if event.Data != expectedData {
		t.Errorf("Expected data %q, got %q", expectedData, event.Data)
	}
}

func TestSSEDataWithLeadingSpace(t *testing.T) {
	server := createTestSSEServer(func(w http.ResponseWriter) {
		fmt.Fprintf(w, "data: Data with space\n")
		fmt.Fprintf(w, "data:No space\n")
		fmt.Fprintf(w, "\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	client := NewClient(Config{
		URL: server.URL,
	})

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	event, err := client.ReadEvent(ctx)
	if err != nil {
		t.Fatalf("ReadEvent failed: %v", err)
	}

	// Leading space after colon should be stripped
	expectedData := "Data with space\nNo space"
	if event.Data != expectedData {
		t.Errorf("Expected data %q, got %q", expectedData, event.Data)
	}
}

func TestSSENewClientDefaults(t *testing.T) {
	client := NewClient(Config{
		URL: "http://localhost:8080/events",
	})

	if client.url != "http://localhost:8080/events" {
		t.Errorf("Expected URL to be 'http://localhost:8080/events', got '%s'", client.url)
	}

	if client.httpClient == nil {
		t.Error("Expected httpClient to be initialized")
	}

	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("Expected default Timeout to be 30s, got %v", client.httpClient.Timeout)
	}
}

func TestSSEConnectionClosed(t *testing.T) {
	server := createTestSSEServer(func(w http.ResponseWriter) {
		fmt.Fprintf(w, "data: First event\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Connection will close after this
	})
	defer server.Close()

	client := NewClient(Config{
		URL: server.URL,
	})

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Read first event (should succeed)
	event, err := client.ReadEvent(ctx)
	if err != nil {
		t.Fatalf("First ReadEvent failed: %v", err)
	}

	if event.Data != "First event" {
		t.Errorf("Expected data 'First event', got '%s'", event.Data)
	}

	// Try to read second event (should fail with EOF/connection closed)
	_, err = client.ReadEvent(ctx)
	if err == nil {
		t.Error("Expected error when reading after connection closed")
	}

	// Error should be tracked in metrics
	metrics := client.Metrics()
	if metrics.Errors == 0 {
		t.Error("Expected error count > 0 in metrics")
	}
}

func TestSSEMalformedLines(t *testing.T) {
	server := createTestSSEServer(func(w http.ResponseWriter) {
		fmt.Fprintf(w, "malformed line without colon\n")
		fmt.Fprintf(w, "event: test\n")
		fmt.Fprintf(w, "data: Valid data\n")
		fmt.Fprintf(w, "\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	client := NewClient(Config{
		URL: server.URL,
	})

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Malformed lines should be skipped
	event, err := client.ReadEvent(ctx)
	if err != nil {
		t.Fatalf("ReadEvent failed: %v", err)
	}

	if event.Event != "test" {
		t.Errorf("Expected event 'test', got '%s'", event.Event)
	}

	if event.Data != "Valid data" {
		t.Errorf("Expected data 'Valid data', got '%s'", event.Data)
	}
}

func TestSSELargeEvent(t *testing.T) {
	server := createTestSSEServer(func(w http.ResponseWriter) {
		fmt.Fprintf(w, "event: large\n")
		// Create a large data payload
		largeData := strings.Repeat("A", 10000)
		fmt.Fprintf(w, "data: %s\n", largeData)
		fmt.Fprintf(w, "\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	client := NewClient(Config{
		URL: server.URL,
	})

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	event, err := client.ReadEvent(ctx)
	if err != nil {
		t.Fatalf("ReadEvent failed: %v", err)
	}

	expectedData := strings.Repeat("A", 10000)
	if event.Data != expectedData {
		t.Errorf("Expected data length %d, got %d", len(expectedData), len(event.Data))
	}

	// Verify metrics tracked the large event
	metrics := client.Metrics()
	if metrics.BytesReceived <= 10000 {
		t.Errorf("Expected BytesReceived > 10000, got %d", metrics.BytesReceived)
	}
}

func TestSSEOnlyEventField(t *testing.T) {
	server := createTestSSEServer(func(w http.ResponseWriter) {
		fmt.Fprintf(w, "event: notification\n")
		fmt.Fprintf(w, "\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	client := NewClient(Config{
		URL: server.URL,
	})

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	event, err := client.ReadEvent(ctx)
	if err != nil {
		t.Fatalf("ReadEvent failed: %v", err)
	}

	if event.Event != "notification" {
		t.Errorf("Expected event 'notification', got '%s'", event.Event)
	}

	if event.Data != "" {
		t.Errorf("Expected empty data, got '%s'", event.Data)
	}
}
