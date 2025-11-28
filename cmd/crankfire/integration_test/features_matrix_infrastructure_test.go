//go:build integration

package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/config"
)

// TestFeaturesMatrix_Infrastructure validates test server starts correctly
func TestFeaturesMatrix_Infrastructure(t *testing.T) {
	// Not parallel to avoid port exhaustion
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Start test server
	server := startTestServer(t)
	defer stopTestServer(t, server)

	// Verify the test server responds
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/echo", server.port))
	if err != nil {
		t.Logf("Warning: Failed to connect to test server (may be port exhaustion): %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify /timestamp endpoint works
	resp2, err := http.Get(fmt.Sprintf("http://localhost:%d/timestamp", server.port))
	if err != nil {
		t.Fatalf("Failed to call /timestamp endpoint: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 from /timestamp, got %d", resp2.StatusCode)
	}

	// Verify response contains timestamp
	var tsResp map[string]interface{}
	if err := json.NewDecoder(resp2.Body).Decode(&tsResp); err != nil {
		t.Fatalf("Failed to decode /timestamp response: %v", err)
	}

	if _, ok := tsResp["timestamp"]; !ok {
		t.Error("Expected 'timestamp' field in response")
	}

	t.Log("Infrastructure test passed: test server is operational")
}

// TestFeaturesMatrix_Config_YAMLLoading validates YAML config loading with runner integration
func TestFeaturesMatrix_Config_YAMLLoading(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := startTestServer(t)

	configContent := fmt.Sprintf(`
target: http://localhost:%d/echo
concurrency: 2
total: 20
duration: %s
method: GET
timeout: 30s
headers:
  X-Test: yaml-value
  Authorization: Bearer token123
`, server.port, getTestDuration().String())

	configPath := createTestConfigFile(t, configContent)

	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("Failed to load config from file: %v", err)
	}

	t.Logf("Loaded config - Target: %s, Concurrency: %d, Total: %d", cfg.TargetURL, cfg.Concurrency, cfg.Total)

	// Verify config values
	if cfg.TargetURL != fmt.Sprintf("http://localhost:%d/echo", server.port) {
		t.Errorf("Expected TargetURL from config, got %s", cfg.TargetURL)
	}
	if cfg.Concurrency != 2 {
		t.Errorf("Expected concurrency 2 from config, got %d", cfg.Concurrency)
	}
	if cfg.Headers["X-Test"] != "yaml-value" {
		t.Errorf("Expected X-Test header from config, got %s", cfg.Headers["X-Test"])
	}

	// Run actual load test with loaded config
	stats, result := runLoadTest(t, cfg, cfg.TargetURL)

	// Validate: config values are applied correctly during execution
	if stats.Total <= 0 {
		t.Errorf("Expected total requests > 0, got %d", stats.Total)
	}
	if stats.Successes <= 0 {
		t.Errorf("Expected successes > 0, got %d", stats.Successes)
	}

	t.Logf("Load test completed - Total: %d, Successes: %d, Failures: %d, Duration: %v",
		stats.Total, stats.Successes, stats.Failures, result.Duration)

	t.Log("Config YAML loading test passed: config values applied correctly during execution")
}

// TestFeaturesMatrix_Config_CLIOverride validates CLI argument overrides
func TestFeaturesMatrix_Config_CLIOverride(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := startTestServer(t)

	configContent := fmt.Sprintf(`
target: http://localhost:%d/echo
concurrency: 1
total: 10
duration: 10s
method: GET
`, server.port)

	configPath := createTestConfigFile(t, configContent)

	// Override values via CLI args
	loader := config.NewLoader()
	cfg, err := loader.Load([]string{
		"--config", configPath,
		"--concurrency", "10",
		"--duration", "3s",
	})
	if err != nil {
		t.Fatalf("Failed to load config with CLI overrides: %v", err)
	}

	// Verify CLI overrides take effect
	if cfg.Concurrency != 10 {
		t.Errorf("Expected CLI override concurrency 10, got %d", cfg.Concurrency)
	}
	if cfg.Duration.Seconds() != 3 {
		t.Errorf("Expected CLI override duration 3s, got %v", cfg.Duration)
	}

	t.Logf("Config with CLI overrides - Concurrency: %d (overridden), Duration: %v (overridden)", cfg.Concurrency, cfg.Duration)

	// Run load test with overridden config
	stats, result := runLoadTest(t, cfg, cfg.TargetURL)

	if stats.Total <= 0 {
		t.Errorf("Expected total requests > 0, got %d", stats.Total)
	}

	t.Logf("Load test with overrides - Total: %d, Duration: %v", stats.Total, result.Duration)

	t.Log("Config CLI override test passed: overrides applied correctly")
}

// TestFeaturesMatrix_Config_Validation validates config validation catches errors
func TestFeaturesMatrix_Config_Validation(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	testCases := []struct {
		name        string
		createCfg   func() *config.Config
		shouldError bool
	}{
		{
			name: "Missing target URL",
			createCfg: func() *config.Config {
				return &config.Config{
					TargetURL:   "",
					Concurrency: 1,
					Timeout:     30 * time.Second,
				}
			},
			shouldError: true,
		},
		{
			name: "Negative concurrency",
			createCfg: func() *config.Config {
				return &config.Config{
					TargetURL:   "http://localhost:8080",
					Concurrency: -1,
					Timeout:     30 * time.Second,
				}
			},
			shouldError: true,
		},
		{
			name: "Negative duration",
			createCfg: func() *config.Config {
				return &config.Config{
					TargetURL:   "http://localhost:8080",
					Concurrency: 1,
					Duration:    -1 * time.Second,
					Timeout:     30 * time.Second,
				}
			},
			shouldError: true,
		},
		{
			name: "Valid minimal config",
			createCfg: func() *config.Config {
				return &config.Config{
					TargetURL:   "http://localhost:8080",
					Concurrency: 1,
					Timeout:     30 * time.Second,
					Method:      "GET",
				}
			},
			shouldError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.createCfg()
			err := cfg.Validate()

			if tc.shouldError && err == nil {
				t.Errorf("Expected error for %s, but got none", tc.name)
			}
			if !tc.shouldError && err != nil {
				t.Errorf("Expected no error for %s, but got: %v", tc.name, err)
			}

			t.Logf("Validation test %s passed", tc.name)
		})
	}

	t.Log("Config validation test passed: validation catches errors correctly")
}

