package httpclient

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/config"
)

func TestBuildRequestWithHeaders(t *testing.T) {
	cfg := &config.Config{
		Method:    "post",
		TargetURL: "http://example.com/api",
		Headers: map[string]string{
			"content-type": "application/json",
			"X-Trace-Id":   "12345",
		},
		Body: `{"hello":"world"}`,
	}

	builder, err := NewRequestBuilder(cfg)
	if err != nil {
		t.Fatalf("expected builder, got error: %v", err)
	}

	req, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("expected request, got error: %v", err)
	}

	if req.Method != http.MethodPost {
		t.Fatalf("expected method POST, got %s", req.Method)
	}

	if req.URL.String() != cfg.TargetURL {
		t.Fatalf("expected URL %s, got %s", cfg.TargetURL, req.URL.String())
	}

	if req.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("expected canonical Content-Type header, got %q", req.Header.Get("Content-Type"))
	}

	if req.Header.Get("X-Trace-Id") != "12345" {
		t.Fatalf("expected X-Trace-Id header, got %q", req.Header.Get("X-Trace-Id"))
	}

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body failed: %v", err)
	}
	if err := req.Body.Close(); err != nil {
		t.Fatalf("close body failed: %v", err)
	}

	expectedBody := cfg.Body
	if string(bodyBytes) != expectedBody {
		t.Fatalf("expected body %q, got %q", expectedBody, string(bodyBytes))
	}

	if req.ContentLength != int64(len(expectedBody)) {
		t.Fatalf("expected content length %d, got %d", len(expectedBody), req.ContentLength)
	}

	if req.GetBody == nil {
		t.Fatalf("expected request to support body replay")
	}

	replayBody, err := req.GetBody()
	if err != nil {
		t.Fatalf("expected replay body, got error: %v", err)
	}
	replayBytes, err := io.ReadAll(replayBody)
	if err != nil {
		t.Fatalf("read replay body failed: %v", err)
	}
	if err := replayBody.Close(); err != nil {
		t.Fatalf("close replay body failed: %v", err)
	}

	if string(replayBytes) != expectedBody {
		t.Fatalf("expected replay body %q, got %q", expectedBody, string(replayBytes))
	}
}

func TestRequestBuilder_InvalidHeaderKey(t *testing.T) {
	cfg := &config.Config{
		Method:    "GET",
		TargetURL: "http://example.com",
		Headers: map[string]string{
			"": "value",
		},
	}
	_, err := NewRequestBuilder(cfg)
	if err == nil {
		t.Fatalf("expected error for empty header key")
	}
}

func TestRequestBuilder_InvalidHeaderKeyWithNewline(t *testing.T) {
	cfg := &config.Config{
		Method:    "GET",
		TargetURL: "http://example.com",
		Headers: map[string]string{
			"Bad\nKey": "value",
		},
	}
	_, err := NewRequestBuilder(cfg)
	if err == nil {
		t.Fatalf("expected error for header key containing newline")
	}
}

func TestRequestBuilder_MethodFallbackAndVerbs(t *testing.T) {
	t.Run("fallback to GET when method empty", func(t *testing.T) {
		cfg := &config.Config{TargetURL: "http://example.com"}
		builder, err := NewRequestBuilder(cfg)
		if err != nil {
			t.Fatalf("NewRequestBuilder error = %v", err)
		}
		req, err := builder.Build(context.Background())
		if err != nil {
			t.Fatalf("Build error = %v", err)
		}
		if req.Method != http.MethodGet {
			t.Fatalf("expected method GET, got %s", req.Method)
		}
	})

	t.Run("supports common verbs including PATCH/PUT/DELETE", func(t *testing.T) {
		verbs := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
		for _, verb := range verbs {
			t.Run(verb, func(t *testing.T) {
				cfg := &config.Config{Method: verb, TargetURL: "http://example.com"}
				builder, err := NewRequestBuilder(cfg)
				if err != nil {
					t.Fatalf("NewRequestBuilder error = %v", err)
				}
				req, err := builder.Build(context.Background())
				if err != nil {
					t.Fatalf("Build error = %v", err)
				}
				if req.Method != verb {
					t.Fatalf("expected method %s, got %s", verb, req.Method)
				}
			})
		}
	})

	t.Run("normalizes lower-case method to upper-case", func(t *testing.T) {
		cfg := &config.Config{Method: "patch", TargetURL: "http://example.com"}
		builder, err := NewRequestBuilder(cfg)
		if err != nil {
			t.Fatalf("NewRequestBuilder error = %v", err)
		}
		req, err := builder.Build(context.Background())
		if err != nil {
			t.Fatalf("Build error = %v", err)
		}
		if req.Method != http.MethodPatch {
			t.Fatalf("expected method PATCH, got %s", req.Method)
		}
	})
}
func TestRequestBuilder_InvalidHeaderValueWithNewline(t *testing.T) {
	cfg := &config.Config{
		Method:    "GET",
		TargetURL: "http://example.com",
		Headers: map[string]string{
			"X-Test": "bad\rvalue",
		},
	}
	_, err := NewRequestBuilder(cfg)
	if err == nil {
		t.Fatalf("expected error for header value containing CR/LF")
	}
}

