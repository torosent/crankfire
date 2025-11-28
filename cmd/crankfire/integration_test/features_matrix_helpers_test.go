//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/feeder"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/runner"
)

// getTestDuration reads the TEST_DURATION environment variable
// and returns the parsed duration, or 5 seconds if not set.
func getTestDuration() time.Duration {
	envDuration := os.Getenv("TEST_DURATION")
	if envDuration == "" {
		return 5 * time.Second
	}
	duration, err := time.ParseDuration(envDuration)
	if err != nil {
		return 5 * time.Second
	}
	return duration
}

// testRequester implements runner.Requester interface for load testing
type testRequester struct {
	client    *http.Client
	url       string
	method    string
	headers   http.Header
	body      string
	collector *metrics.Collector
}

// Do executes an HTTP request and records metrics
func (tr *testRequester) Do(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, tr.method, tr.url, strings.NewReader(tr.body))
	if err != nil {
		return err
	}

	// Apply headers
	for key, values := range tr.headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	start := time.Now()
	resp, err := tr.client.Do(req)
	latency := time.Since(start)

	// Record latency and error to collector
	if err != nil {
		tr.collector.RecordRequest(latency, err, &metrics.RequestMetadata{
			Protocol: "http",
		})
		return err
	}
	defer resp.Body.Close()

	// Consume response body
	io.ReadAll(resp.Body)

	// Record as success or failure based on status code
	var recordErr error
	if resp.StatusCode >= 400 {
		recordErr = http.ErrBodyReadAfterClose
	}

	tr.collector.RecordRequest(latency, recordErr, &metrics.RequestMetadata{
		Protocol:   "http",
		StatusCode: resp.Status,
	})

	if resp.StatusCode >= 400 {
		return recordErr
	}

	return nil
}

// feederTestRequester implements runner.Requester interface with feeder support for load testing
type feederTestRequester struct {
	client    *http.Client
	baseURL   string
	method    string
	headers   http.Header
	body      string
	collector *metrics.Collector
	feeder    feeder.Feeder
}

// Do executes an HTTP request with feeder data substitution and records metrics
func (ftr *feederTestRequester) Do(ctx context.Context) error {
	// Get next record from feeder
	record, err := ftr.feeder.Next(ctx)
	if err != nil {
		ftr.collector.RecordRequest(0, err, &metrics.RequestMetadata{
			Protocol: "http",
		})
		return err
	}

	// Substitute placeholders in URL and body
	url := substitutePlaceholders(ftr.baseURL, record)
	body := substitutePlaceholders(ftr.body, record)

	req, err := http.NewRequestWithContext(ctx, ftr.method, url, strings.NewReader(body))
	if err != nil {
		return err
	}

	// Apply headers
	for key, values := range ftr.headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	start := time.Now()
	resp, err := ftr.client.Do(req)
	latency := time.Since(start)

	// Record latency and error to collector
	if err != nil {
		ftr.collector.RecordRequest(latency, err, &metrics.RequestMetadata{
			Protocol: "http",
		})
		return err
	}
	defer resp.Body.Close()

	// Consume response body
	io.ReadAll(resp.Body)

	// Record as success or failure based on status code
	var recordErr error
	if resp.StatusCode >= 400 {
		recordErr = http.ErrBodyReadAfterClose
	}

	ftr.collector.RecordRequest(latency, recordErr, &metrics.RequestMetadata{
		Protocol:   "http",
		StatusCode: resp.Status,
	})

	if resp.StatusCode >= 400 {
		return recordErr
	}

	return nil
}

