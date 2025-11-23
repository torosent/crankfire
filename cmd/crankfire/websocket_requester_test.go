package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/metrics"
)

// Mock Auth Provider
type mockAuthProvider struct{}

func (m *mockAuthProvider) Apply(ctx context.Context, req *http.Request) error {
	req.Header.Set("Authorization", "Bearer mock-token")
	return nil
}

func (m *mockAuthProvider) GetToken(ctx context.Context) (string, error) {
	return "mock-token", nil
}

func (m *mockAuthProvider) Token(ctx context.Context) (string, error) {
	return "mock-token", nil
}

func (m *mockAuthProvider) InjectHeader(ctx context.Context, req *http.Request) error {
	req.Header.Set("Authorization", "Bearer mock-token")
	return nil
}

func (m *mockAuthProvider) Close() error {
	return nil
}

// Mock Feeder
type mockFeeder struct {
	data map[string]string
}

func (m *mockFeeder) Next(ctx context.Context) (map[string]string, error) {
	return m.data, nil
}

func (m *mockFeeder) Close() error {
	return nil
}

func (m *mockFeeder) Len() int {
	return 1
}

func TestWebsocketRequester_Do(t *testing.T) {
	// Start a test websocket server
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Auth Header
		if r.Header.Get("Authorization") != "Bearer mock-token" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()

		for {
			mt, message, err := c.ReadMessage()
			if err != nil {
				break
			}
			// Echo back if it's "ping"
			if string(message) == "ping" {
				if err := c.WriteMessage(mt, []byte("pong")); err != nil {
					break
				}
			}
		}
	}))
	defer srv.Close()

	// Convert http:// to ws://
	wsURL := "ws" + srv.URL[4:]

	tests := []struct {
		name           string
		cfg            config.WebSocketConfig
		feederData     map[string]string
		wantConnectErr bool
		wantSendErr    bool
		wantRecvErr    bool
	}{
		{
			name: "successful connect and send",
			cfg: config.WebSocketConfig{
				Messages: []string{"hello"},
			},
			feederData:     nil,
			wantConnectErr: false,
		},
		{
			name: "successful ping pong",
			cfg: config.WebSocketConfig{
				Messages:       []string{"ping"},
				ReceiveTimeout: 100 * time.Millisecond,
			},
			feederData:     nil,
			wantConnectErr: false,
		},
		{
			name: "connect error invalid url",
			cfg: config.WebSocketConfig{
				Messages: []string{"hello"},
			},
			// We will override target URL in the test loop for this case
			wantConnectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup Config
			cfg := &config.Config{
				TargetURL: wsURL,
				WebSocket: tt.cfg,
			}

			if tt.name == "connect error invalid url" {
				cfg.TargetURL = "://invalid"
			}

			// Setup Dependencies
			collector := metrics.NewCollector()
			provider := &mockAuthProvider{}
			feeder := &mockFeeder{data: tt.feederData}

			// Create Requester
			wr := newWebSocketRequester(cfg, collector, provider, feeder)

			// Execute
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := wr.Do(ctx)

			if tt.wantConnectErr {
				if err == nil {
					t.Error("Do() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Do() unexpected error: %v", err)
			}
		})
	}
}

func TestWebsocketRequester_Placeholders(t *testing.T) {
	// Start a test websocket server
	upgrader := websocket.Upgrader{}
	receivedMsg := make(chan string, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()

		_, message, err := c.ReadMessage()
		if err == nil {
			receivedMsg <- string(message)
		}
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:]

	cfg := &config.Config{
		TargetURL: wsURL,
		WebSocket: config.WebSocketConfig{
			Messages: []string{"Hello {{name}}"},
		},
	}

	collector := metrics.NewCollector()
	provider := &mockAuthProvider{}
	feeder := &mockFeeder{data: map[string]string{"name": "World"}}

	wr := newWebSocketRequester(cfg, collector, provider, feeder)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := wr.Do(ctx)
	if err != nil {
		t.Fatalf("Do() failed: %v", err)
	}

	select {
	case msg := <-receivedMsg:
		if msg != "Hello World" {
			t.Errorf("Expected 'Hello World', got '%s'", msg)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for message")
	}
}
