package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/metrics"
)

func TestWebSocketRequester(t *testing.T) {
	// Create a test WebSocket server
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Failed to upgrade connection: %v", err)
		}
		defer conn.Close()

		// Echo back any messages received
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			if err := conn.WriteMessage(msgType, msg); err != nil {
				break
			}
		}
	}))
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Create config
	cfg := &config.Config{
		TargetURL: wsURL,
		Headers:   make(map[string]string),
		WebSocket: config.WebSocketConfig{
			Messages:         []string{`{"test": "message"}`},
			MessageInterval:  100 * time.Millisecond,
			ReceiveTimeout:   0, // Don't wait for receives
			HandshakeTimeout: 5 * time.Second,
		},
	}

	collector := metrics.NewCollector()
	requester := newWebSocketRequester(cfg, collector, nil, nil, nil)

	ctx := context.Background()
	err := requester.Do(ctx)
	if err != nil {
		t.Fatalf("WebSocket requester failed: %v", err)
	}

	// Verify metrics were collected
	stats := collector.Stats(1 * time.Second)
	if stats.Total != 1 {
		t.Errorf("Expected 1 request, got %d", stats.Total)
	}
	if stats.Successes != 1 {
		t.Errorf("Expected 1 success, got %d", stats.Successes)
	}
}

func TestSSERequester(t *testing.T) {
	// Create a test SSE server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// Send a few events
		for i := 0; i < 5; i++ {
			_, _ = w.Write([]byte("data: test event\n\n"))
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	// Create config
	cfg := &config.Config{
		TargetURL: server.URL,
		Headers:   make(map[string]string),
		SSE: config.SSEConfig{
			ReadTimeout: 2 * time.Second,
			MaxEvents:   5,
		},
	}

	collector := metrics.NewCollector()
	requester := newSSERequester(cfg, collector, nil, nil, nil)

	ctx := context.Background()
	err := requester.Do(ctx)
	if err != nil {
		t.Fatalf("SSE requester failed: %v", err)
	}

	// Verify metrics were collected
	stats := collector.Stats(1 * time.Second)
	if stats.Total != 1 {
		t.Errorf("Expected 1 request, got %d", stats.Total)
	}
	if stats.Successes != 1 {
		t.Errorf("Expected 1 success, got %d", stats.Successes)
	}
}

func TestProtocolSelection(t *testing.T) {
	tests := []struct {
		name     string
		protocol config.Protocol
	}{
		{"HTTP", config.ProtocolHTTP},
		{"WebSocket", config.ProtocolWebSocket},
		{"SSE", config.ProtocolSSE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Protocol:  tt.protocol,
				TargetURL: "http://example.com",
				Headers:   make(map[string]string),
			}

			switch tt.protocol {
			case config.ProtocolWebSocket:
				cfg.WebSocket = config.WebSocketConfig{
					Messages:         []string{"test"},
					HandshakeTimeout: 5 * time.Second,
				}
			case config.ProtocolSSE:
				cfg.SSE = config.SSEConfig{
					ReadTimeout: 5 * time.Second,
					MaxEvents:   10,
				}
			}

			// Just verify we can create the requester without panicking
			collector := metrics.NewCollector()
			switch tt.protocol {
			case config.ProtocolWebSocket:
				_ = newWebSocketRequester(cfg, collector, nil, nil, nil)
			case config.ProtocolSSE:
				_ = newSSERequester(cfg, collector, nil, nil, nil)
			default:
				// HTTP protocol - skip as it requires more setup
			}
		})
	}
}
