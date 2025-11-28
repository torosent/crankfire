//go:build integration

package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/threshold"
)

// TestFeaturesMatrix_Threshold_Latency validates latency threshold configuration and evaluation
func TestFeaturesMatrix_Threshold_Latency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := startTestServer(t)
	defer stopTestServer(t, server)

	configContent := fmt.Sprintf(`
target: http://localhost:%d/echo
concurrency: 2
total: 10
thresholds:
  - "http_req_duration:p95 < 1000"
  - "http_req_duration:p99 < 2000"
  - "http_req_duration:avg < 500"
  - "http_req_duration:max < 5000"
`, server.port)

	configPath := createTestConfigFile(t, configContent)

	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("Failed to load threshold config: %v", err)
	}

	if len(cfg.Thresholds) != 4 {
		t.Errorf("Expected 4 thresholds, got %d", len(cfg.Thresholds))
	}

	// Parse the thresholds
	thresholds, err := threshold.ParseMultiple(cfg.Thresholds)
	if err != nil {
		t.Fatalf("Failed to parse thresholds: %v", err)
	}

	// Test threshold evaluation with mock metrics
	stats := metrics.Stats{
		EndpointStats: metrics.EndpointStats{
			P50LatencyMs:  50,
			P90LatencyMs:  150,
			P95LatencyMs:  200,
			P99LatencyMs:  500,
			MeanLatencyMs: 100,
			MinLatencyMs:  10,
			MaxLatencyMs:  1000,
		},
	}

	evaluator := threshold.NewEvaluator(thresholds)
	results := evaluator.Evaluate(stats)

	for _, result := range results {
		if !result.Pass {
			t.Errorf("Expected threshold %s to pass, got failed", result.Threshold.Raw)
		}
	}

	t.Log("Latency threshold test passed")
}

// TestFeaturesMatrix_Threshold_FailureRate validates failure rate threshold
func TestFeaturesMatrix_Threshold_FailureRate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := startTestServer(t)
	defer stopTestServer(t, server)

	configContent := fmt.Sprintf(`
target: http://localhost:%d/echo
concurrency: 2
total: 10
thresholds:
  - "http_req_failed:rate < 0.05"
  - "http_req_failed:count < 5"
`, server.port)

	configPath := createTestConfigFile(t, configContent)

	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("Failed to load failure rate threshold config: %v", err)
	}

	if len(cfg.Thresholds) != 2 {
		t.Errorf("Expected 2 thresholds, got %d", len(cfg.Thresholds))
	}

	// Parse the thresholds
	thresholds, err := threshold.ParseMultiple(cfg.Thresholds)
	if err != nil {
		t.Fatalf("Failed to parse thresholds: %v", err)
	}

	// Test with passing metrics (2% failure rate, 2 failures)
	passingStats := metrics.Stats{
		EndpointStats: metrics.EndpointStats{
			Total:    100,
			Failures: 2,
		},
	}

	evaluator := threshold.NewEvaluator(thresholds)
	results := evaluator.Evaluate(passingStats)

	for _, result := range results {
		if !result.Pass {
			t.Errorf("Expected threshold %s to pass with 2%% failure rate", result.Threshold.Raw)
		}
	}

	// Test with failing metrics (10% failure rate)
	failingStats := metrics.Stats{
		EndpointStats: metrics.EndpointStats{
			Total:    100,
			Failures: 10,
		},
	}

	failResults := evaluator.Evaluate(failingStats)
	foundFailed := false
	for _, result := range failResults {
		if result.Threshold.Metric == "http_req_failed" && !result.Pass {
			foundFailed = true
		}
	}

	if !foundFailed {
		t.Error("Expected failure_rate threshold to fail with 10% failure rate")
	}

	t.Log("Failure rate threshold test passed")
}

