package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torosent/crankfire/internal/config"
)

// TestHARIntegration_BasicRun tests loading a HAR file and verifying endpoints are created.
func TestHARIntegration_BasicRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a HAR file with endpoints
	harContent := `{
  "log": {
    "version": "1.2",
    "creator": {"name": "test", "version": "1.0"},
    "pages": [],
    "entries": [
      {
        "startedDateTime": "2025-01-15T10:00:00.000Z",
        "time": 0.05,
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/api/users",
          "httpVersion": "HTTP/1.1",
          "headers": [{"name": "Accept", "value": "application/json"}],
          "queryString": [],
          "cookies": [],
          "headersSize": 100,
          "bodySize": 0
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "cookies": [],
          "content": {"size": 0, "mimeType": ""},
          "redirectURL": "",
          "headersSize": 0,
          "bodySize": 0
        },
        "cache": {},
        "timings": {"blocked": 0, "dns": 0, "connect": 0, "send": 5, "wait": 25, "receive": 20}
      },
      {
        "startedDateTime": "2025-01-15T10:00:00.100Z",
        "time": 0.05,
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/api/users",
          "httpVersion": "HTTP/1.1",
          "headers": [{"name": "Content-Type", "value": "application/json"}],
          "queryString": [],
          "cookies": [],
          "postData": {"mimeType": "application/json", "text": "{}"},
          "headersSize": 100,
          "bodySize": 20
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "cookies": [],
          "content": {"size": 0, "mimeType": ""},
          "redirectURL": "",
          "headersSize": 0,
          "bodySize": 0
        },
        "cache": {},
        "timings": {"blocked": 0, "dns": 0, "connect": 0, "send": 5, "wait": 25, "receive": 20}
      }
    ]
  }
}`

	tmpDir := t.TempDir()
	harFile := filepath.Join(tmpDir, "test.har")
	if err := os.WriteFile(harFile, []byte(harContent), 0644); err != nil {
		t.Fatalf("Failed to write HAR file: %v", err)
	}

	// Load the HAR file and verify endpoints are created
	loader := config.NewLoader()
	args := []string{
		"--har", harFile,
		"--concurrency", "1",
		"--total", "3",
	}

	cfg, err := loader.Load(args)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := loadHAREndpoints(cfg); err != nil {
		t.Fatalf("Failed to load HAR endpoints: %v", err)
	}

	// Verify that endpoints were created from HAR
	if len(cfg.Endpoints) == 0 {
		t.Fatalf("Expected endpoints from HAR file, got none")
	}

	t.Logf("Successfully created %d endpoints from HAR file", len(cfg.Endpoints))

	// Verify endpoints have expected names and methods
	endpointMethods := make(map[string]int)
	for _, ep := range cfg.Endpoints {
		method := strings.ToUpper(ep.Method)
		endpointMethods[method]++
	}

	// We expect at least 1 GET and 1 POST request
	if endpointMethods["GET"] < 1 {
		t.Errorf("Expected at least 1 GET endpoint, got %d", endpointMethods["GET"])
	}
	if endpointMethods["POST"] < 1 {
		t.Errorf("Expected at least 1 POST endpoint, got %d", endpointMethods["POST"])
	}

	t.Logf("Endpoint methods: %v", endpointMethods)
}