func TestRequestBuilder_HeadersWithLongValues(t *testing.T) {
	long := make([]byte, 2048)
	for i := range long {
		long[i] = 'a'
	}
	cfg := &config.Config{
		Method:    "GET",
		TargetURL: "http://example.com",
		Headers: map[string]string{
			"X-Long": string(long),
		},
	}
	b, err := NewRequestBuilder(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req, err := b.Build(context.Background())
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if got := req.Header.Get("X-Long"); got != string(long) {
		t.Fatalf("long header value mismatch")
	}
}

func TestRequestBuilder_EmptyHeaderValueAllowed(t *testing.T) {
	cfg := &config.Config{
		Method:    "GET",
		TargetURL: "http://example.com",
		Headers: map[string]string{
			"X-Empty": "",
		},
	}
	b, err := NewRequestBuilder(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req, err := b.Build(context.Background())
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if got := req.Header.Get("X-Empty"); got != "" {
		t.Fatalf("expected empty header value, got %q", got)
	}
}

func TestBodySourceFromFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "body.txt")
	content := "file body payload"

	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	cfg := &config.Config{BodyFile: filePath}
	source, err := NewBodySource(cfg)
	if err != nil {
		t.Fatalf("expected body source, got error: %v", err)
	}

	for i := 0; i < 2; i++ {
		reader, err := source.NewReader()
		if err != nil {
			t.Fatalf("expected reader #%d, got error: %v", i+1, err)
		}

		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("read body #%d failed: %v", i+1, err)
		}
		if err := reader.Close(); err != nil {
			t.Fatalf("close body #%d failed: %v", i+1, err)
		}

		if string(data) != content {
			t.Fatalf("expected body %q, got %q on iteration %d", content, string(data), i+1)
		}
	}
}

func TestClientTimeoutApplied(t *testing.T) {
	timeout := 50 * time.Millisecond
	client := NewClient(timeout)
	defer client.CloseIdleConnections()

	if client.Timeout != timeout {
		t.Fatalf("expected client timeout %s, got %s", timeout, client.Timeout)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(timeout * 3)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	start := time.Now()
	resp, err := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}

	elapsed := time.Since(start)
	if elapsed < timeout {
		t.Fatalf("request returned too quickly: %s < %s", elapsed, timeout)
	}
	if elapsed > timeout*5 {
		t.Fatalf("request took too long: %s", elapsed)
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		var netErr net.Error
		if !errors.As(err, &netErr) || !netErr.Timeout() {
			t.Fatalf("expected timeout error, got %v", err)
		}
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", client.Transport)
	}
	if transport.MaxIdleConns == 0 {
		t.Fatalf("expected transport to allow idle connections")
	}
	if transport.IdleConnTimeout == 0 {
		t.Fatalf("expected transport to set idle connection timeout")
	}
}

func TestRequestBuilderWithAuthProvider(t *testing.T) {
	mockProvider := &mockAuthProvider{token: "test-auth-token"}

	cfg := &config.Config{
		TargetURL: "https://api.example.com/data",
		Method:    "GET",
	}

	builder, err := NewRequestBuilderWithAuth(cfg, mockProvider)
	if err != nil {
		t.Fatalf("NewRequestBuilderWithAuth() error = %v", err)
	}

	ctx := context.Background()
	req, err := builder.Build(ctx)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	authHeader := req.Header.Get("Authorization")
	expected := "Bearer test-auth-token"
	if authHeader != expected {
		t.Errorf("Authorization header = %q, want %q", authHeader, expected)
	}

	// Verify provider was called
	if mockProvider.tokenCalls != 1 {
		t.Errorf("Token() calls = %d, want 1", mockProvider.tokenCalls)
	}
}

func TestRequestBuilderWithAuthProviderMultipleRequests(t *testing.T) {
	mockProvider := &mockAuthProvider{token: "multi-request-token"}

	cfg := &config.Config{
		TargetURL: "https://api.example.com/users",
		Method:    "POST",
		Body:      `{"name":"test"}`,
	}

	builder, err := NewRequestBuilderWithAuth(cfg, mockProvider)
	if err != nil {
		t.Fatalf("NewRequestBuilderWithAuth() error = %v", err)
	}

	ctx := context.Background()

	// Build multiple requests
	for i := 0; i < 3; i++ {
		req, err := builder.Build(ctx)
		if err != nil {
			t.Fatalf("Build() request %d error = %v", i, err)
		}

		authHeader := req.Header.Get("Authorization")
		expected := "Bearer multi-request-token"
		if authHeader != expected {
			t.Errorf("Request %d Authorization header = %q, want %q", i, authHeader, expected)
		}
	}

	// Verify provider was called for each request
	if mockProvider.tokenCalls != 3 {
		t.Errorf("Token() calls = %d, want 3", mockProvider.tokenCalls)
	}
}

