package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/metrics"
)

func TestSSERequester_Do(t *testing.T) {
	// Start a test SSE server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Auth Header
		if r.Header.Get("Authorization") != "Bearer mock-token" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		// Send events
		fmt.Fprintf(w, "data: event1\n\n")
		flusher.Flush()
		time.Sleep(10 * time.Millisecond)
		fmt.Fprintf(w, "data: event2\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	tests := []struct {
		name           string
		cfg            config.SSEConfig
		feederData     map[string]string
		wantConnectErr bool
		wantReadErr    bool
	}{
		{
			name: "successful connect and read",
			cfg: config.SSEConfig{
				MaxEvents:   2,
				ReadTimeout: 1 * time.Second,
			},
			feederData:     nil,
			wantConnectErr: false,
		},
		{
			name: "connect error invalid url",
			cfg: config.SSEConfig{
				MaxEvents: 1,
			},
			// We will override target URL in the test loop for this case
			wantConnectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup Config
			cfg := &config.Config{
				TargetURL: srv.URL,
				SSE:       tt.cfg,
			}

			if tt.name == "connect error invalid url" {
				cfg.TargetURL = "://invalid"
			}

			// Setup Dependencies
			collector := metrics.NewCollector()
			provider := &mockAuthProvider{}
			feeder := &mockFeeder{data: tt.feederData}

			// Create Requester
			sr := newSSERequester(cfg, collector, provider, feeder)

			// Execute
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := sr.Do(ctx)

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

func TestSSERequester_Placeholders(t *testing.T) {
	// Start a test SSE server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/events/123" {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "data: ok\n\n")
	}))
	defer srv.Close()

	cfg := &config.Config{
		TargetURL: srv.URL + "/events/{{id}}",
		SSE: config.SSEConfig{
			MaxEvents: 1,
		},
	}

	collector := metrics.NewCollector()
	provider := &mockAuthProvider{}
	feeder := &mockFeeder{data: map[string]string{"id": "123"}}

	sr := newSSERequester(cfg, collector, provider, feeder)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := sr.Do(ctx)
	if err != nil {
		t.Fatalf("Do() failed: %v", err)
	}
}