// TestHARIntegration_WithFilter_MethodGET tests filtering HAR entries by GET method only.
func TestHARIntegration_WithFilter_MethodGET(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a HAR file with mixed methods
	harContent := createTestHARWithMixedMethods()
	tmpDir := t.TempDir()
	harFile := filepath.Join(tmpDir, "test.har")
	if err := os.WriteFile(harFile, []byte(harContent), 0644); err != nil {
		t.Fatalf("Failed to write HAR file: %v", err)
	}

	// Load the HAR file with a filter for GET method only
	loader := config.NewLoader()
	args := []string{
		"--har", harFile,
		"--har-filter", "method:GET",
	}

	cfg, err := loader.Load(args)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := loadHAREndpoints(cfg); err != nil {
		t.Fatalf("Failed to load HAR endpoints: %v", err)
	}

	// Verify that only GET endpoints were created
	if len(cfg.Endpoints) == 0 {
		t.Fatalf("Expected GET endpoints from HAR file, got none")
	}

	for _, ep := range cfg.Endpoints {
		method := strings.ToUpper(ep.Method)
		if method != "GET" {
			t.Errorf("Expected GET method, got %s for endpoint %s", method, ep.Name)
		}
	}

	t.Logf("Successfully filtered HAR to %d GET endpoints", len(cfg.Endpoints))
}

// TestHARIntegration_WithFilter_MethodPOST tests filtering HAR entries by POST method only.
func TestHARIntegration_WithFilter_MethodPOST(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a HAR file with mixed methods
	harContent := createTestHARWithMixedMethods()
	tmpDir := t.TempDir()
	harFile := filepath.Join(tmpDir, "test.har")
	if err := os.WriteFile(harFile, []byte(harContent), 0644); err != nil {
		t.Fatalf("Failed to write HAR file: %v", err)
	}

	// Load the HAR file with a filter for POST method only
	loader := config.NewLoader()
	args := []string{
		"--har", harFile,
		"--har-filter", "method:POST",
	}

	cfg, err := loader.Load(args)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := loadHAREndpoints(cfg); err != nil {
		t.Fatalf("Failed to load HAR endpoints: %v", err)
	}

	// Verify that POST endpoints were created and no GET endpoints
	if len(cfg.Endpoints) == 0 {
		t.Fatalf("Expected POST endpoints from HAR file, got none")
	}

	for _, ep := range cfg.Endpoints {
		method := strings.ToUpper(ep.Method)
		if method != "POST" {
			t.Errorf("Expected POST method, got %s for endpoint %s", method, ep.Name)
		}
	}

	t.Logf("Successfully filtered HAR to %d POST endpoints", len(cfg.Endpoints))
}

// TestHARIntegration_ExcludesStaticAssets tests that static assets (.js, .css, images) are excluded by default.
func TestHARIntegration_ExcludesStaticAssets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a HAR file with mixed API and static asset requests
	harContent := createTestHARWithStaticAssets()
	tmpDir := t.TempDir()
	harFile := filepath.Join(tmpDir, "test.har")
	if err := os.WriteFile(harFile, []byte(harContent), 0644); err != nil {
		t.Fatalf("Failed to write HAR file: %v", err)
	}

	// Load the HAR file (static assets should be excluded by default)
	loader := config.NewLoader()
	args := []string{
		"--har", harFile,
	}

	cfg, err := loader.Load(args)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := loadHAREndpoints(cfg); err != nil {
		t.Fatalf("Failed to load HAR endpoints: %v", err)
	}

	// Verify that static assets are excluded
	for _, ep := range cfg.Endpoints {
		if strings.HasSuffix(ep.URL, ".js") || strings.HasSuffix(ep.URL, ".css") ||
			strings.HasSuffix(ep.URL, ".png") || strings.HasSuffix(ep.URL, ".jpg") ||
			strings.HasSuffix(ep.URL, ".gif") || strings.HasSuffix(ep.URL, ".svg") {
			t.Errorf("Static asset found in endpoints: %s", ep.URL)
		}
	}

	// Count the number of API vs static endpoints
	// In the test HAR, we have 6 API endpoints and 3 static assets
	// After filtering, we should only have the 6 API endpoints
	if len(cfg.Endpoints) < 5 {
		t.Errorf("Expected at least 5 API endpoints after filtering static assets, got %d", len(cfg.Endpoints))
	}

	t.Logf("Successfully excluded static assets, %d API endpoints remaining", len(cfg.Endpoints))
}

