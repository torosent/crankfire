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
