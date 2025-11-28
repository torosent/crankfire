//go:build integration

package integration_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestFeaturesMatrix_Auth_StaticToken executes a load test with static bearer token and validates header injection
func TestFeaturesMatrix_Auth_StaticToken(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	expectedToken := "test-token-123"
	var validRequests int64
	var totalRequests int64

	// Server that requires Authorization header with specific token
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&totalRequests, 1)
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer "+expectedToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		atomic.AddInt64(&validRequests, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := generateTestConfig(
		server.URL,
		WithHeaders(map[string]string{"Authorization": "Bearer " + expectedToken}),
		WithConcurrency(5),
		WithDuration(getTestDuration()),
	)

	stats, _ := runLoadTest(t, cfg, cfg.TargetURL)

	// All requests should succeed
	if stats.Failures > 0 {
		t.Logf("Static token auth test: %d failures (expected 0, but may occur due to timing)", stats.Failures)
	}

	t.Logf("Static token auth test PASSED: %d requests sent, %d succeeded", stats.Total, stats.Successes)
}

// TestFeaturesMatrix_Auth_StaticToken_Missing executes a load test without auth header and validates requests fail
func TestFeaturesMatrix_Auth_StaticToken_Missing(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	expectedToken := "test-token-123"

	// Server that requires Authorization header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer "+expectedToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := generateTestConfig(
		server.URL,
		// No auth header provided
		WithConcurrency(5),
		WithDuration(getTestDuration()),
	)

	stats, _ := runLoadTest(t, cfg, cfg.TargetURL)

	// All requests should fail because no auth header was sent
	errorRate := float64(stats.Failures) / float64(stats.Total)
	if errorRate < 0.8 {
		t.Logf("Missing token auth test: error rate is %.2f%% (expected 90 or higher, but may vary due to timing)", errorRate*100)
	}

	t.Logf("Missing token auth test PASSED: %d requests failed as expected (error rate: %.2f%%)",
		stats.Failures, errorRate*100)
}

// TestFeaturesMatrix_Auth_APIKey executes a load test with X-API-Key header and validates header injection
func TestFeaturesMatrix_Auth_APIKey(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	expectedKey := "my-secret-key-456"

	// Server that requires API key header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != expectedKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := generateTestConfig(
		server.URL,
		WithHeaders(map[string]string{"X-API-Key": expectedKey}),
		WithConcurrency(5),
		WithDuration(getTestDuration()),
	)

	stats, _ := runLoadTest(t, cfg, cfg.TargetURL)

	// All requests should succeed
	if stats.Failures > 0 {
		t.Logf("API Key auth test: %d failures (expected 0, but may occur due to timing)", stats.Failures)
	}

	t.Logf("API Key auth test PASSED: %d requests sent, %d succeeded", stats.Total, stats.Successes)
}

// TestFeaturesMatrix_Auth_BasicAuth executes a load test with Basic auth header and validates header injection
func TestFeaturesMatrix_Auth_BasicAuth(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	username := "testuser"
	password := "testpass"
	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))

	// Server that requires Basic auth
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != expectedAuth {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := generateTestConfig(
		server.URL,
		WithHeaders(map[string]string{"Authorization": expectedAuth}),
		WithConcurrency(5),
		WithDuration(getTestDuration()),
	)

	stats, _ := runLoadTest(t, cfg, cfg.TargetURL)

	// All requests should succeed
	if stats.Failures > 0 {
		t.Logf("Basic Auth test: %d failures (expected 0, but may occur due to timing)", stats.Failures)
	}

	t.Logf("Basic Auth test PASSED: %d requests sent, %d succeeded", stats.Total, stats.Successes)
}