// TestHARIntegration_MergeWithExistingEndpoints tests that HAR endpoints are merged with config file endpoints.
func TestHARIntegration_MergeWithExistingEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a config file with some endpoints
	configContent := `target: http://localhost:8080
concurrency: 1
total: 5
endpoints:
  - name: config-endpoint
    path: /config-api
    method: GET
`
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create a HAR file with endpoints
	harContent := `{
  "log": {
    "version": "1.2",
    "creator": {"name": "test", "version": "1.0"},
    "pages": [],
    "entries": [
      {
        "startedDateTime": "2025-01-15T10:00:00.000Z",
        "time": 0.05,
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/api/users",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "queryString": [],
          "cookies": [],
          "headersSize": 100,
          "bodySize": 0
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "cookies": [],
          "content": {"size": 0, "mimeType": ""},
          "redirectURL": "",
          "headersSize": 0,
          "bodySize": 0
        },
        "cache": {},
        "timings": {"blocked": 0, "dns": 0, "connect": 0, "send": 5, "wait": 25, "receive": 20}
      },
      {
        "startedDateTime": "2025-01-15T10:00:00.100Z",
        "time": 0.05,
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/api/users",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "queryString": [],
          "cookies": [],
          "postData": {"mimeType": "application/json", "text": "{}"},
          "headersSize": 100,
          "bodySize": 20
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "cookies": [],
          "content": {"size": 0, "mimeType": ""},
          "redirectURL": "",
          "headersSize": 0,
          "bodySize": 0
        },
        "cache": {},
        "timings": {"blocked": 0, "dns": 0, "connect": 0, "send": 5, "wait": 25, "receive": 20}
      }
    ]
  }
}`
	harFile := filepath.Join(tmpDir, "test.har")
	if err := os.WriteFile(harFile, []byte(harContent), 0644); err != nil {
		t.Fatalf("Failed to write HAR file: %v", err)
	}

	// Load both config file and HAR file
	loader := config.NewLoader()
	args := []string{
		"--config", configFile,
		"--har", harFile,
	}

	cfg, err := loader.Load(args)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Before loading HAR endpoints, we should only have 1 from config
	if len(cfg.Endpoints) != 1 {
		t.Fatalf("Expected 1 endpoint from config, got %d", len(cfg.Endpoints))
	}

	if err := loadHAREndpoints(cfg); err != nil {
		t.Fatalf("Failed to load HAR endpoints: %v", err)
	}

	// After loading HAR endpoints, we should have more than 1
	if len(cfg.Endpoints) <= 1 {
		t.Fatalf("Expected more than 1 endpoint after merging HAR, got %d", len(cfg.Endpoints))
	}

	// Verify that the config endpoint is still present
	configEndpointFound := false
	for _, ep := range cfg.Endpoints {
		if ep.Name == "config-endpoint" {
			configEndpointFound = true
			break
		}
	}
	if !configEndpointFound {
		t.Error("Original config endpoint was not preserved after merging HAR")
	}

	t.Logf("Successfully merged %d HAR endpoints with config endpoints (total: %d)", len(cfg.Endpoints)-1, len(cfg.Endpoints))
}