// substitutePlaceholders replaces {{field}} patterns with values from the record
func substitutePlaceholders(template string, record map[string]string) string {
	result := template
	for key, value := range record {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// convertLoadPatterns converts config.LoadPattern to runner.LoadPattern
func convertLoadPatterns(patterns []config.LoadPattern) []runner.LoadPattern {
	if len(patterns) == 0 {
		return nil
	}
	result := make([]runner.LoadPattern, len(patterns))
	for i, p := range patterns {
		result[i] = runner.LoadPattern{
			Name:     p.Name,
			Type:     runner.LoadPatternType(p.Type),
			FromRPS:  p.FromRPS,
			ToRPS:    p.ToRPS,
			Duration: p.Duration,
			Steps:    convertLoadSteps(p.Steps),
			RPS:      p.RPS,
		}
	}
	return result
}

// convertLoadSteps converts config.LoadStep to runner.LoadStep
func convertLoadSteps(steps []config.LoadStep) []runner.LoadStep {
	if len(steps) == 0 {
		return nil
	}
	result := make([]runner.LoadStep, len(steps))
	for i, s := range steps {
		result[i] = runner.LoadStep{
			RPS:      s.RPS,
			Duration: s.Duration,
		}
	}
	return result
}

// convertArrivalModel converts config.ArrivalModel to runner.ArrivalModel
func convertArrivalModel(model config.ArrivalModel) runner.ArrivalModel {
	switch model {
	case config.ArrivalModelPoisson:
		return runner.ArrivalModelPoisson
	default:
		return runner.ArrivalModelUniform
	}
}

// runLoadTest executes a load test using runner.Runner and collects metrics
func runLoadTest(t *testing.T, cfg *config.Config, serverURL string) (*metrics.Stats, runner.Result) {
	// Create collector and start it
	collector := metrics.NewCollector()
	collector.Start()

	// Create test requester
	transport := &http.Transport{
		DisableKeepAlives:   false, // Keep alive is good for performance, but let's keep it
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
	}

	headers := make(http.Header)
	for key, value := range cfg.Headers {
		headers.Add(key, value)
	}

	requester := &testRequester{
		client:    client,
		url:       serverURL,
		method:    cfg.Method,
		headers:   headers,
		body:      cfg.Body,
		collector: collector,
	}

	// Build runner options
	opts := runner.Options{
		Concurrency:   cfg.Concurrency,
		TotalRequests: cfg.Total,
		Duration:      cfg.Duration,
		RatePerSecond: cfg.Rate,
		Requester:     requester,
		LoadPatterns:  convertLoadPatterns(cfg.LoadPatterns),
		ArrivalModel:  convertArrivalModel(cfg.Arrival.Model),
	}

	// Run the load test
	result := runner.New(opts).Run(context.Background())

	// Get statistics
	stats := collector.Stats(result.Duration)

	return &stats, result
}

// runLoadTestWithFeeder executes a load test with feeder data substitution
func runLoadTestWithFeeder(t *testing.T, cfg *config.Config, serverURL string, f feeder.Feeder) (*metrics.Stats, runner.Result) {
	// Create collector and start it
	collector := metrics.NewCollector()
	collector.Start()

	// Create feeder-aware test requester
	transport := &http.Transport{
		DisableKeepAlives:   false,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
	}

	headers := make(http.Header)
	for key, value := range cfg.Headers {
		headers.Add(key, value)
	}

	requester := &feederTestRequester{
		client:    client,
		baseURL:   serverURL,
		method:    cfg.Method,
		headers:   headers,
		body:      cfg.Body,
		collector: collector,
		feeder:    f,
	}

	// Build runner options
	opts := runner.Options{
		Concurrency:   cfg.Concurrency,
		TotalRequests: cfg.Total,
		Duration:      cfg.Duration,
		RatePerSecond: cfg.Rate,
		Requester:     requester,
		LoadPatterns:  convertLoadPatterns(cfg.LoadPatterns),
		ArrivalModel:  convertArrivalModel(cfg.Arrival.Model),
	}

	// Run the load test
	result := runner.New(opts).Run(context.Background())

	// Get statistics
	stats := collector.Stats(result.Duration)

	return &stats, result
}

// testServer represents a running test server instance
type testServer struct {
	port       int
	httpServer *httptest.Server
	running    bool
}

var (
	testServerMu sync.Mutex
	testServers  = make(map[*testing.T]*testServer)
)

// startTestServer starts an embedded HTTP test server for this test
func startTestServer(t *testing.T) *testServer {
	testServerMu.Lock()
	defer testServerMu.Unlock()

	// Create embedded HTTP server
	mux := http.NewServeMux()

	// /echo endpoint - echoes request details
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		body := ""
		if r.Body != nil {
			bodyBytes, _ := io.ReadAll(r.Body)
			body = string(bodyBytes)
		}

		respData := map[string]interface{}{
			"method":  r.Method,
			"path":    r.URL.Path,
			"headers": r.Header,
			"body":    body,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(respData)
	})

	// /timestamp endpoint - returns current server timestamp
	mux.HandleFunc("/timestamp", func(w http.ResponseWriter, r *http.Request) {
		respData := map[string]interface{}{
			"timestamp": time.Now().UnixNano(),
			"iso":       time.Now().UTC().Format(time.RFC3339Nano),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(respData)
	})

	// Default endpoint for general requests
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		respData := map[string]interface{}{
			"ok":   true,
			"path": r.URL.Path,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(respData)
	})

	// Start server on random available port
	httpServer := httptest.NewServer(mux)

	// Give the server a moment to fully initialize
	time.Sleep(10 * time.Millisecond)

	// Extract port from listener address
	listener := httpServer.Listener
	addr := listener.Addr()
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		httpServer.Close()
		t.Fatalf("Failed to get TCP address from server")
	}

	ts := &testServer{
		port:       tcpAddr.Port,
		httpServer: httpServer,
		running:    true,
	}

	// Store the server so we can clean up later
	testServers[t] = ts

	// Ensure cleanup happens
	t.Cleanup(func() {
		stopTestServer(t, ts)
	})

	return ts
}