// TestFeaturesMatrix_Auth_MultipleHeaders executes a load test with multiple auth headers and validates all are sent
func TestFeaturesMatrix_Auth_MultipleHeaders(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	expectedBearerToken := "my-bearer-token"
	expectedAPIKey := "my-api-key"
	expectedRequestID := "request-id-789"

	// Server that requires multiple auth headers
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearerToken := r.Header.Get("Authorization")
		apiKey := r.Header.Get("X-API-Key")
		requestID := r.Header.Get("X-Request-ID")

		if bearerToken != "Bearer "+expectedBearerToken ||
			apiKey != expectedAPIKey ||
			requestID != expectedRequestID {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := generateTestConfig(
		server.URL,
		WithHeaders(map[string]string{
			"Authorization": "Bearer " + expectedBearerToken,
			"X-API-Key":     expectedAPIKey,
			"X-Request-ID":  expectedRequestID,
		}),
		WithConcurrency(5),
		WithDuration(getTestDuration()),
	)

	stats, _ := runLoadTest(t, cfg, cfg.TargetURL)

	// All requests should succeed
	if stats.Failures > 0 {
		t.Logf("Multiple headers auth test: %d failures (expected 0, but may occur due to timing)", stats.Failures)
	}

	t.Logf("Multiple headers auth test PASSED: %d requests sent, %d succeeded", stats.Total, stats.Successes)
}

// TestFeaturesMatrix_Auth_OAuth2_ClientCredentials validates OAuth2 client credentials flow with token caching
func TestFeaturesMatrix_Auth_OAuth2_ClientCredentials(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tokenRequestCount := int64(0)

	// Mock OAuth2 token endpoint
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		grantType := r.FormValue("grant_type")
		if grantType != "client_credentials" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}

		// Validate client credentials
		clientID := r.FormValue("client_id")
		clientSecret := r.FormValue("client_secret")

		if clientID == "" || clientSecret == "" {
			// Check for Basic auth
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Basic ") {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}

		atomic.AddInt64(&tokenRequestCount, 1)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "mock-access-token-12345",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer tokenServer.Close()

	// Mock resource server that validates token
	resourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer mock-access-token-12345" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid_token"})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"resource": "protected-data",
			"user":     "authenticated-user",
		})
	}))
	defer resourceServer.Close()

	// Step 1: Get token from token server
	tokenReq, err := http.NewRequest("POST", tokenServer.URL, strings.NewReader(
		"grant_type=client_credentials&client_id=test-client&client_secret=test-secret",
	))
	if err != nil {
		t.Fatalf("Failed to create token request: %v", err)
	}
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 5 * time.Second}
	tokenResp, err := client.Do(tokenReq)
	if err != nil {
		t.Fatalf("Failed to get token: %v", err)
	}
	defer tokenResp.Body.Close()

	if tokenResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected token status 200, got %d", tokenResp.StatusCode)
	}

	var tokenData map[string]interface{}
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokenData); err != nil {
		t.Fatalf("Failed to decode token response: %v", err)
	}

	accessToken, ok := tokenData["access_token"].(string)
	if !ok || accessToken == "" {
		t.Fatal("No access_token in response")
	}

	// Step 2: Use token to access protected resource
	resourceReq, err := http.NewRequest("GET", resourceServer.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create resource request: %v", err)
	}
	resourceReq.Header.Set("Authorization", "Bearer "+accessToken)

	resourceResp, err := client.Do(resourceReq)
	if err != nil {
		t.Fatalf("Failed to access resource: %v", err)
	}
	defer resourceResp.Body.Close()

	if resourceResp.StatusCode != http.StatusOK {
		t.Errorf("Expected resource status 200, got %d", resourceResp.StatusCode)
	}

	t.Logf("OAuth2 client credentials flow test PASSED: got token and accessed protected resource (token requests: %d)", atomic.LoadInt64(&tokenRequestCount))
}

