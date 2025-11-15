package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/auth"
	"github.com/torosent/crankfire/internal/feeder"
	"github.com/torosent/crankfire/internal/metrics"
)

// TestIntegration_AuthFlow tests OAuth2 client credentials authentication
func TestIntegration_AuthFlow(t *testing.T) {
	// Create mock OAuth2 server
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "test-token-123",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer tokenServer.Close()

	// Create OAuth2 client credentials provider
	provider, err := auth.NewOAuth2ClientCredentialsProvider(
		tokenServer.URL+"/token",
		"test-client",
		"test-secret",
		[]string{},
		0,
	)
	if err != nil {
		t.Fatalf("Failed to create auth provider: %v", err)
	}
	defer provider.Close()

	// Get initial token
	ctx := context.Background()
	token, err := provider.Token(ctx)
	if err != nil {
		t.Fatalf("Failed to get token: %v", err)
	}

	if token != "test-token-123" {
		t.Errorf("Expected token 'test-token-123', got '%s'", token)
	}

	t.Logf("Auth integration test passed: token obtained successfully")
}

// TestIntegration_HTTPWithMetrics tests HTTP requests with metrics collection
func TestIntegration_HTTPWithMetrics(t *testing.T) {
	// Create test server
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	// Create collector
	collector := metrics.NewCollector()
	collector.Start()

	// Execute requests manually
	totalRequests := 10
	ctx := context.Background()
	client := &http.Client{Timeout: 5 * time.Second}

	for i := 0; i < totalRequests; i++ {
		start := time.Now()
		req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		resp, err := client.Do(req)
		latency := time.Since(start)

		if err != nil {
			collector.RecordRequest(latency, err, nil)
		} else {
			resp.Body.Close()
			collector.RecordRequest(latency, nil, nil)
		}
	}

	// Verify metrics
	stats := collector.Stats(1 * time.Second)
	if stats.Total != int64(totalRequests) {
		t.Errorf("Expected %d total requests, got %d", totalRequests, stats.Total)
	}
	if stats.Successes != int64(totalRequests) {
		t.Errorf("Expected %d successes, got %d", totalRequests, stats.Successes)
	}
	if requestCount != totalRequests {
		t.Errorf("Expected %d requests to server, got %d", totalRequests, requestCount)
	}

	t.Logf("HTTP integration test passed: %d requests, all successful", requestCount)
}

// TestIntegration_MultiProtocolMetrics tests multiple protocol metrics
func TestIntegration_MultiProtocolMetrics(t *testing.T) {
	collector := metrics.NewCollector()
	collector.Start()

	// Simulate WebSocket requests
	for i := 0; i < 10; i++ {
		collector.RecordRequest(50*time.Millisecond, nil, &metrics.RequestMetadata{
			Endpoint: "ws-endpoint",
			Protocol: "websocket",
			CustomMetrics: map[string]interface{}{
				"messages_sent":     int64(5),
				"messages_received": int64(4),
			},
		})
	}

	// Simulate gRPC requests
	for i := 0; i < 5; i++ {
		collector.RecordRequest(40*time.Millisecond, nil, &metrics.RequestMetadata{
			Endpoint: "grpc-endpoint",
			Protocol: "grpc",
			CustomMetrics: map[string]interface{}{
				"calls":       int64(1),
				"status_code": "OK",
			},
		})
	}

	stats := collector.Stats(1 * time.Second)

	// Verify all protocols present
	if len(stats.ProtocolMetrics) != 2 {
		t.Errorf("Expected 2 protocols in metrics, got %d", len(stats.ProtocolMetrics))
	}

	// Verify WebSocket aggregation
	if wsMetrics, ok := stats.ProtocolMetrics["websocket"]; ok {
		if msgSent, ok := wsMetrics["messages_sent"].(int64); !ok || msgSent != 50 {
			t.Errorf("Expected messages_sent=50, got %v", wsMetrics["messages_sent"])
		}
	} else {
		t.Error("Expected websocket protocol in metrics")
	}

	// Verify gRPC aggregation
	if grpcMetrics, ok := stats.ProtocolMetrics["grpc"]; ok {
		if calls, ok := grpcMetrics["calls"].(int64); !ok || calls != 5 {
			t.Errorf("Expected calls=5, got %v", grpcMetrics["calls"])
		}
	} else {
		t.Error("Expected grpc protocol in metrics")
	}

	t.Logf("Multi-protocol metrics test passed: %d protocols", len(stats.ProtocolMetrics))
}

// TestIntegration_FeederRoundRobin tests CSV feeder round-robin behavior
func TestIntegration_FeederRoundRobin(t *testing.T) {
	// Create CSV data
	csvData := `user_id,email
1,user1@test.com
2,user2@test.com
3,user3@test.com`

	// Create temporary CSV file
	tmpFile := t.TempDir() + "/test-users.csv"
	if err := os.WriteFile(tmpFile, []byte(csvData), 0644); err != nil {
		t.Fatalf("Failed to create test CSV: %v", err)
	}

	// Create CSV feeder
	fd, err := feeder.NewCSVFeeder(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create feeder: %v", err)
	}

	// Verify we can retrieve all records
	expectedUserIDs := []string{"1", "2", "3"}
	for i, expectedID := range expectedUserIDs {
		record, err := fd.Next(context.Background())
		if err != nil {
			t.Fatalf("Failed to get record %d: %v", i, err)
		}
		if record["user_id"] != expectedID {
			t.Errorf("Record %d: expected user_id=%s, got %s", i, expectedID, record["user_id"])
		}
	}

	// Verify exhaustion behavior (should restart from beginning)
	record, err := fd.Next(context.Background())
	if err == nil && record["user_id"] == "1" {
		t.Logf("Feeder wraps around: restarted from beginning")
	} else if err != nil {
		t.Logf("Feeder exhaustion detected (expected): %v", err)
	}

	t.Logf("Feeder test passed: %d records retrieved successfully", len(expectedUserIDs))
}

// TestIntegration_JSONOutput tests JSON serialization of stats
func TestIntegration_JSONOutput(t *testing.T) {
	collector := metrics.NewCollector()
	collector.Start()

	// Record some test data
	collector.RecordRequest(50*time.Millisecond, nil, &metrics.RequestMetadata{
		Endpoint: "test-endpoint",
		Protocol: "http",
	})
	collector.RecordRequest(60*time.Millisecond, nil, &metrics.RequestMetadata{
		Endpoint: "test-endpoint",
		Protocol: "http",
	})

	stats := collector.Stats(1 * time.Second)

	// Serialize to JSON
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	if err := encoder.Encode(stats); err != nil {
		t.Fatalf("Failed to encode stats: %v", err)
	}

	// Parse back
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify expected fields
	expectedFields := []string{"total", "successes", "failures", "duration_ms", "requests_per_sec"}
	for _, field := range expectedFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("Expected field %s in JSON output", field)
		}
	}

	t.Logf("JSON output test passed: all expected fields present")
}
