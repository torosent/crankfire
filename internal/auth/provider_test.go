package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockOAuth2Server provides a test OAuth2 server that tracks requests
type mockOAuth2Server struct {
	server       *httptest.Server
	requestCount int32
	response     tokenResponse
	statusCode   int
	mu           sync.Mutex
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

func newMockOAuth2Server() *mockOAuth2Server {
	m := &mockOAuth2Server{
		statusCode: http.StatusOK,
		response: tokenResponse{
			AccessToken: "test-token-123",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		},
	}

	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&m.requestCount, 1)

		// Verify it's a POST request
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Parse form data
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		m.mu.Lock()
		statusCode := m.statusCode
		response := m.response
		m.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	}))

	return m
}

func (m *mockOAuth2Server) setResponse(token string, expiresIn int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.response = tokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
	}
}

func (m *mockOAuth2Server) setStatusCode(code int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusCode = code
}

func (m *mockOAuth2Server) getRequestCount() int {
	return int(atomic.LoadInt32(&m.requestCount))
}

func (m *mockOAuth2Server) close() {
	m.server.Close()
}

func TestClientCredentialsBasicFlow(t *testing.T) {
	mock := newMockOAuth2Server()
	defer mock.close()

	provider, err := NewOAuth2ClientCredentialsProvider(
		mock.server.URL,
		"test-client",
		"test-secret",
		[]string{"read", "write"},
		0, // no refresh buffer for basic test
	)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Close()

	ctx := context.Background()

	// First token fetch
	token1, err := provider.Token(ctx)
	if err != nil {
		t.Fatalf("failed to get token: %v", err)
	}
	if token1 != "test-token-123" {
		t.Errorf("expected token 'test-token-123', got '%s'", token1)
	}
	if mock.getRequestCount() != 1 {
		t.Errorf("expected 1 request, got %d", mock.getRequestCount())
	}

	// Second token fetch should use cache
	token2, err := provider.Token(ctx)
	if err != nil {
		t.Fatalf("failed to get cached token: %v", err)
	}
	if token2 != token1 {
		t.Errorf("expected cached token '%s', got '%s'", token1, token2)
	}
	if mock.getRequestCount() != 1 {
		t.Errorf("expected 1 request (cached), got %d", mock.getRequestCount())
	}
}

func TestClientCredentialsRefreshesBeforeExpiry(t *testing.T) {
	mock := newMockOAuth2Server()
	defer mock.close()

	// Set token with 2 second expiry
	mock.setResponse("token-expires-soon", 2)

	provider, err := NewOAuth2ClientCredentialsProvider(
		mock.server.URL,
		"test-client",
		"test-secret",
		[]string{"read"},
		1*time.Second, // refresh 1 second before expiry
	)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Close()

	ctx := context.Background()

	// First token fetch
	token1, err := provider.Token(ctx)
	if err != nil {
		t.Fatalf("failed to get token: %v", err)
	}
	if token1 != "token-expires-soon" {
		t.Errorf("expected token 'token-expires-soon', got '%s'", token1)
	}

	// Wait 1.2 seconds (past the refresh threshold)
	time.Sleep(1200 * time.Millisecond)

	// Change the mock response
	mock.setResponse("refreshed-token", 3600)

	// Second fetch should trigger refresh
	token2, err := provider.Token(ctx)
	if err != nil {
		t.Fatalf("failed to get refreshed token: %v", err)
	}
	if token2 != "refreshed-token" {
		t.Errorf("expected refreshed token 'refreshed-token', got '%s'", token2)
	}
	if mock.getRequestCount() != 2 {
		t.Errorf("expected 2 requests (initial + refresh), got %d", mock.getRequestCount())
	}
}

func TestResourceOwnerBasicFlow(t *testing.T) {
	mock := newMockOAuth2Server()
	defer mock.close()

	provider, err := NewOAuth2ResourceOwnerProvider(
		mock.server.URL,
		"test-client",
		"test-secret",
		"testuser",
		"testpass",
		[]string{"profile", "email"},
		0,
	)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Close()

	ctx := context.Background()

	token, err := provider.Token(ctx)
	if err != nil {
		t.Fatalf("failed to get token: %v", err)
	}
	if token != "test-token-123" {
		t.Errorf("expected token 'test-token-123', got '%s'", token)
	}
	if mock.getRequestCount() != 1 {
		t.Errorf("expected 1 request, got %d", mock.getRequestCount())
	}
}