// TestHARIntegration_EndpointContent tests that endpoints have proper content (headers, body, etc).
func TestHARIntegration_EndpointContent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a HAR file with rich content
	harContent := `{
  "log": {
    "version": "1.2",
    "creator": {"name": "test", "version": "1.0"},
    "pages": [],
    "entries": [
      {
        "startedDateTime": "2025-01-15T10:00:00.000Z",
        "time": 0.05,
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/api/users",
          "httpVersion": "HTTP/1.1",
          "headers": [
            {"name": "User-Agent", "value": "Mozilla/5.0"},
            {"name": "Accept", "value": "application/json"}
          ],
          "queryString": [
            {"name": "limit", "value": "10"}
          ],
          "cookies": [],
          "headersSize": 150,
          "bodySize": 0
        },
        "response": {"status": 200, "statusText": "OK", "httpVersion": "HTTP/1.1", "headers": [], "cookies": [], "content": {"size": 0, "mimeType": ""}, "redirectURL": "", "headersSize": 0, "bodySize": 0},
        "cache": {},
        "timings": {"blocked": 0, "dns": 0, "connect": 0, "send": 5, "wait": 25, "receive": 20}
      },
      {
        "startedDateTime": "2025-01-15T10:00:00.100Z",
        "time": 0.05,
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/api/users",
          "httpVersion": "HTTP/1.1",
          "headers": [
            {"name": "Content-Type", "value": "application/json"},
            {"name": "User-Agent", "value": "Mozilla/5.0"}
          ],
          "queryString": [],
          "cookies": [],
          "postData": {
            "mimeType": "application/json",
            "text": "{\"name\": \"John Doe\", \"email\": \"john@example.com\"}"
          },
          "headersSize": 150,
          "bodySize": 50
        },
        "response": {"status": 201, "statusText": "Created", "httpVersion": "HTTP/1.1", "headers": [], "cookies": [], "content": {"size": 0, "mimeType": ""}, "redirectURL": "", "headersSize": 0, "bodySize": 0},
        "cache": {},
        "timings": {"blocked": 0, "dns": 0, "connect": 0, "send": 5, "wait": 25, "receive": 20}
      }
    ]
  }
}`

	tmpDir := t.TempDir()
	harFile := filepath.Join(tmpDir, "test.har")
	if err := os.WriteFile(harFile, []byte(harContent), 0644); err != nil {
		t.Fatalf("Failed to write HAR file: %v", err)
	}

	// Load the HAR file
	loader := config.NewLoader()
	args := []string{
		"--har", harFile,
		"--har-filter", "method:POST",
	}

	cfg, err := loader.Load(args)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := loadHAREndpoints(cfg); err != nil {
		t.Fatalf("Failed to load HAR endpoints: %v", err)
	}

	// Verify that at least one endpoint has a body
	foundBodyContent := false
	for _, ep := range cfg.Endpoints {
		if strings.TrimSpace(ep.Body) != "" {
			foundBodyContent = true
			t.Logf("Found endpoint with body: %s", ep.Body)
			break
		}
	}

	if !foundBodyContent {
		t.Error("Expected at least one endpoint to have body content from HAR")
	}

	// Verify that POST endpoints have the correct method
	for _, ep := range cfg.Endpoints {
		if strings.ToUpper(ep.Method) != "POST" {
			t.Errorf("Expected POST method, got %s", ep.Method)
		}
	}

	t.Logf("Successfully verified endpoint content from HAR file")
}