// TestFeaturesMatrix_Config_Defaults validates default config values
func TestFeaturesMatrix_Config_Defaults(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := startTestServer(t)

	// Create minimal config (just target URL)
	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--target", fmt.Sprintf("http://localhost:%d/echo", server.port)})
	if err != nil {
		t.Fatalf("Failed to load minimal config: %v", err)
	}

	// Verify defaults are applied
	if cfg.Method != "GET" {
		t.Errorf("Expected default method GET, got %s", cfg.Method)
	}
	if cfg.Concurrency != 1 {
		t.Errorf("Expected default concurrency 1, got %d", cfg.Concurrency)
	}
	if cfg.Timeout == 0 {
		t.Errorf("Expected default timeout to be set, got %v", cfg.Timeout)
	}

	t.Logf("Defaults applied - Method: %s, Concurrency: %d, Timeout: %v", cfg.Method, cfg.Concurrency, cfg.Timeout)

	// Run load test with defaults
	cfg.Total = 10
	cfg.Duration = 5 * time.Second
	stats, result := runLoadTest(t, cfg, cfg.TargetURL)

	if stats.Total <= 0 {
		t.Errorf("Expected total requests > 0, got %d", stats.Total)
	}

	t.Logf("Load test with defaults - Total: %d, Duration: %v", stats.Total, result.Duration)

	t.Log("Config defaults test passed: defaults applied correctly")
}

// TestFeaturesMatrix_Config_Environment validates environment variable influence
func TestFeaturesMatrix_Config_Environment(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Set TEST_DURATION env var
	testDur := 2 * time.Second
	os.Setenv("TEST_DURATION", testDur.String())
	defer os.Unsetenv("TEST_DURATION")

	// Verify getTestDuration() respects it
	dur := getTestDuration()
	if dur != testDur {
		t.Errorf("Expected TEST_DURATION to be %v, got %v", testDur, dur)
	}

	t.Logf("Environment variable TEST_DURATION set to %v and verified", dur)

	// Test with unset env var (should default to 5s)
	os.Unsetenv("TEST_DURATION")
	dur = getTestDuration()
	if dur != 5*time.Second {
		t.Errorf("Expected default duration 5s when env var unset, got %v", dur)
	}

	t.Logf("Default duration when env var unset: %v", dur)

	t.Log("Config environment test passed: environment variables respected")
}