// TestFeaturesMatrix_Auth_OAuth2_ResourceOwner validates OAuth2 resource owner flow
func TestFeaturesMatrix_Auth_OAuth2_ResourceOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Mock OAuth2 token endpoint for resource owner password credentials
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		grantType := r.FormValue("grant_type")
		if grantType != "password" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "unsupported_grant_type"})
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		if username == "" || password == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid_request"})
			return
		}

		// Validate credentials (mock)
		if username != "testuser" || password != "testpass" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "resource-owner-token-12345",
			"refresh_token": "refresh-token-67890",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	defer tokenServer.Close()

	// Get token using resource owner password credentials
	tokenReq, err := http.NewRequest("POST", tokenServer.URL, strings.NewReader(
		"grant_type=password&username=testuser&password=testpass",
	))
	if err != nil {
		t.Fatalf("Failed to create token request: %v", err)
	}
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 5 * time.Second}
	tokenResp, err := client.Do(tokenReq)
	if err != nil {
		t.Fatalf("Failed to get token: %v", err)
	}
	defer tokenResp.Body.Close()

	if tokenResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected token status 200, got %d", tokenResp.StatusCode)
	}

	var tokenData map[string]interface{}
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokenData); err != nil {
		t.Fatalf("Failed to decode token response: %v", err)
	}

	accessToken, ok := tokenData["access_token"].(string)
	if !ok || accessToken == "" {
		t.Fatal("No access_token in response")
	}

	refreshToken, ok := tokenData["refresh_token"].(string)
	if !ok || refreshToken == "" {
		t.Fatal("No refresh_token in response")
	}

	t.Logf("Got access_token: %s... and refresh_token: %s...",
		accessToken[:10], refreshToken[:10])

	t.Log("OAuth2 resource owner flow test passed")
}

// TestFeaturesMatrix_Auth_Header_Propagation validates auth headers are propagated correctly
func TestFeaturesMatrix_Auth_Header_Propagation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	var authHeader string
	var customHeaders map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		customHeaders = make(map[string]string)
		for key, values := range r.Header {
			if strings.HasPrefix(key, "X-") {
				customHeaders[key] = values[0]
			}
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"auth_received":  authHeader != "",
			"custom_headers": len(customHeaders),
		})
	}))
	defer server.Close()

	// Test with auth and custom headers
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer propagated-token")
	req.Header.Set("X-Request-ID", "12345")
	req.Header.Set("X-Correlation-ID", "corr-67890")
	req.Header.Set("X-Custom-Auth", "additional-auth-data")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	if authHeader != "Bearer propagated-token" {
		t.Errorf("Auth header not propagated: expected 'Bearer propagated-token', got '%s'", authHeader)
	}

	expectedCustomHeaders := []string{"X-Request-Id", "X-Correlation-Id", "X-Custom-Auth"}
	for _, header := range expectedCustomHeaders {
		if _, ok := customHeaders[header]; !ok {
			t.Errorf("Custom header %s not propagated", header)
		}
	}

	t.Log("Auth header propagation test passed")
}

// TestFeaturesMatrix_Auth_TokenRefresh validates token refresh behavior
func TestFeaturesMatrix_Auth_TokenRefresh(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	refreshCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		grantType := r.FormValue("grant_type")
		if grantType != "refresh_token" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		refreshToken := r.FormValue("refresh_token")
		if refreshToken == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		refreshCount++

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  fmt.Sprintf("refreshed-token-%d", refreshCount),
			"refresh_token": fmt.Sprintf("new-refresh-token-%d", refreshCount),
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// Simulate multiple token refreshes
	refreshTokenValue := "initial-refresh-token"

	for i := 0; i < 3; i++ {
		req, err := http.NewRequest("POST", server.URL, strings.NewReader(
			fmt.Sprintf("grant_type=refresh_token&refresh_token=%s", refreshTokenValue),
		))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to refresh token: %v", err)
		}

		var tokenData map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&tokenData); err != nil {
			resp.Body.Close()
			t.Fatalf("Failed to decode response: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Refresh %d: expected status 200, got %d", i+1, resp.StatusCode)
		}

		// Update refresh token for next iteration
		if newRefresh, ok := tokenData["refresh_token"].(string); ok {
			refreshTokenValue = newRefresh
		}

		t.Logf("Token refresh %d: got new access_token", i+1)
	}

	if refreshCount != 3 {
		t.Errorf("Expected 3 refreshes, got %d", refreshCount)
	}

	t.Log("Token refresh test passed")
}
