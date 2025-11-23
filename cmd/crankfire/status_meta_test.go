package main

import (
	"errors"
	"testing"

	"github.com/torosent/crankfire/internal/metrics"
)

func TestEnsureProtocolMeta(t *testing.T) {
	t.Run("nil meta", func(t *testing.T) {
		got := ensureProtocolMeta(nil, "http")
		if got == nil {
			t.Fatal("ensureProtocolMeta(nil) returned nil")
		}
		if got.Protocol != "http" {
			t.Errorf("Protocol = %q, want http", got.Protocol)
		}
	})

	t.Run("existing meta", func(t *testing.T) {
		meta := &metrics.RequestMetadata{Protocol: "grpc"}
		got := ensureProtocolMeta(meta, "http")
		if got.Protocol != "grpc" {
			t.Errorf("Protocol = %q, want grpc", got.Protocol)
		}
	})

	t.Run("empty protocol in meta", func(t *testing.T) {
		meta := &metrics.RequestMetadata{}
		got := ensureProtocolMeta(meta, "http")
		if got.Protocol != "http" {
			t.Errorf("Protocol = %q, want http", got.Protocol)
		}
	})
}

func TestAnnotateStatus(t *testing.T) {
	meta := &metrics.RequestMetadata{}
	got := annotateStatus(meta, "http", "200 OK")
	if got.Protocol != "http" {
		t.Errorf("Protocol = %q, want http", got.Protocol)
	}
	if got.StatusCode != "200_OK" {
		t.Errorf("StatusCode = %q, want 200_OK", got.StatusCode)
	}
}

func TestSanitizeStatusCode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"200", "200"},
		{"200 OK", "200_OK"},
		{"  error  ", "ERROR"},
		{"invalid/char", "INVALID_CHAR"},
		{"", "UNKNOWN"},
		{"   ", "UNKNOWN"},
		{"-._", "UNKNOWN"}, // All replaced to _ then trimmed
	}

	for _, tt := range tests {
		got := sanitizeStatusCode(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeStatusCode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFallbackStatusCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"simple error", errors.New("oops"), "ERRORSTRING"}, // errors.New returns *errors.errorString
		{"custom error", &MyError{}, "MYERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fallbackStatusCode(tt.err)
			if got != tt.want {
				t.Errorf("fallbackStatusCode() = %q, want %q", got, tt.want)
			}
		})
	}
}

type MyError struct{}

func (e *MyError) Error() string { return "my error" }
