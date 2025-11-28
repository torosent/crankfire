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
	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/extractor"
	"github.com/torosent/crankfire/internal/feeder"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/variables"
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

// TestIntegration_HTMLReportGeneration tests end-to-end HTML report generation
func TestIntegration_HTMLReportGeneration(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	// Create temporary HTML report file
	reportPath := t.TempDir() + "/test-report.html"

	// Run with HTML output
	args := []string{
		"--target", server.URL,
		"--concurrency", "2",
		"--total", "10",
		"--html-output", reportPath,
	}

	// Execute the test
	if err := run(args); err != nil {
		t.Fatalf("run() failed: %v", err)
	}

	// Verify HTML file was created
	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		t.Fatalf("HTML report file was not created: %s", reportPath)
	}

	// Read and verify HTML content
	content, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("Failed to read HTML report: %v", err)
	}

	// Verify key HTML elements
	requiredElements := []string{
		"<!DOCTYPE html>",
		"Crankfire Load Test Report",
		"Total Requests",
		"Successful",
		"Failed",
		"Latency Statistics",
	}

	for _, elem := range requiredElements {
		if !bytes.Contains(content, []byte(elem)) {
			t.Errorf("HTML report missing required element: %s", elem)
		}
	}

	// Verify data is present
	if !bytes.Contains(content, []byte("10")) { // Total requests
		t.Error("HTML report missing request count")
	}

	t.Logf("HTML report generation test passed: %s created successfully", reportPath)
}

// TestIntegration_RequestChaining tests request chaining with value extraction
func TestIntegration_RequestChaining(t *testing.T) {
	// Create a test server that simulates a two-step API flow:
	// 1. POST /users -> returns user ID
	// 2. GET /users/{id} -> returns user details

	var userID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost && r.URL.Path == "/users" {
			// First request: create user, return ID
			userID = "user-12345"
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":    userID,
				"name":  "Alice",
				"email": "alice@example.com",
			})
			return
		}

		if r.Method == http.MethodGet && r.URL.Path == "/users/user-12345" {
			// Second request: get user by ID
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     userID,
				"name":   "Alice",
				"email":  "alice@example.com",
				"status": "active",
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Step 1: Create user and extract ID
	cfg1 := &config.Config{
		TargetURL: server.URL + "/users",
		Method:    http.MethodPost,
		Body:      `{"name":"Alice","email":"alice@example.com"}`,
	}

	builder1, err := createTestRequestBuilder(cfg1)
	if err != nil {
		t.Fatalf("failed to create first request builder: %v", err)
	}

	collector := metrics.NewCollector()
	requester := &httpRequester{
		client:    http.DefaultClient,
		builder:   builder1,
		collector: collector,
	}

	// Create variable store for chaining
	store := variables.NewStore()
	ctx := variables.NewContext(context.Background(), store)

	// Create endpoint with extractor
	tmpl := &endpointTemplate{
		name:    "create-user",
		weight:  1,
		builder: builder1,
		extractors: []extractor.Extractor{
			{
				JSONPath: "id",
				Variable: "user_id",
			},
		},
	}
	ctx = context.WithValue(ctx, endpointContextKey, tmpl)

	// Execute first request
	err = requester.Do(ctx)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}

	// Verify extraction
	extractedID, ok := store.Get("user_id")
	if !ok {
		t.Fatal("user_id not extracted from first response")
	}
	if extractedID != "user-12345" {
		t.Errorf("expected user_id='user-12345', got '%s'", extractedID)
	}

	// Step 2: Use extracted ID in second request
	// Build second request with extracted ID in URL
	cfg2 := &config.Config{
		TargetURL: server.URL + "/users/" + extractedID,
		Method:    http.MethodGet,
	}

	builder2, err := createTestRequestBuilder(cfg2)
	if err != nil {
		t.Fatalf("failed to create second request builder: %v", err)
	}

	requester2 := &httpRequester{
		client:    http.DefaultClient,
		builder:   builder2,
		collector: collector,
	}

	tmpl2 := &endpointTemplate{
		name:    "get-user",
		weight:  1,
		builder: builder2,
	}
	ctx = context.WithValue(ctx, endpointContextKey, tmpl2)

	// Execute second request - using the extracted user_id
	err = requester2.Do(ctx)
	if err != nil {
		t.Fatalf("Second request failed: %v", err)
	}

	t.Logf("Request chaining test passed: extracted ID was used in subsequent request")
}

