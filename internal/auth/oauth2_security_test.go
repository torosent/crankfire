package auth

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestOAuth2ClientCredentials_UsesBasicAuth verifies that client credentials
// are passed via Basic Auth header instead of form fields
func TestOAuth2ClientCredentials_UsesBasicAuth(t *testing.T) {
	var receivedAuth string
	var receivedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	}))
	defer server.Close()

	provider, err := NewOAuth2ClientCredentialsProvider(
		server.URL,
		"test-client-id",
		"test-client-secret",
		[]string{"read", "write"},
		30*time.Second,
	)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()

	token, err := provider.Token(context.Background())
	if err != nil {
		t.Fatalf("Failed to get token: %v", err)
	}

	if token != "test-token" {
		t.Errorf("Expected token 'test-token', got '%s'", token)
	}

	// Verify Basic Auth header is present
	if !strings.HasPrefix(receivedAuth, "Basic ") {
		t.Errorf("Expected Basic Auth header, got: %s", receivedAuth)
	}

	// Decode and verify credentials
	encodedCreds := strings.TrimPrefix(receivedAuth, "Basic ")
	decodedCreds, err := base64.StdEncoding.DecodeString(encodedCreds)
	if err != nil {
		t.Fatalf("Failed to decode Basic Auth: %v", err)
	}

	expectedCreds := "test-client-id:test-client-secret"
	if string(decodedCreds) != expectedCreds {
		t.Errorf("Expected credentials '%s', got '%s'", expectedCreds, string(decodedCreds))
	}

	// Verify credentials are NOT in form body
	if strings.Contains(receivedBody, "client_id") {
		t.Error("client_id should not be in form body")
	}
	if strings.Contains(receivedBody, "client_secret") {
		t.Error("client_secret should not be in form body")
	}

	// Verify grant_type and scopes are in body
	if !strings.Contains(receivedBody, "grant_type=client_credentials") {
		t.Error("grant_type should be in form body")
	}
	if !strings.Contains(receivedBody, "scope=read+write") {
		t.Error("scopes should be in form body")
	}
}

// TestOAuth2ResourceOwner_UsesBasicAuth verifies that client credentials
// are passed via Basic Auth header for resource owner flow
func TestOAuth2ResourceOwner_UsesBasicAuth(t *testing.T) {
	var receivedAuth string
	var receivedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	}))
	defer server.Close()

	provider, err := NewOAuth2ResourceOwnerProvider(
		server.URL,
		"test-client-id",
		"test-client-secret",
		"test-user",
		"test-password",
		[]string{"read"},
		30*time.Second,
	)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()

	token, err := provider.Token(context.Background())
	if err != nil {
		t.Fatalf("Failed to get token: %v", err)
	}

	if token != "test-token" {
		t.Errorf("Expected token 'test-token', got '%s'", token)
	}

	// Verify Basic Auth header is present
	if !strings.HasPrefix(receivedAuth, "Basic ") {
		t.Errorf("Expected Basic Auth header, got: %s", receivedAuth)
	}

	// Verify client credentials are NOT in form body
	if strings.Contains(receivedBody, "client_id") {
		t.Error("client_id should not be in form body")
	}
	if strings.Contains(receivedBody, "client_secret") {
		t.Error("client_secret should not be in form body")
	}

	// Verify user credentials ARE in body
	if !strings.Contains(receivedBody, "username=test-user") {
		t.Error("username should be in form body")
	}
	if !strings.Contains(receivedBody, "password=test-password") {
		t.Error("password should be in form body")
	}
	if !strings.Contains(receivedBody, "grant_type=password") {
		t.Error("grant_type should be in form body")
	}
}

// TestOAuth2TokenCaching_Concurrency verifies token caching works correctly
// under concurrent access
func TestOAuth2TokenCaching_Concurrency(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	}))
	defer server.Close()

	provider, err := NewOAuth2ClientCredentialsProvider(
		server.URL,
		"test-client-id",
		"test-client-secret",
		nil,
		30*time.Second,
	)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()

	// Make 10 concurrent requests
	numRequests := 10
	tokens := make(chan string, numRequests)
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			token, err := provider.Token(context.Background())
			if err != nil {
				errors <- err
				return
			}
			tokens <- token
		}()
	}

	// Collect results
	for i := 0; i < numRequests; i++ {
		select {
		case err := <-errors:
			t.Fatalf("Unexpected error: %v", err)
		case token := <-tokens:
			if token != "test-token" {
				t.Errorf("Expected 'test-token', got '%s'", token)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for token")
		}
	}

	// Should only make 1 request due to caching
	if requestCount != 1 {
		t.Errorf("Expected 1 token request, got %d", requestCount)
	}
}

// TestOAuth2TokenRefresh verifies token refresh behavior
func TestOAuth2TokenRefresh(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Very short expiry to trigger refresh
		w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":1}`))
	}))
	defer server.Close()

	provider, err := NewOAuth2ClientCredentialsProvider(
		server.URL,
		"test-client-id",
		"test-client-secret",
		nil,
		500*time.Millisecond, // Refresh before 500ms of expiry
	)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()

	// First request
	token1, err := provider.Token(context.Background())
	if err != nil {
		t.Fatalf("Failed to get token: %v", err)
	}

	// Wait for token to expire
	time.Sleep(1 * time.Second)

	// Second request should trigger refresh
	token2, err := provider.Token(context.Background())
	if err != nil {
		t.Fatalf("Failed to get token after expiry: %v", err)
	}

	if token1 != "test-token" || token2 != "test-token" {
		t.Errorf("Unexpected token values: %s, %s", token1, token2)
	}

	// Should have made 2 requests (initial + refresh)
	if requestCount != 2 {
		t.Errorf("Expected 2 token requests, got %d", requestCount)
	}
}

// TestOAuth2ErrorHandling verifies proper error handling
func TestOAuth2ErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectedError  string
	}{
		{
			name:          "Non-200 status code",
			statusCode:    401,
			responseBody:  `{"error":"invalid_client"}`,
			expectedError: "token request failed with status 401",
		},
		{
			name:          "OAuth2 error response",
			statusCode:    200,
			responseBody:  `{"error":"invalid_scope","error_description":"Requested scope is invalid"}`,
			expectedError: "oauth2 error: invalid_scope - Requested scope is invalid",
		},
		{
			name:          "Missing access token",
			statusCode:    200,
			responseBody:  `{"token_type":"Bearer"}`,
			expectedError: "no access token in response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			provider, err := NewOAuth2ClientCredentialsProvider(
				server.URL,
				"test-client-id",
				"test-client-secret",
				nil,
				30*time.Second,
			)
			if err != nil {
				t.Fatalf("Failed to create provider: %v", err)
			}
			defer provider.Close()

			_, err = provider.Token(context.Background())
			if err == nil {
				t.Fatal("Expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.expectedError, err.Error())
			}
		})
	}
}