// createTestHARWithMixedMethods creates a HAR file with GET, POST, PUT, DELETE methods
func createTestHARWithMixedMethods() string {
	return `{
  "log": {
    "version": "1.2",
    "creator": {"name": "test", "version": "1.0"},
    "pages": [],
    "entries": [
      {
        "startedDateTime": "2025-01-15T10:00:00.000Z",
        "time": 0.05,
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/api/users",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "queryString": [],
          "cookies": [],
          "headersSize": 100,
          "bodySize": 0
        },
        "response": {"status": 200, "statusText": "OK", "httpVersion": "HTTP/1.1", "headers": [], "cookies": [], "content": {"size": 0, "mimeType": ""}, "redirectURL": "", "headersSize": 0, "bodySize": 0},
        "cache": {},
        "timings": {"blocked": 0, "dns": 0, "connect": 0, "send": 5, "wait": 25, "receive": 20}
      },
      {
        "startedDateTime": "2025-01-15T10:00:00.100Z",
        "time": 0.05,
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/api/products",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "queryString": [],
          "cookies": [],
          "headersSize": 100,
          "bodySize": 0
        },
        "response": {"status": 200, "statusText": "OK", "httpVersion": "HTTP/1.1", "headers": [], "cookies": [], "content": {"size": 0, "mimeType": ""}, "redirectURL": "", "headersSize": 0, "bodySize": 0},
        "cache": {},
        "timings": {"blocked": 0, "dns": 0, "connect": 0, "send": 5, "wait": 25, "receive": 20}
      },
      {
        "startedDateTime": "2025-01-15T10:00:00.200Z",
        "time": 0.05,
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/api/users",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "queryString": [],
          "cookies": [],
          "postData": {"mimeType": "application/json", "text": "{}"},
          "headersSize": 100,
          "bodySize": 20
        },
        "response": {"status": 201, "statusText": "Created", "httpVersion": "HTTP/1.1", "headers": [], "cookies": [], "content": {"size": 0, "mimeType": ""}, "redirectURL": "", "headersSize": 0, "bodySize": 0},
        "cache": {},
        "timings": {"blocked": 0, "dns": 0, "connect": 0, "send": 5, "wait": 25, "receive": 20}
      },
      {
        "startedDateTime": "2025-01-15T10:00:00.300Z",
        "time": 0.05,
        "request": {
          "method": "PUT",
          "url": "http://localhost:8080/api/users/1",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "queryString": [],
          "cookies": [],
          "postData": {"mimeType": "application/json", "text": "{}"},
          "headersSize": 100,
          "bodySize": 20
        },
        "response": {"status": 200, "statusText": "OK", "httpVersion": "HTTP/1.1", "headers": [], "cookies": [], "content": {"size": 0, "mimeType": ""}, "redirectURL": "", "headersSize": 0, "bodySize": 0},
        "cache": {},
        "timings": {"blocked": 0, "dns": 0, "connect": 0, "send": 5, "wait": 25, "receive": 20}
      },
      {
        "startedDateTime": "2025-01-15T10:00:00.400Z",
        "time": 0.05,
        "request": {
          "method": "DELETE",
          "url": "http://localhost:8080/api/users/1",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "queryString": [],
          "cookies": [],
          "headersSize": 100,
          "bodySize": 0
        },
        "response": {"status": 204, "statusText": "No Content", "httpVersion": "HTTP/1.1", "headers": [], "cookies": [], "content": {"size": 0, "mimeType": ""}, "redirectURL": "", "headersSize": 0, "bodySize": 0},
        "cache": {},
        "timings": {"blocked": 0, "dns": 0, "connect": 0, "send": 5, "wait": 25, "receive": 20}
      }
    ]
  }
}`
}