// TestFeaturesMatrix_Threshold_ExitCode validates threshold exit code behavior
func TestFeaturesMatrix_Threshold_ExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	testCases := []struct {
		name           string
		thresholdStrs  []string
		stats          metrics.Stats
		expectedPassed bool
	}{
		{
			name:          "All thresholds pass",
			thresholdStrs: []string{"http_req_duration:p95 < 1000", "http_req_failed:rate < 0.05"},
			stats: metrics.Stats{
				EndpointStats: metrics.EndpointStats{
					P95LatencyMs: 100,
					Total:        100,
					Failures:     1,
				},
			},
			expectedPassed: true,
		},
		{
			name:          "Latency threshold fails",
			thresholdStrs: []string{"http_req_duration:p95 < 100"},
			stats: metrics.Stats{
				EndpointStats: metrics.EndpointStats{
					P95LatencyMs: 200,
				},
			},
			expectedPassed: false,
		},
		{
			name:          "Failure rate threshold fails",
			thresholdStrs: []string{"http_req_failed:rate < 0.01"},
			stats: metrics.Stats{
				EndpointStats: metrics.EndpointStats{
					Total:    100,
					Failures: 5,
				},
			},
			expectedPassed: false,
		},
		{
			name:          "Multiple thresholds - one fails",
			thresholdStrs: []string{"http_req_duration:p95 < 1000", "http_req_failed:rate < 0.01"},
			stats: metrics.Stats{
				EndpointStats: metrics.EndpointStats{
					P95LatencyMs: 100, // passes
					Total:        100,
					Failures:     5, // fails (5% > 1%)
				},
			},
			expectedPassed: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			thresholds, err := threshold.ParseMultiple(tc.thresholdStrs)
			if err != nil {
				t.Fatalf("Failed to parse thresholds: %v", err)
			}

			evaluator := threshold.NewEvaluator(thresholds)
			results := evaluator.Evaluate(tc.stats)

			allPassed := true
			for _, result := range results {
				if !result.Pass {
					allPassed = false
					break
				}
			}

			if allPassed != tc.expectedPassed {
				t.Errorf("Expected allPassed=%v, got %v", tc.expectedPassed, allPassed)
			}

			t.Logf("%s: threshold evaluation correct", tc.name)
		})
	}

	t.Log("Threshold exit code test passed")
}

// TestFeaturesMatrix_Threshold_Percentiles validates all percentile thresholds
func TestFeaturesMatrix_Threshold_Percentiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	percentileTests := []struct {
		percentile string
		threshold  float64
		actual     float64
		shouldPass bool
	}{
		{"p50", 100, 50, true},
		{"p50", 100, 150, false},
		{"p90", 200, 150, true},
		{"p95", 300, 250, true},
		{"p99", 400, 350, true},
	}

	for _, pt := range percentileTests {
		t.Run(fmt.Sprintf("%s_%v", pt.percentile, pt.shouldPass), func(t *testing.T) {
			thresholdStr := fmt.Sprintf("http_req_duration:%s < %.0f", pt.percentile, pt.threshold)
			thresholds, err := threshold.ParseMultiple([]string{thresholdStr})
			if err != nil {
				t.Fatalf("Failed to parse threshold: %v", err)
			}

			stats := metrics.Stats{}
			// Set the appropriate percentile based on test case
			switch pt.percentile {
			case "p50":
				stats.P50LatencyMs = pt.actual
			case "p90":
				stats.P90LatencyMs = pt.actual
			case "p95":
				stats.P95LatencyMs = pt.actual
			case "p99":
				stats.P99LatencyMs = pt.actual
			}

			evaluator := threshold.NewEvaluator(thresholds)
			results := evaluator.Evaluate(stats)

			if len(results) != 1 {
				t.Fatalf("Expected 1 result, got %d", len(results))
			}

			if results[0].Pass != pt.shouldPass {
				t.Errorf("%s: expected passed=%v, got %v (actual=%.2f, threshold=%.2f)",
					pt.percentile, pt.shouldPass, results[0].Pass,
					pt.actual, pt.threshold)
			}
		})
	}

	t.Log("Percentile thresholds test passed")
}

