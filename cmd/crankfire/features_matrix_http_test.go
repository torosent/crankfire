//go:build integration

package main

import (
	"fmt"
	"testing"
)

// TestFeaturesMatrix_HTTP_Methods validates all HTTP methods work correctly with load testing
func TestFeaturesMatrix_HTTP_Methods(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			// Start test server with echo endpoint
			server := startTestServer(t)

			// Create config for load test
			var body string
			if method == "POST" || method == "PUT" || method == "PATCH" {
				body = `{"test":"data"}`
			}

			cfg := generateTestConfig(
				server.httpServer.URL+"/echo",
				WithMethod(method),
				WithBody(body),
				WithConcurrency(5),
				WithDuration(getTestDuration()),
			)

			// Run load test
			stats, _ := runLoadTest(t, cfg, cfg.TargetURL)

			// Validate results
			validateTestResults(t, stats, resultExpectations{
				minRequests: 10,
				maxFailures: -1,
				errorRate:   0.5,
			})

			// Log metrics
			t.Logf("%s: Completed %d requests, %.2f RPS, P50: %v", method, stats.Total, stats.RequestsPerSec, stats.P50Latency)
		})
	}
}

// TestFeaturesMatrix_HTTP_Headers validates custom headers are sent correctly with load testing
func TestFeaturesMatrix_HTTP_Headers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	testHeaders := map[string]string{
		"X-Custom-Header":    "custom-value",
		"X-Request-ID":       "12345",
		"Authorization":      "Bearer test-token",
		"Content-Type":       "application/json",
		"Accept":             "application/json",
		"X-Multi-Word":       "value with spaces",
		"X-Case-Sensitivity": "CamelCaseValue",
	}

	// Start test server with echo endpoint
	server := startTestServer(t)

	// Create config with headers for load test
	cfg := generateTestConfig(
		server.httpServer.URL+"/echo",
		WithMethod("GET"),
		WithHeaders(testHeaders),
		WithConcurrency(5),
		WithDuration(getTestDuration()),
	)

	// Run load test
	stats, _ := runLoadTest(t, cfg, cfg.TargetURL)

	// Validate results
	validateTestResults(t, stats, resultExpectations{
		minRequests: 10,
		maxFailures: -1,
		errorRate:   0.5,
	})

	// Log metrics
	t.Logf("HTTP headers: Completed %d requests, %.2f RPS, P50: %v", stats.Total, stats.RequestsPerSec, stats.P50Latency)
}

// TestFeaturesMatrix_HTTP_Body validates request bodies are sent correctly with load testing
func TestFeaturesMatrix_HTTP_Body(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	testCases := []struct {
		name        string
		contentType string
		body        string
	}{
		{
			name:        "JSON body",
			contentType: "application/json",
			body:        `{"key":"value","number":42,"nested":{"inner":"data"}}`,
		},
		{
			name:        "Form data",
			contentType: "application/x-www-form-urlencoded",
			body:        "field1=value1&field2=value2&field3=complex%20value",
		},
		{
			name:        "Plain text",
			contentType: "text/plain",
			body:        "This is a plain text body with some content",
		},
		{
			name:        "XML body",
			contentType: "application/xml",
			body:        `<?xml version="1.0"?><root><element>value</element></root>`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Start test server with echo endpoint
			server := startTestServer(t)

			// Create config with body type and headers for load test
			contentTypeHeader := map[string]string{
				"Content-Type": tc.contentType,
			}

			cfg := generateTestConfig(
				server.httpServer.URL+"/echo",
				WithMethod("POST"),
				WithBody(tc.body),
				WithHeaders(contentTypeHeader),
				WithConcurrency(5),
				WithDuration(getTestDuration()),
			)

			// Run load test
			stats, _ := runLoadTest(t, cfg, cfg.TargetURL)

			// Validate results
			validateTestResults(t, stats, resultExpectations{
				minRequests: 10,
				maxFailures: -1,
				errorRate:   0.5,
			})

			// Log metrics
			t.Logf("%s: Completed %d requests, %.2f RPS, P50: %v", tc.name, stats.Total, stats.RequestsPerSec, stats.P50Latency)
		})
	}
}

// TestFeaturesMatrix_HTTP_MultipleEndpoints validates concurrent endpoint testing with load testing
func TestFeaturesMatrix_HTTP_MultipleEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	endpoints := []struct {
		path   string
		method string
	}{
		{"/api/users", "GET"},
		{"/api/products", "GET"},
		{"/api/orders", "POST"},
		{"/health", "GET"},
		{"/metrics", "GET"},
	}

	for _, ep := range endpoints {
		t.Run(fmt.Sprintf("%s_%s", ep.method, ep.path), func(t *testing.T) {
			// Start test server with echo endpoint for this test
			server := startTestServer(t)

			// Create config for this endpoint with load test
			cfg := generateTestConfig(
				server.httpServer.URL+"/echo",
				WithMethod(ep.method),
				WithConcurrency(5),
				WithDuration(getTestDuration()),
			)

			// Run load test
			stats, _ := runLoadTest(t, cfg, cfg.TargetURL)

			// Validate results
			validateTestResults(t, stats, resultExpectations{
				minRequests: 10,
				maxFailures: -1,
				errorRate:   0.5,
			})

			// Log metrics
			t.Logf("%s %s: Completed %d requests, %.2f RPS, P50: %v", ep.method, ep.path, stats.Total, stats.RequestsPerSec, stats.P50Latency)
		})
	}
}