// createTestHARWithStaticAssets creates a HAR file with both API and static asset requests
func createTestHARWithStaticAssets() string {
	return `{
  "log": {
    "version": "1.2",
    "creator": {"name": "test", "version": "1.0"},
    "pages": [],
    "entries": [
      {
        "startedDateTime": "2025-01-15T10:00:00.000Z",
        "time": 0.05,
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/api/users",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "queryString": [],
          "cookies": [],
          "headersSize": 100,
          "bodySize": 0
        },
        "response": {"status": 200, "statusText": "OK", "httpVersion": "HTTP/1.1", "headers": [], "cookies": [], "content": {"size": 0, "mimeType": ""}, "redirectURL": "", "headersSize": 0, "bodySize": 0},
        "cache": {},
        "timings": {"blocked": 0, "dns": 0, "connect": 0, "send": 5, "wait": 25, "receive": 20}
      },
      {
        "startedDateTime": "2025-01-15T10:00:00.100Z",
        "time": 0.05,
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/api/products",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "queryString": [],
          "cookies": [],
          "headersSize": 100,
          "bodySize": 0
        },
        "response": {"status": 200, "statusText": "OK", "httpVersion": "HTTP/1.1", "headers": [], "cookies": [], "content": {"size": 0, "mimeType": ""}, "redirectURL": "", "headersSize": 0, "bodySize": 0},
        "cache": {},
        "timings": {"blocked": 0, "dns": 0, "connect": 0, "send": 5, "wait": 25, "receive": 20}
      },
      {
        "startedDateTime": "2025-01-15T10:00:00.200Z",
        "time": 0.05,
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/api/orders",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "queryString": [],
          "cookies": [],
          "headersSize": 100,
          "bodySize": 0
        },
        "response": {"status": 200, "statusText": "OK", "httpVersion": "HTTP/1.1", "headers": [], "cookies": [], "content": {"size": 0, "mimeType": ""}, "redirectURL": "", "headersSize": 0, "bodySize": 0},
        "cache": {},
        "timings": {"blocked": 0, "dns": 0, "connect": 0, "send": 5, "wait": 25, "receive": 20}
      },
      {
        "startedDateTime": "2025-01-15T10:00:00.300Z",
        "time": 0.05,
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/api/auth/login",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "queryString": [],
          "cookies": [],
          "headersSize": 100,
          "bodySize": 0
        },
        "response": {"status": 200, "statusText": "OK", "httpVersion": "HTTP/1.1", "headers": [], "cookies": [], "content": {"size": 0, "mimeType": ""}, "redirectURL": "", "headersSize": 0, "bodySize": 0},
        "cache": {},
        "timings": {"blocked": 0, "dns": 0, "connect": 0, "send": 5, "wait": 25, "receive": 20}
      },
      {
        "startedDateTime": "2025-01-15T10:00:00.400Z",
        "time": 0.05,
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/api/data",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "queryString": [],
          "cookies": [],
          "postData": {"mimeType": "application/json", "text": "{}"},
          "headersSize": 100,
          "bodySize": 20
        },
        "response": {"status": 201, "statusText": "Created", "httpVersion": "HTTP/1.1", "headers": [], "cookies": [], "content": {"size": 0, "mimeType": ""}, "redirectURL": "", "headersSize": 0, "bodySize": 0},
        "cache": {},
        "timings": {"blocked": 0, "dns": 0, "connect": 0, "send": 5, "wait": 25, "receive": 20}
      },
      {
        "startedDateTime": "2025-01-15T10:00:00.500Z",
        "time": 0.05,
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/static/app.js",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "queryString": [],
          "cookies": [],
          "headersSize": 100,
          "bodySize": 0
        },
        "response": {"status": 200, "statusText": "OK", "httpVersion": "HTTP/1.1", "headers": [], "cookies": [], "content": {"size": 0, "mimeType": "application/javascript"}, "redirectURL": "", "headersSize": 0, "bodySize": 0},
        "cache": {},
        "timings": {"blocked": 0, "dns": 0, "connect": 0, "send": 5, "wait": 25, "receive": 20}
      },
      {
        "startedDateTime": "2025-01-15T10:00:00.600Z",
        "time": 0.05,
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/static/style.css",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "queryString": [],
          "cookies": [],
          "headersSize": 100,
          "bodySize": 0
        },
        "response": {"status": 200, "statusText": "OK", "httpVersion": "HTTP/1.1", "headers": [], "cookies": [], "content": {"size": 0, "mimeType": "text/css"}, "redirectURL": "", "headersSize": 0, "bodySize": 0},
        "cache": {},
        "timings": {"blocked": 0, "dns": 0, "connect": 0, "send": 5, "wait": 25, "receive": 20}
      },
      {
        "startedDateTime": "2025-01-15T10:00:00.700Z",
        "time": 0.05,
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/images/logo.png",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "queryString": [],
          "cookies": [],
          "headersSize": 100,
          "bodySize": 0
        },
        "response": {"status": 200, "statusText": "OK", "httpVersion": "HTTP/1.1", "headers": [], "cookies": [], "content": {"size": 0, "mimeType": "image/png"}, "redirectURL": "", "headersSize": 0, "bodySize": 0},
        "cache": {},
        "timings": {"blocked": 0, "dns": 0, "connect": 0, "send": 5, "wait": 25, "receive": 20}
      }
    ]
  }
}`
}