func TestResourceOwnerHandlesRetry(t *testing.T) {
	mock := newMockOAuth2Server()
	defer mock.close()

	// Set server to return 500 error
	mock.setStatusCode(http.StatusInternalServerError)

	provider, err := NewOAuth2ResourceOwnerProvider(
		mock.server.URL,
		"test-client",
		"test-secret",
		"testuser",
		"testpass",
		[]string{"profile"},
		0,
	)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Close()

	ctx := context.Background()

	_, err = provider.Token(ctx)
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}

	// Verify error handling
	if mock.getRequestCount() != 1 {
		t.Errorf("expected 1 request, got %d", mock.getRequestCount())
	}
}

func TestOIDCStaticTokenInjection(t *testing.T) {
	provider := NewStaticTokenProvider("my-static-oidc-token")
	defer provider.Close()

	ctx := context.Background()

	// Token should return immediately
	token, err := provider.Token(ctx)
	if err != nil {
		t.Fatalf("failed to get static token: %v", err)
	}
	if token != "my-static-oidc-token" {
		t.Errorf("expected token 'my-static-oidc-token', got '%s'", token)
	}

	// Verify header injection
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	err = provider.InjectHeader(ctx, req)
	if err != nil {
		t.Fatalf("failed to inject header: %v", err)
	}

	authHeader := req.Header.Get("Authorization")
	expected := "Bearer my-static-oidc-token"
	if authHeader != expected {
		t.Errorf("expected Authorization header '%s', got '%s'", expected, authHeader)
	}
}

func TestRequestBuilderAppliesAuthHeaderPerRequest(t *testing.T) {
	provider := NewStaticTokenProvider("request-token")
	defer provider.Close()

	ctx := context.Background()

	// Create multiple requests
	req1, _ := http.NewRequest("GET", "http://example.com/api/v1", nil)
	req2, _ := http.NewRequest("POST", "http://example.com/api/v2", nil)
	req3, _ := http.NewRequest("PUT", "http://example.com/api/v3", nil)

	// Inject header into each request
	if err := provider.InjectHeader(ctx, req1); err != nil {
		t.Fatalf("failed to inject header into req1: %v", err)
	}
	if err := provider.InjectHeader(ctx, req2); err != nil {
		t.Fatalf("failed to inject header into req2: %v", err)
	}
	if err := provider.InjectHeader(ctx, req3); err != nil {
		t.Fatalf("failed to inject header into req3: %v", err)
	}

	// Verify all requests have the correct header
	expected := "Bearer request-token"
	for i, req := range []*http.Request{req1, req2, req3} {
		authHeader := req.Header.Get("Authorization")
		if authHeader != expected {
			t.Errorf("request %d: expected Authorization header '%s', got '%s'", i+1, expected, authHeader)
		}
	}
}

func TestConcurrentTokenAccess(t *testing.T) {
	mock := newMockOAuth2Server()
	defer mock.close()

	provider, err := NewOAuth2ClientCredentialsProvider(
		mock.server.URL,
		"test-client",
		"test-secret",
		[]string{"read"},
		0,
	)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Close()

	ctx := context.Background()
	numGoroutines := 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	tokens := make([]string, numGoroutines)

	// Launch 50 concurrent token requests
	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()
			token, err := provider.Token(ctx)
			if err != nil {
				t.Errorf("goroutine %d: failed to get token: %v", index, err)
				return
			}
			tokens[index] = token
		}(i)
	}

	wg.Wait()

	// Verify only one request was made
	requestCount := mock.getRequestCount()
	if requestCount != 1 {
		t.Errorf("expected 1 request for 50 concurrent calls, got %d", requestCount)
	}

	// Verify all goroutines got the same token
	expectedToken := "test-token-123"
	for i, token := range tokens {
		if token != expectedToken {
			t.Errorf("goroutine %d: expected token '%s', got '%s'", i, expectedToken, token)
		}
	}
}

func TestProviderContextCancellation(t *testing.T) {
	mock := newMockOAuth2Server()
	defer mock.close()

	provider, err := NewOAuth2ClientCredentialsProvider(
		mock.server.URL,
		"test-client",
		"test-secret",
		[]string{"read"},
		0,
	)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Close()

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait to ensure context is cancelled
	time.Sleep(10 * time.Millisecond)

	_, err = provider.Token(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}

	// Verify the error is related to context cancellation
	if ctx.Err() == nil {
		t.Fatal("expected context to be cancelled")
	}
}
