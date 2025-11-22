package output_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/output"
	"github.com/torosent/crankfire/internal/threshold"
)

func TestGenerateHTMLReport(t *testing.T) {
	stats := metrics.Stats{
		EndpointStats: metrics.EndpointStats{
			Total:          100,
			Successes:      95,
			Failures:       5,
			MinLatency:     10 * time.Millisecond,
			MaxLatency:     100 * time.Millisecond,
			MeanLatency:    50 * time.Millisecond,
			P50Latency:     45 * time.Millisecond,
			P90Latency:     80 * time.Millisecond,
			P95Latency:     90 * time.Millisecond,
			P99Latency:     95 * time.Millisecond,
			RequestsPerSec: 50.0,
		},
		Duration: 2 * time.Second,
		Endpoints: map[string]metrics.EndpointStats{
			"users": {
				Total:          60,
				Successes:      58,
				Failures:       2,
				P99Latency:     85 * time.Millisecond,
				RequestsPerSec: 30.0,
			},
			"orders": {
				Total:          40,
				Successes:      37,
				Failures:       3,
				P99Latency:     90 * time.Millisecond,
				RequestsPerSec: 20.0,
			},
		},
	}

	history := []metrics.DataPoint{
		{
			Timestamp:          time.Now(),
			TotalRequests:      50,
			SuccessfulRequests: 48,
			Errors:             2,
			CurrentRPS:         50.0,
			P50Latency:         45 * time.Millisecond,
			P95Latency:         85 * time.Millisecond,
			P99Latency:         90 * time.Millisecond,
			P50LatencyMs:       45.0,
			P95LatencyMs:       85.0,
			P99LatencyMs:       90.0,
		},
		{
			Timestamp:          time.Now().Add(1 * time.Second),
			TotalRequests:      100,
			SuccessfulRequests: 95,
			Errors:             5,
			CurrentRPS:         50.0,
			P50Latency:         45 * time.Millisecond,
			P95Latency:         90 * time.Millisecond,
			P99Latency:         95 * time.Millisecond,
			P50LatencyMs:       45.0,
			P95LatencyMs:       90.0,
			P99LatencyMs:       95.0,
		},
	}

	thresholdResults := []threshold.Result{
		{
			Threshold: threshold.Threshold{
				Raw:       "http_req_duration:p95 < 100",
				Metric:    "http_req_duration",
				Aggregate: "p95",
				Operator:  "<",
				Value:     100,
			},
			Actual: 90.0,
			Pass:   true,
		},
		{
			Threshold: threshold.Threshold{
				Raw:       "http_req_failed:rate < 0.1",
				Metric:    "http_req_failed",
				Aggregate: "rate",
				Operator:  "<",
				Value:     0.1,
			},
			Actual: 0.05,
			Pass:   true,
		},
	}

	var buf bytes.Buffer
	err := output.GenerateHTMLReport(&buf, stats, history, thresholdResults, output.ReportMetadata{})
	if err != nil {
		t.Fatalf("GenerateHTMLReport() error = %v", err)
	}

	html := buf.String()

	// Verify HTML structure
	requiredElements := []string{
		"<!DOCTYPE html>",
		"<html",
		"<head>",
		"<body>",
		"Crankfire Load Test Report",
		"Total Requests",
		"Successful",
		"Failed",
		"Requests/sec",
	}

	for _, elem := range requiredElements {
		if !strings.Contains(html, elem) {
			t.Errorf("HTML missing required element: %s", elem)
		}
	}

	// Verify data is embedded
	if !strings.Contains(html, "100") { // Total requests
		t.Errorf("HTML missing total requests count")
	}
	if !strings.Contains(html, "95") { // Successes
		t.Errorf("HTML missing success count")
	}
	if !strings.Contains(html, "5") { // Failures
		t.Errorf("HTML missing failure count")
	}

	// Verify chart scripts are present
	if !strings.Contains(html, "uPlot") {
		t.Errorf("HTML missing uPlot chart library")
	}
	if !strings.Contains(html, "rps-chart") {
		t.Errorf("HTML missing RPS chart container")
	}
	if !strings.Contains(html, "latency-chart") {
		t.Errorf("HTML missing latency chart container")
	}

	// Verify thresholds section
	if !strings.Contains(html, "Thresholds") {
		t.Errorf("HTML missing thresholds section")
	}
	if !strings.Contains(html, "http_req_duration:p95 &lt; 100") {
		t.Errorf("HTML missing threshold definition")
	}

	// Verify endpoint breakdown
	if !strings.Contains(html, "Endpoint Breakdown") {
		t.Errorf("HTML missing endpoint breakdown section")
	}
	if !strings.Contains(html, "users") {
		t.Errorf("HTML missing users endpoint")
	}
	if !strings.Contains(html, "orders") {
		t.Errorf("HTML missing orders endpoint")
	}
}