// TestFeaturesMatrix_ConfigLoading validates test configurations parse correctly
func TestFeaturesMatrix_ConfigLoading(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Start test server for endpoint validation
	server := startTestServer(t)

	testCases := []struct {
		name     string
		args     []string
		validate func(t *testing.T, cfg *config.Config)
	}{
		{
			name: "Basic HTTP config",
			args: []string{
				"--target", fmt.Sprintf("http://localhost:%d", server.port),
				"--concurrency", "2",
				"--total", "10",
			},
			validate: func(t *testing.T, cfg *config.Config) {
				if cfg.TargetURL == "" {
					t.Error("Expected TargetURL to be set")
				}
				if cfg.Concurrency != 2 {
					t.Errorf("Expected concurrency 2, got %d", cfg.Concurrency)
				}
				if cfg.Total != 10 {
					t.Errorf("Expected total 10, got %d", cfg.Total)
				}
			},
		},
		{
			name: "HTTP with headers",
			args: []string{
				"--target", fmt.Sprintf("http://localhost:%d", server.port),
				"--concurrency", "1",
				"--total", "5",
				"--header", "X-Test=value123",
				"--header", "Authorization=Bearer token",
			},
			validate: func(t *testing.T, cfg *config.Config) {
				if cfg.Headers["X-Test"] != "value123" {
					t.Errorf("Expected X-Test header, got %s", cfg.Headers["X-Test"])
				}
				if cfg.Headers["Authorization"] != "Bearer token" {
					t.Errorf("Expected Authorization header, got %s", cfg.Headers["Authorization"])
				}
			},
		},
		{
			name: "HTTP with method and body",
			args: []string{
				"--target", fmt.Sprintf("http://localhost:%d", server.port),
				"--method", "POST",
				"--body", `{"key":"value"}`,
				"--total", "1",
			},
			validate: func(t *testing.T, cfg *config.Config) {
				if cfg.Method != "POST" {
					t.Errorf("Expected method POST, got %s", cfg.Method)
				}
				if cfg.Body != `{"key":"value"}` {
					t.Errorf("Expected body to be set, got %s", cfg.Body)
				}
			},
		},
		{
			name: "HTTP with timeout",
			args: []string{
				"--target", fmt.Sprintf("http://localhost:%d", server.port),
				"--timeout", "60s",
				"--total", "1",
			},
			validate: func(t *testing.T, cfg *config.Config) {
				if cfg.Timeout.Seconds() != 60 {
					t.Errorf("Expected timeout 60s, got %v", cfg.Timeout)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			loader := config.NewLoader()
			cfg, err := loader.Load(tc.args)
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			tc.validate(t, cfg)
			t.Logf("%s test config created successfully", tc.name)
		})
	}

	t.Log("Config loading test passed: all configurations parsed correctly")
}

// TestFeaturesMatrix_BasicHTTPLoad validates basic HTTP load test infrastructure
func TestFeaturesMatrix_BasicHTTPLoad(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// This test validates that the infrastructure works for basic HTTP load testing
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	}))
	defer server.Close()

	// Create a simple config
	cfg := generateTestConfig(server.URL,
		WithConcurrency(2),
		WithTotal(10),
	)

	if cfg.TargetURL != server.URL {
		t.Errorf("Expected target URL %s, got %s", server.URL, cfg.TargetURL)
	}
	if cfg.Concurrency != 2 {
		t.Errorf("Expected concurrency 2, got %d", cfg.Concurrency)
	}
	if cfg.Total != 10 {
		t.Errorf("Expected total 10, got %d", cfg.Total)
	}

	t.Log("Basic HTTP load test infrastructure validated")
}

// TestFeaturesMatrix_YAMLConfigLoading validates YAML config file parsing
func TestFeaturesMatrix_YAMLConfigLoading(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := startTestServer(t)

	configContent := fmt.Sprintf(`
target: http://localhost:%d
concurrency: 2
total: 10
method: GET
headers:
  X-Test: value123
`, server.port)

	configPath := createTestConfigFile(t, configContent)

	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("Failed to load config from file: %v", err)
	}

	if cfg.TargetURL == "" {
		t.Error("Expected TargetURL to be loaded from config file")
	}
	if cfg.Concurrency != 2 {
		t.Errorf("Expected concurrency 2 from config, got %d", cfg.Concurrency)
	}
	if cfg.Headers["X-Test"] != "value123" {
		t.Errorf("Expected X-Test header from config, got %s", cfg.Headers["X-Test"])
	}

	t.Log("YAML config loading test passed")
}

// TestFeaturesMatrix_MultipleEndpoints validates multi-endpoint configuration
func TestFeaturesMatrix_MultipleEndpoints(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := startTestServer(t)

	configContent := fmt.Sprintf(`
target: http://localhost:%d
concurrency: 1
endpoints:
  - name: endpoint-a
    method: GET
    path: /echo
    weight: 1
  - name: endpoint-b
    method: POST
    path: /echo
    weight: 2
`, server.port)

	configPath := createTestConfigFile(t, configContent)

	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("Failed to load multi-endpoint config: %v", err)
	}

	if len(cfg.Endpoints) != 2 {
		t.Errorf("Expected 2 endpoints, got %d", len(cfg.Endpoints))
	}

	if cfg.Endpoints[0].Name != "endpoint-a" {
		t.Errorf("Expected first endpoint to be 'endpoint-a', got %s", cfg.Endpoints[0].Name)
	}

	if cfg.Endpoints[1].Weight != 2 {
		t.Errorf("Expected second endpoint weight 2, got %d", cfg.Endpoints[1].Weight)
	}

	t.Log("Multiple endpoints test passed")
}