// mockAuthProvider simulates an auth provider for testing
type mockAuthProvider struct {
	token      string
	tokenCalls int
	tokenErr   error
}

func (m *mockAuthProvider) Token(ctx context.Context) (string, error) {
	m.tokenCalls++
	if m.tokenErr != nil {
		return "", m.tokenErr
	}
	return m.token, nil
}

func (m *mockAuthProvider) InjectHeader(ctx context.Context, req *http.Request) error {
	token, err := m.Token(ctx)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

func (m *mockAuthProvider) Close() error {
	return nil
}

// mockFeeder simulates a data feeder
type mockFeeder struct {
	records []map[string]string
	index   int
}

func (m *mockFeeder) Next(ctx context.Context) (map[string]string, error) {
	if m.index >= len(m.records) {
		return nil, errors.New("feeder exhausted")
	}
	record := m.records[m.index]
	m.index++
	return record, nil
}

func (m *mockFeeder) Close() error {
	return nil
}

func (m *mockFeeder) Len() int {
	return len(m.records)
}

func TestRequestBuilderWithFeeder(t *testing.T) {
	feeder := &mockFeeder{
		records: []map[string]string{
			{"id": "123", "name": "alice"},
			{"id": "456", "name": "bob"},
		},
	}

	cfg := &config.Config{
		TargetURL: "http://example.com/users/{{id}}",
		Method:    "POST",
		Headers: map[string]string{
			"X-User": "{{name}}",
		},
		Body: `{"user_id": "{{id}}"}`,
	}

	builder, err := NewRequestBuilderWithFeeder(cfg, feeder)
	if err != nil {
		t.Fatalf("NewRequestBuilderWithFeeder() error = %v", err)
	}

	// First request
	req1, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() 1 error = %v", err)
	}

	if req1.URL.String() != "http://example.com/users/123" {
		t.Errorf("URL 1 = %q, want http://example.com/users/123", req1.URL.String())
	}
	if req1.Header.Get("X-User") != "alice" {
		t.Errorf("Header 1 = %q, want alice", req1.Header.Get("X-User"))
	}
	body1, _ := io.ReadAll(req1.Body)
	if string(body1) != `{"user_id": "123"}` {
		t.Errorf("Body 1 = %q, want %q", string(body1), `{"user_id": "123"}`)
	}

	// Second request
	req2, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() 2 error = %v", err)
	}

	if req2.URL.String() != "http://example.com/users/456" {
		t.Errorf("URL 2 = %q, want http://example.com/users/456", req2.URL.String())
	}
	if req2.Header.Get("X-User") != "bob" {
		t.Errorf("Header 2 = %q, want bob", req2.Header.Get("X-User"))
	}
	body2, _ := io.ReadAll(req2.Body)
	if string(body2) != `{"user_id": "456"}` {
		t.Errorf("Body 2 = %q, want %q", string(body2), `{"user_id": "456"}`)
	}

	// Third request (exhausted)
	_, err = builder.Build(context.Background())
	if err == nil {
		t.Error("Build() 3 error = nil, want error")
	}
}

func TestRequestBuilderWithAuthAndFeeder(t *testing.T) {
	feeder := &mockFeeder{
		records: []map[string]string{
			{"id": "1"},
		},
	}
	authProvider := &mockAuthProvider{token: "auth-token"}

	cfg := &config.Config{
		TargetURL: "http://example.com/{{id}}",
	}

	builder, err := NewRequestBuilderWithAuthAndFeeder(cfg, authProvider, feeder)
	if err != nil {
		t.Fatalf("NewRequestBuilderWithAuthAndFeeder() error = %v", err)
	}

	req, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if req.URL.String() != "http://example.com/1" {
		t.Errorf("URL = %q, want http://example.com/1", req.URL.String())
	}
	if req.Header.Get("Authorization") != "Bearer auth-token" {
		t.Errorf("Authorization = %q, want Bearer auth-token", req.Header.Get("Authorization"))
	}
}

func TestSubstitutePlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		template string
		record   map[string]string
		want     string
	}{
		{
			name:     "simple substitution",
			template: "hello {{name}}",
			record:   map[string]string{"name": "world"},
			want:     "hello world",
		},
		{
			name:     "multiple substitutions",
			template: "{{greeting}} {{name}}",
			record:   map[string]string{"greeting": "hi", "name": "there"},
			want:     "hi there",
		},
		{
			name:     "missing key",
			template: "hello {{name}}",
			record:   map[string]string{"other": "value"},
			want:     "hello {{name}}",
		},
		{
			name:     "empty template",
			template: "",
			record:   map[string]string{"key": "value"},
			want:     "",
		},
		{
			name:     "no placeholders",
			template: "static text",
			record:   map[string]string{"key": "value"},
			want:     "static text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substitutePlaceholders(tt.template, tt.record)
			if got != tt.want {
				t.Errorf("substitutePlaceholders(%q) = %q, want %q", tt.template, got, tt.want)
			}
		})
	}
}