func TestGenerateHTMLReport_NoHistory(t *testing.T) {
	stats := metrics.Stats{
		EndpointStats: metrics.EndpointStats{
			Total:          50,
			Successes:      45,
			Failures:       5,
			RequestsPerSec: 25.0,
		},
		Duration: 2 * time.Second,
	}

	var buf bytes.Buffer
	err := output.GenerateHTMLReport(&buf, stats, nil, nil, output.ReportMetadata{})
	if err != nil {
		t.Fatalf("GenerateHTMLReport() error = %v", err)
	}

	html := buf.String()

	// Should still have basic structure
	if !strings.Contains(html, "Crankfire Load Test Report") {
		t.Errorf("HTML missing title")
	}

	// Should NOT have chart sections when no history
	if strings.Contains(html, "Performance Over Time") {
		t.Errorf("HTML should not have charts section without history")
	}
}

func TestGenerateHTMLReport_NoThresholds(t *testing.T) {
	stats := metrics.Stats{
		EndpointStats: metrics.EndpointStats{
			Total:          50,
			Successes:      50,
			Failures:       0,
			RequestsPerSec: 25.0,
		},
		Duration: 2 * time.Second,
	}

	history := []metrics.DataPoint{
		{
			Timestamp:          time.Now(),
			TotalRequests:      50,
			SuccessfulRequests: 50,
			Errors:             0,
			CurrentRPS:         25.0,
		},
	}

	var buf bytes.Buffer
	err := output.GenerateHTMLReport(&buf, stats, history, nil, output.ReportMetadata{})
	if err != nil {
		t.Fatalf("GenerateHTMLReport() error = %v", err)
	}

	html := buf.String()

	// Should still have basic structure and charts
	if !strings.Contains(html, "Crankfire Load Test Report") {
		t.Errorf("HTML missing title")
	}
	if !strings.Contains(html, "Performance Over Time") {
		t.Errorf("HTML missing charts section")
	}

	// Should NOT have thresholds section
	if strings.Contains(html, "Thresholds (") {
		t.Errorf("HTML should not have thresholds section when none provided")
	}
}

func TestGenerateHTMLReport_NoEndpoints(t *testing.T) {
	stats := metrics.Stats{
		EndpointStats: metrics.EndpointStats{
			Total:          50,
			Successes:      50,
			Failures:       0,
			RequestsPerSec: 25.0,
		},
		Duration: 2 * time.Second,
	}

	var buf bytes.Buffer
	err := output.GenerateHTMLReport(&buf, stats, nil, nil, output.ReportMetadata{})
	if err != nil {
		t.Fatalf("GenerateHTMLReport() error = %v", err)
	}

	html := buf.String()

	// Should NOT have endpoint breakdown section
	if strings.Contains(html, "Endpoint Breakdown") {
		t.Errorf("HTML should not have endpoint breakdown when no endpoints")
	}
}

func TestGenerateHTMLReport_EscapesHTMLInData(t *testing.T) {
	stats := metrics.Stats{
		EndpointStats: metrics.EndpointStats{
			Total:          10,
			Successes:      10,
			Failures:       0,
			RequestsPerSec: 5.0,
		},
		Duration: 2 * time.Second,
		Endpoints: map[string]metrics.EndpointStats{
			"<script>alert('xss')</script>": {
				Total:     10,
				Successes: 10,
				Failures:  0,
			},
		},
	}

	var buf bytes.Buffer
	err := output.GenerateHTMLReport(&buf, stats, nil, nil, output.ReportMetadata{})
	if err != nil {
		t.Fatalf("GenerateHTMLReport() error = %v", err)
	}

	html := buf.String()

	// Script tags should be escaped
	if strings.Contains(html, "<script>alert('xss')</script>") {
		t.Errorf("HTML did not escape dangerous content")
	}
	// Should contain escaped version
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Errorf("HTML did not properly escape content")
	}
}

func TestGenerateHTMLReport_WithMetadata(t *testing.T) {
	stats := metrics.Stats{
		EndpointStats: metrics.EndpointStats{
			Total:          10,
			Successes:      10,
			Failures:       0,
			RequestsPerSec: 5.0,
		},
		Duration: 2 * time.Second,
	}

	metadata := output.ReportMetadata{
		TargetURL: "https://api.example.com",
		TestedEndpoints: []output.TestedEndpoint{
			{Name: "Login", Method: "POST", URL: "/login"},
			{Name: "GetUsers", Method: "GET", URL: "/users"},
		},
	}

	var buf bytes.Buffer
	err := output.GenerateHTMLReport(&buf, stats, nil, nil, metadata)
	if err != nil {
		t.Fatalf("GenerateHTMLReport() error = %v", err)
	}

	html := buf.String()

	if !strings.Contains(html, "https://api.example.com") {
		t.Errorf("HTML missing target URL")
	}
	if !strings.Contains(html, "Tested Endpoints Configuration") {
		t.Errorf("HTML missing endpoints configuration section")
	}
	if !strings.Contains(html, "Login") || !strings.Contains(html, "/login") {
		t.Errorf("HTML missing Login endpoint details")
	}
	if !strings.Contains(html, "GetUsers") || !strings.Contains(html, "/users") {
		t.Errorf("HTML missing GetUsers endpoint details")
	}
}