// stopTestServer stops the test server
func stopTestServer(t *testing.T, ts *testServer) {
	testServerMu.Lock()
	defer testServerMu.Unlock()

	if ts == nil {
		return
	}

	if !ts.running {
		return
	}

	if ts.httpServer != nil {
		ts.httpServer.Close()
	}

	ts.running = false
	delete(testServers, t)

	// Allow TCP sockets to fully close (TIME_WAIT state on macOS)
	// Needed for port reuse between sequential tests
	time.Sleep(300 * time.Millisecond)
	runtime.GC()
}

// generateTestConfig creates a test configuration programmatically
func generateTestConfig(targetURL string, options ...configOption) *config.Config {
	cfg := &config.Config{
		TargetURL:   targetURL,
		Method:      "GET",
		Concurrency: 1,
		Total:       0,
		Timeout:     30 * time.Second,
		Headers:     make(map[string]string),
		Duration:    30 * time.Second,
		Arrival: config.ArrivalConfig{
			Model: config.ArrivalModelUniform,
		},
	}

	for _, opt := range options {
		opt(cfg)
	}

	return cfg
}

type configOption func(*config.Config)

// WithMethod sets the HTTP method
func WithMethod(method string) configOption {
	return func(cfg *config.Config) {
		cfg.Method = method
	}
}

// WithHeaders sets HTTP headers
func WithHeaders(headers map[string]string) configOption {
	return func(cfg *config.Config) {
		cfg.Headers = headers
	}
}

// WithBody sets the request body
func WithBody(body string) configOption {
	return func(cfg *config.Config) {
		cfg.Body = body
	}
}

// WithConcurrency sets concurrency level
func WithConcurrency(concurrency int) configOption {
	return func(cfg *config.Config) {
		cfg.Concurrency = concurrency
	}
}

// WithTotal sets total requests
func WithTotal(total int) configOption {
	return func(cfg *config.Config) {
		cfg.Total = total
	}
}

// WithDuration sets test duration
func WithDuration(duration time.Duration) configOption {
	return func(cfg *config.Config) {
		cfg.Duration = duration
	}
}

// WithLoadPatterns sets load patterns
func WithLoadPatterns(patterns []config.LoadPattern) configOption {
	return func(cfg *config.Config) {
		cfg.LoadPatterns = patterns
	}
}

// WithArrivalModel sets the arrival model
func WithArrivalModel(model config.ArrivalModel) configOption {
	return func(cfg *config.Config) {
		cfg.Arrival.Model = model
	}
}

// WithRate sets the rate (RPS)
func WithRate(rps int) configOption {
	return func(cfg *config.Config) {
		cfg.Rate = rps
	}
}

// validateTestResults validates load test results.
// NOTE: This function is for Phase 2+ when actual load tests are implemented.
func validateTestResults(t *testing.T, stats *metrics.Stats, expectations resultExpectations) {
	if expectations.minSuccesses > 0 && stats.Successes < int64(expectations.minSuccesses) {
		t.Errorf("Expected at least %d successes, got %d", expectations.minSuccesses, stats.Successes)
	}

	if expectations.maxFailures >= 0 && stats.Failures > int64(expectations.maxFailures) {
		t.Errorf("Expected at most %d failures, got %d", expectations.maxFailures, stats.Failures)
	}

	if expectations.minRPS > 0 && stats.RequestsPerSec < expectations.minRPS {
		t.Errorf("Expected at least %.2f RPS, got %.2f", expectations.minRPS, stats.RequestsPerSec)
	}

	if expectations.minRequests > 0 && stats.Total < int64(expectations.minRequests) {
		t.Errorf("Expected at least %d total requests, got %d", expectations.minRequests, stats.Total)
	}

	if expectations.maxP95Latency > 0 && stats.P95Latency > expectations.maxP95Latency {
		t.Errorf("Expected P95 latency at most %v, got %v", expectations.maxP95Latency, stats.P95Latency)
	}

	if expectations.errorRate >= 0 && expectations.errorRate <= 1.0 {
		total := stats.Successes + stats.Failures
		if total > 0 {
			actualErrorRate := float64(stats.Failures) / float64(total)
			if actualErrorRate > expectations.errorRate {
				t.Errorf("Expected error rate at most %.2f%%, got %.2f%%", expectations.errorRate*100, actualErrorRate*100)
			}
		}
	}
}

// resultExpectations defines expectations for validating load test results
type resultExpectations struct {
	minSuccesses  int
	maxFailures   int
	minRPS        float64
	minRequests   int           // minimum total requests
	maxP95Latency time.Duration // max acceptable P95 latency
	errorRate     float64       // max acceptable error rate (0.0 to 1.0)
}

// createTestConfigFile creates a temporary config file for testing
func createTestConfigFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yml")

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	return configPath
}

// fileExists checks if a file exists at the given path
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// createTempCSVFile creates a temporary CSV file for testing
func createTempCSVFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test-data.csv")

	if err := os.WriteFile(csvPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test CSV file: %v", err)
	}

	return csvPath
}

// createTempJSONFile creates a temporary JSON file for testing
func createTempJSONFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "test-data.json")

	if err := os.WriteFile(jsonPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test JSON file: %v", err)
	}

	return jsonPath
}