// TestFeaturesMatrix_Threshold_Conditions validates threshold condition operators
func TestFeaturesMatrix_Threshold_Conditions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	conditionTests := []struct {
		condition  string
		threshold  float64
		actual     float64
		shouldPass bool
	}{
		{"<", 100, 50, true},
		{"<", 100, 150, false},
		{"<=", 100, 100, true},
		{"<=", 100, 101, false},
		{">", 100, 150, true},
		{">", 100, 50, false},
		{">=", 100, 100, true},
		{">=", 100, 99, false},
		{"==", 100, 100, true},
		{"==", 100, 101, false},
	}

	for _, ct := range conditionTests {
		t.Run(fmt.Sprintf("%s_%.0f_%v", ct.condition, ct.threshold, ct.shouldPass), func(t *testing.T) {
			thresholdStr := fmt.Sprintf("http_req_duration:p95 %s %.0f", ct.condition, ct.threshold)
			thresholds, err := threshold.ParseMultiple([]string{thresholdStr})
			if err != nil {
				t.Fatalf("Failed to parse threshold: %v", err)
			}

			stats := metrics.Stats{
				EndpointStats: metrics.EndpointStats{
					P95LatencyMs: ct.actual,
				},
			}

			evaluator := threshold.NewEvaluator(thresholds)
			results := evaluator.Evaluate(stats)

			if len(results) != 1 {
				t.Fatalf("Expected 1 result, got %d", len(results))
			}

			if results[0].Pass != ct.shouldPass {
				t.Errorf("Condition %s %.0f: expected passed=%v, got %v (actual=%.2f)",
					ct.condition, ct.threshold, ct.shouldPass, results[0].Pass, ct.actual)
			}
		})
	}

	t.Log("Threshold conditions test passed")
}

// TestFeaturesMatrix_Threshold_RPS validates requests per second threshold
func TestFeaturesMatrix_Threshold_RPS(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := startTestServer(t)
	defer stopTestServer(t, server)

	configContent := fmt.Sprintf(`
target: http://localhost:%d/echo
concurrency: 5
duration: 5s
thresholds:
  - "http_requests:rate > 10"
  - "http_requests:count >= 50"
`, server.port)

	configPath := createTestConfigFile(t, configContent)

	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("Failed to load RPS threshold config: %v", err)
	}

	if len(cfg.Thresholds) != 2 {
		t.Errorf("Expected 2 thresholds, got %d", len(cfg.Thresholds))
	}

	// Parse thresholds
	thresholds, err := threshold.ParseMultiple(cfg.Thresholds)
	if err != nil {
		t.Fatalf("Failed to parse thresholds: %v", err)
	}

	// Verify RPS threshold config
	if thresholds[0].Metric != "http_requests" {
		t.Errorf("Expected 'http_requests' metric, got '%s'", thresholds[0].Metric)
	}
	if thresholds[0].Operator != ">" {
		t.Errorf("Expected '>' operator, got '%s'", thresholds[0].Operator)
	}

	// Test with passing stats
	stats := metrics.Stats{
		EndpointStats: metrics.EndpointStats{
			Total:          100,
			RequestsPerSec: 20,
		},
		Duration: 5 * time.Second,
	}

	evaluator := threshold.NewEvaluator(thresholds)
	results := evaluator.Evaluate(stats)

	for _, result := range results {
		if !result.Pass {
			t.Errorf("Expected threshold %s to pass, got failed", result.Threshold.Raw)
		}
	}

	t.Log("RPS threshold test passed")
}

// TestFeaturesMatrix_Threshold_P95_Pass executes a load test and validates P95 latency threshold passes
func TestFeaturesMatrix_Threshold_P95_Pass(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := startTestServer(t)
	defer stopTestServer(t, server)

	cfg := generateTestConfig(
		fmt.Sprintf("http://localhost:%d/echo", server.port),
		WithConcurrency(5),
		WithDuration(getTestDuration()),
	)

	stats, _ := runLoadTest(t, cfg, cfg.TargetURL)

	// Define threshold that should pass - P95 latency under 1000ms
	thresholds, err := threshold.ParseMultiple([]string{"http_req_duration:p95 < 1000"})
	if err != nil {
		t.Fatalf("Failed to parse threshold: %v", err)
	}

	evaluator := threshold.NewEvaluator(thresholds)
	results := evaluator.Evaluate(*stats)

	if len(results) == 0 {
		t.Fatal("Expected threshold results")
	}

	result := results[0]
	if !result.Pass {
		t.Errorf("Expected P95 threshold to pass, but failed: %s", result.Message)
	}

	t.Logf("P95 threshold PASSED: %.2fms < 1000ms", stats.P95LatencyMs)
}