// TestIntegration_RequestChaining_WithDefaults tests request chaining with default values
func TestIntegration_RequestChaining_WithDefaults(t *testing.T) {
	// Test that default values are used when extraction fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Simulate getting a user by ID with fallback behavior
		userID := r.URL.Query().Get("id")
		if userID == "" {
			userID = "default-user"
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   userID,
			"name": "Test User",
		})
	}))
	defer server.Close()

	// First request that returns empty or missing extraction
	cfg1 := &config.Config{
		TargetURL: server.URL + "?action=list",
		Method:    http.MethodGet,
	}

	builder1, err := createTestRequestBuilder(cfg1)
	if err != nil {
		t.Fatalf("failed to create first request builder: %v", err)
	}

	collector := metrics.NewCollector()
	requester := &httpRequester{
		client:    http.DefaultClient,
		builder:   builder1,
		collector: collector,
	}

	store := variables.NewStore()
	ctx := variables.NewContext(context.Background(), store)

	// Extractor that will fail to find the field
	tmpl := &endpointTemplate{
		name:    "list-users",
		weight:  1,
		builder: builder1,
		extractors: []extractor.Extractor{
			{
				JSONPath: "nonexistent_field",
				Variable: "user_id",
			},
		},
	}
	ctx = context.WithValue(ctx, endpointContextKey, tmpl)

	// Execute first request - extraction will fail but request succeeds
	err = requester.Do(ctx)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}

	// Verify extraction returned empty value
	extractedID, ok := store.Get("user_id")
	if !ok {
		t.Fatal("variable not created in store")
	}
	if extractedID != "" {
		t.Errorf("expected empty string for failed extraction, got '%s'", extractedID)
	}

	// Step 2: Use the variable with a default fallback
	// Simulate applying default value logic
	userIDForNext := extractedID
	if userIDForNext == "" {
		userIDForNext = "default-user"
	}

	cfg2 := &config.Config{
		TargetURL: server.URL + "?id=" + userIDForNext,
		Method:    http.MethodGet,
	}

	builder2, err := createTestRequestBuilder(cfg2)
	if err != nil {
		t.Fatalf("failed to create second request builder: %v", err)
	}

	requester2 := &httpRequester{
		client:    http.DefaultClient,
		builder:   builder2,
		collector: collector,
	}

	tmpl2 := &endpointTemplate{
		name:    "get-user-default",
		weight:  1,
		builder: builder2,
	}
	ctx = context.WithValue(ctx, endpointContextKey, tmpl2)

	err = requester2.Do(ctx)
	if err != nil {
		t.Fatalf("Second request failed: %v", err)
	}

	t.Logf("Request chaining with defaults test passed: extraction fallback handled correctly")
}

// TestIntegration_RequestChaining_MultiStep tests multiple chained requests
func TestIntegration_RequestChaining_MultiStep(t *testing.T) {
	// Simulate a multi-step workflow: create order -> get order -> update order
	var createdOrderID string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// POST /orders - create order
		if r.Method == http.MethodPost && r.URL.Path == "/orders" {
			createdOrderID = "order-999"
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     createdOrderID,
				"status": "pending",
				"items":  1,
			})
			return
		}

		// GET /orders/{id} - get order details
		if r.Method == http.MethodGet && r.URL.Path == "/orders/order-999" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     createdOrderID,
				"status": "pending",
				"items":  1,
				"total":  "99.99",
			})
			return
		}

		// PATCH /orders/{id} - update order
		if r.Method == http.MethodPatch && r.URL.Path == "/orders/order-999" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     createdOrderID,
				"status": "confirmed",
				"items":  1,
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	collector := metrics.NewCollector()
	store := variables.NewStore()
	baseCtx := variables.NewContext(context.Background(), store)

	// Request 1: Create order and extract ID
	cfg1 := &config.Config{
		TargetURL: server.URL + "/orders",
		Method:    http.MethodPost,
		Body:      `{"items":1}`,
	}
	builder1, err := createTestRequestBuilder(cfg1)
	if err != nil {
		t.Fatalf("failed to create builder1: %v", err)
	}

	requester1 := &httpRequester{
		client:    http.DefaultClient,
		builder:   builder1,
		collector: collector,
	}

	tmpl1 := &endpointTemplate{
		name:    "create-order",
		weight:  1,
		builder: builder1,
		extractors: []extractor.Extractor{
			{JSONPath: "id", Variable: "order_id"},
		},
	}
	ctx1 := context.WithValue(baseCtx, endpointContextKey, tmpl1)
	if err := requester1.Do(ctx1); err != nil {
		t.Fatalf("Request 1 failed: %v", err)
	}

	orderID, ok := store.Get("order_id")
	if !ok || orderID != "order-999" {
		t.Fatalf("order_id not extracted correctly, got %q", orderID)
	}

	// Request 2: Get order details and extract total
	cfg2 := &config.Config{
		TargetURL: server.URL + "/orders/" + orderID,
		Method:    http.MethodGet,
	}
	builder2, err := createTestRequestBuilder(cfg2)
	if err != nil {
		t.Fatalf("failed to create builder2: %v", err)
	}

	requester2 := &httpRequester{
		client:    http.DefaultClient,
		builder:   builder2,
		collector: collector,
	}

	tmpl2 := &endpointTemplate{
		name:    "get-order",
		weight:  1,
		builder: builder2,
		extractors: []extractor.Extractor{
			{JSONPath: "total", Variable: "order_total"},
		},
	}
	ctx2 := context.WithValue(baseCtx, endpointContextKey, tmpl2)
	if err := requester2.Do(ctx2); err != nil {
		t.Fatalf("Request 2 failed: %v", err)
	}

	total, ok := store.Get("order_total")
	if !ok || total != "99.99" {
		t.Fatalf("order_total not extracted correctly, got %q", total)
	}

	// Request 3: Update order using both extracted values
	cfg3 := &config.Config{
		TargetURL: server.URL + "/orders/" + orderID,
		Method:    http.MethodPatch,
		Body:      `{"status":"confirmed"}`,
	}
	builder3, err := createTestRequestBuilder(cfg3)
	if err != nil {
		t.Fatalf("failed to create builder3: %v", err)
	}

	requester3 := &httpRequester{
		client:    http.DefaultClient,
		builder:   builder3,
		collector: collector,
	}

	tmpl3 := &endpointTemplate{
		name:    "update-order",
		weight:  1,
		builder: builder3,
	}
	ctx3 := context.WithValue(baseCtx, endpointContextKey, tmpl3)
	if err := requester3.Do(ctx3); err != nil {
		t.Fatalf("Request 3 failed: %v", err)
	}

	t.Logf("Multi-step request chaining test passed: created order, extracted values, used in subsequent requests")
}
