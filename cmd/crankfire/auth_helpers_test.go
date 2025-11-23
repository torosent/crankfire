package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/torosent/crankfire/internal/auth"
	"github.com/torosent/crankfire/internal/config"
)

func TestBuildAuthProvider(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		_, err := buildAuthProvider(nil)
		if err == nil {
			t.Error("buildAuthProvider(nil) error = nil, want error")
		}
	})

	t.Run("empty auth type", func(t *testing.T) {
		cfg := &config.Config{Auth: config.AuthConfig{Type: ""}}
		provider, err := buildAuthProvider(cfg)
		if err != nil {
			t.Fatalf("buildAuthProvider(empty) error = %v", err)
		}
		if provider != nil {
			t.Error("buildAuthProvider(empty) provider != nil, want nil")
		}
	})

	t.Run("unsupported auth type", func(t *testing.T) {
		cfg := &config.Config{Auth: config.AuthConfig{Type: "unknown"}}
		_, err := buildAuthProvider(cfg)
		if err == nil {
			t.Error("buildAuthProvider(unknown) error = nil, want error")
		}
	})

	t.Run("static token", func(t *testing.T) {
		cfg := &config.Config{
			Auth: config.AuthConfig{
				Type:        config.AuthTypeOIDCImplicit,
				StaticToken: "my-token",
			},
		}
		provider, err := buildAuthProvider(cfg)
		if err != nil {
			t.Fatalf("buildAuthProvider(static) error = %v", err)
		}
		if provider == nil {
			t.Fatal("buildAuthProvider(static) provider = nil")
		}
		defer provider.Close()

		token, err := provider.Token(context.Background())
		if err != nil {
			t.Fatalf("Token() error = %v", err)
		}
		if token != "my-token" {
			t.Errorf("Token() = %q, want my-token", token)
		}
	})

	t.Run("static token missing", func(t *testing.T) {
		cfg := &config.Config{
			Auth: config.AuthConfig{
				Type: config.AuthTypeOIDCImplicit,
			},
		}
		_, err := buildAuthProvider(cfg)
		if err == nil {
			t.Error("buildAuthProvider(missing static) error = nil, want error")
		}
	})
}

func TestEnsureAuthHeader(t *testing.T) {
	t.Run("nil provider", func(t *testing.T) {
		headers := make(http.Header)
		if err := ensureAuthHeader(context.Background(), nil, headers); err != nil {
			t.Fatalf("ensureAuthHeader(nil) error = %v", err)
		}
		if headers.Get("Authorization") != "" {
			t.Error("Authorization header set when provider is nil")
		}
	})

	t.Run("nil headers", func(t *testing.T) {
		provider := auth.NewStaticTokenProvider("token")
		if err := ensureAuthHeader(context.Background(), provider, nil); err == nil {
			t.Error("ensureAuthHeader(nil headers) error = nil, want error")
		}
	})

	t.Run("valid provider", func(t *testing.T) {
		provider := auth.NewStaticTokenProvider("my-token")
		headers := make(http.Header)
		if err := ensureAuthHeader(context.Background(), provider, headers); err != nil {
			t.Fatalf("ensureAuthHeader() error = %v", err)
		}
		want := "Bearer my-token"
		if got := headers.Get("Authorization"); got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
	})
}