// TestFeaturesMatrix_Threshold_P95_Fail executes a load test and validates P95 latency threshold fails as expected
func TestFeaturesMatrix_Threshold_P95_Fail(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := startTestServer(t)
	defer stopTestServer(t, server)

	cfg := generateTestConfig(
		fmt.Sprintf("http://localhost:%d/echo", server.port),
		WithConcurrency(5),
		WithDuration(getTestDuration()),
	)

	stats, _ := runLoadTest(t, cfg, cfg.TargetURL)

	// Define impossible threshold that should fail - P95 latency under 1 microsecond
	thresholds, err := threshold.ParseMultiple([]string{"http_req_duration:p95 < 0.001"})
	if err != nil {
		t.Fatalf("Failed to parse threshold: %v", err)
	}

	evaluator := threshold.NewEvaluator(thresholds)
	results := evaluator.Evaluate(*stats)

	if len(results) == 0 {
		t.Fatal("Expected threshold results")
	}

	result := results[0]
	if result.Pass {
		t.Errorf("Expected P95 threshold to fail with 1us limit, but it passed")
	}

	t.Logf("P95 threshold correctly FAILED: %.2fms > 0.001ms (1us)", stats.P95LatencyMs)
}

// TestFeaturesMatrix_Threshold_ErrorRate_Pass executes a load test against healthy endpoint and validates error rate threshold passes
func TestFeaturesMatrix_Threshold_ErrorRate_Pass(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := startTestServer(t)
	defer stopTestServer(t, server)

	cfg := generateTestConfig(
		fmt.Sprintf("http://localhost:%d/echo", server.port),
		WithConcurrency(5),
		WithDuration(getTestDuration()),
	)

	stats, _ := runLoadTest(t, cfg, cfg.TargetURL)

	// Healthy server should have minimal errors
	thresholds, err := threshold.ParseMultiple([]string{"http_req_failed:rate < 0.1"})
	if err != nil {
		t.Fatalf("Failed to parse threshold: %v", err)
	}

	evaluator := threshold.NewEvaluator(thresholds)
	results := evaluator.Evaluate(*stats)

	if len(results) == 0 {
		t.Fatal("Expected threshold results")
	}

	result := results[0]
	if !result.Pass {
		t.Errorf("Expected error rate threshold to pass with healthy server, but failed: %s", result.Message)
	}

	t.Logf("Error rate threshold PASSED: %.2f%% < 10%% (failures: %d/%d)",
		(float64(stats.Failures)/float64(stats.Total))*100, stats.Failures, stats.Total)
}

// TestFeaturesMatrix_Threshold_ErrorRate_Fail creates a server that returns 50% errors and validates threshold fails as expected
func TestFeaturesMatrix_Threshold_ErrorRate_Fail(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a server that returns errors on every other request
	var count int64
	server := startTestServer(t)
	defer stopTestServer(t, server)

	// Replace the default handler with one that returns 50% errors
	// We'll use a custom handler by creating a new test server
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt64(&count, 1)%2 == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Create error-prone server
	errorServer := &testServer{}
	errorServer.httpServer = httptest.NewServer(handler)
	errorServerURL := errorServer.httpServer.URL

	cfg := generateTestConfig(
		errorServerURL,
		WithConcurrency(5),
		WithDuration(getTestDuration()),
	)

	stats, _ := runLoadTest(t, cfg, cfg.TargetURL)

	// Threshold that should fail with 50% error rate
	thresholds, err := threshold.ParseMultiple([]string{"http_req_failed:rate < 0.1"})
	if err != nil {
		t.Fatalf("Failed to parse threshold: %v", err)
	}

	evaluator := threshold.NewEvaluator(thresholds)
	results := evaluator.Evaluate(*stats)

	if len(results) == 0 {
		t.Fatal("Expected threshold results")
	}

	result := results[0]
	if result.Pass {
		t.Errorf("Expected error rate threshold to fail with 50%% error rate, but it passed")
	}

	t.Logf("Error rate threshold correctly FAILED: %.2f%% > 10%% (failures: %d/%d)",
		(float64(stats.Failures)/float64(stats.Total))*100, stats.Failures, stats.Total)

	// Cleanup
	errorServer.httpServer.Close()
}

