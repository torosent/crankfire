package auth

import (
	"context"
	"net/http/httptest"
	"testing"
)

func TestStaticTokenProvider(t *testing.T) {
	token := "my-static-token"
	provider := NewStaticTokenProvider(token)

	// Test Token()
	gotToken, err := provider.Token(context.Background())
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if gotToken != token {
		t.Errorf("Token() = %q, want %q", gotToken, token)
	}

	// Test InjectHeader()
	req := httptest.NewRequest("GET", "http://example.com", nil)
	if err := provider.InjectHeader(context.Background(), req); err != nil {
		t.Fatalf("InjectHeader() error = %v", err)
	}

	gotHeader := req.Header.Get("Authorization")
	wantHeader := "Bearer " + token
	if gotHeader != wantHeader {
		t.Errorf("Authorization header = %q, want %q", gotHeader, wantHeader)
	}

	// Test Close()
	if err := provider.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}