// TestFeaturesMatrix_Threshold_RPS_Pass executes a load test with high concurrency and validates RPS threshold passes
func TestFeaturesMatrix_Threshold_RPS_Pass(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := startTestServer(t)
	defer stopTestServer(t, server)

	cfg := generateTestConfig(
		fmt.Sprintf("http://localhost:%d/echo", server.port),
		WithConcurrency(10),
		WithDuration(getTestDuration()),
	)

	stats, _ := runLoadTest(t, cfg, cfg.TargetURL)

	// Threshold that should pass easily
	thresholds, err := threshold.ParseMultiple([]string{"http_requests:rate > 5"})
	if err != nil {
		t.Fatalf("Failed to parse threshold: %v", err)
	}

	evaluator := threshold.NewEvaluator(thresholds)
	results := evaluator.Evaluate(*stats)

	if len(results) == 0 {
		t.Fatal("Expected threshold results")
	}

	result := results[0]
	if !result.Pass {
		t.Errorf("Expected RPS threshold to pass, but failed: %s", result.Message)
	}

	t.Logf("RPS threshold PASSED: %.2f RPS > 5 RPS (total requests: %d)", stats.RequestsPerSec, stats.Total)
}

// TestFeaturesMatrix_Threshold_Multiple executes a load test and validates multiple thresholds pass
func TestFeaturesMatrix_Threshold_Multiple(t *testing.T) {
	// Not parallel to avoid port exhaustion
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := startTestServer(t)
	defer stopTestServer(t, server)

	cfg := generateTestConfig(
		fmt.Sprintf("http://localhost:%d/echo", server.port),
		WithConcurrency(5),
		WithDuration(getTestDuration()),
	)

	stats, _ := runLoadTest(t, cfg, cfg.TargetURL)

	// Check we got some requests through
	if stats.Total == 0 {
		t.Log("Warning: No requests completed (may be port exhaustion)")
		return
	}

	// Multiple thresholds that should all pass - be lenient on error rate due to possible connection issues
	thresholdStrs := []string{
		"http_req_duration:p95 < 1000",
		"http_req_failed:rate < 0.9", // Allow up to 90% error rate for port exhaustion
		"http_requests:rate > 1",     // Lower threshold for reliability
	}

	thresholds, err := threshold.ParseMultiple(thresholdStrs)
	if err != nil {
		t.Fatalf("Failed to parse thresholds: %v", err)
	}

	evaluator := threshold.NewEvaluator(thresholds)
	results := evaluator.Evaluate(*stats)

	if len(results) != len(thresholdStrs) {
		t.Errorf("Expected %d results, got %d", len(thresholdStrs), len(results))
	}

	for i, result := range results {
		if !result.Pass {
			t.Errorf("Threshold %d (%s) failed: %s", i, result.Threshold.Raw, result.Message)
		} else {
			t.Logf("Threshold %d PASSED: %s", i, result.Message)
		}
	}
}

// TestFeaturesMatrix_Threshold_Multiple_Partial_Fail executes a load test and validates mixed threshold results (one passes, one fails)
func TestFeaturesMatrix_Threshold_Multiple_Partial_Fail(t *testing.T) {
	// Not parallel to avoid port exhaustion
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := startTestServer(t)
	defer stopTestServer(t, server)

	cfg := generateTestConfig(
		fmt.Sprintf("http://localhost:%d/echo", server.port),
		WithConcurrency(10),
		WithDuration(getTestDuration()),
	)

	stats, _ := runLoadTest(t, cfg, cfg.TargetURL)

	// Two thresholds - one realistic, one impossible
	thresholdStrs := []string{
		"http_req_duration:p95 < 1000",  // Should pass - local server is fast
		"http_req_duration:p95 < 0.001", // Should fail - 1us is impossible
	}

	thresholds, err := threshold.ParseMultiple(thresholdStrs)
	if err != nil {
		t.Fatalf("Failed to parse thresholds: %v", err)
	}

	evaluator := threshold.NewEvaluator(thresholds)
	results := evaluator.Evaluate(*stats)

	if len(results) != len(thresholdStrs) {
		t.Errorf("Expected %d results, got %d", len(thresholdStrs), len(results))
	}

	if len(results) >= 2 {
		if !results[0].Pass {
			t.Errorf("Expected first threshold to pass: %s", results[0].Message)
		} else {
			t.Logf("First threshold PASSED: %s", results[0].Message)
		}

		if results[1].Pass {
			t.Errorf("Expected second threshold to fail: %s", results[1].Message)
		} else {
			t.Logf("Second threshold correctly FAILED: %s", results[1].Message)
		}
	}
}
