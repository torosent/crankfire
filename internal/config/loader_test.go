package config

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

func TestAsString(t *testing.T) {
	tests := []struct {
		input interface{}
		want  string
	}{
		{"hello", "hello"},
		{123, "123"},
		{true, "true"},
		{nil, ""},
		{[]byte("bytes"), "bytes"},
	}

	for _, tt := range tests {
		got, err := asString(tt.input)
		if err != nil {
			t.Errorf("asString(%v) error = %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("asString(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestAsInt(t *testing.T) {
	tests := []struct {
		input interface{}
		want  int
	}{
		{123, 123},
		{"456", 456},
		{int64(789), 789},
		{float64(10.0), 10},
		{nil, 0},
	}

	for _, tt := range tests {
		got, err := asInt(tt.input)
		if err != nil {
			t.Errorf("asInt(%v) error = %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("asInt(%v) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestAsBool(t *testing.T) {
	tests := []struct {
		input interface{}
		want  bool
	}{
		{true, true},
		{"true", true},
		{"1", true},
		{false, false},
		{"false", false},
		{"0", false},
		{nil, false},
	}

	for _, tt := range tests {
		got, err := asBool(tt.input)
		if err != nil {
			t.Errorf("asBool(%v) error = %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("asBool(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestAsDuration(t *testing.T) {
	tests := []struct {
		input interface{}
		want  time.Duration
	}{
		{time.Second, time.Second},
		{"1m", time.Minute},
		{10, 10 * time.Second}, // int treated as seconds
		{nil, 0},
	}

	for _, tt := range tests {
		got, err := asDuration(tt.input)
		if err != nil {
			t.Errorf("asDuration(%v) error = %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("asDuration(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestApplyConfigSettings(t *testing.T) {
	cfg := &Config{}
	settings := map[string]interface{}{
		"target":      "http://example.com",
		"method":      "POST",
		"concurrency": 10,
		"timeout":     "5s",
		"headers": map[string]interface{}{
			"Content-Type": "application/json",
		},
		"auth": map[string]interface{}{
			"type":     "basic",
			"username": "user",
			"password": "pass",
		},
	}

	if err := applyConfigSettings(cfg, settings); err != nil {
		t.Fatalf("applyConfigSettings() error = %v", err)
	}

	if cfg.TargetURL != "http://example.com" {
		t.Errorf("TargetURL = %q, want http://example.com", cfg.TargetURL)
	}
	if cfg.Method != "POST" {
		t.Errorf("Method = %q, want POST", cfg.Method)
	}
	if cfg.Concurrency != 10 {
		t.Errorf("Concurrency = %d, want 10", cfg.Concurrency)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", cfg.Timeout)
	}
	if cfg.Headers["Content-Type"] != "application/json" {
		t.Errorf("Headers[Content-Type] = %q, want application/json", cfg.Headers["Content-Type"])
	}
	if cfg.Auth.Type != "basic" {
		t.Errorf("Auth.Type = %q, want basic", cfg.Auth.Type)
	}
	if cfg.Auth.Username != "user" {
		t.Errorf("Auth.Username = %q, want user", cfg.Auth.Username)
	}
}

func TestApplyFlagOverrides(t *testing.T) {
	cfg := &Config{
		Concurrency: 1,
		Method:      "GET",
	}

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	configureFlags(fs)

	// Simulate parsing flags
	args := []string{
		"--concurrency=5",
		"--method=PUT",
		"--header=X-Test=123",
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if err := applyFlagOverrides(cfg, fs); err != nil {
		t.Fatalf("applyFlagOverrides() error = %v", err)
	}

	if cfg.Concurrency != 5 {
		t.Errorf("Concurrency = %d, want 5", cfg.Concurrency)
	}
	if cfg.Method != "PUT" {
		t.Errorf("Method = %q, want PUT", cfg.Method)
	}
	if cfg.Headers["X-Test"] != "123" {
		t.Errorf("Headers[X-Test] = %q, want 123", cfg.Headers["X-Test"])
	}
}

func TestLoader_Load(t *testing.T) {
	loader := NewLoader()
	args := []string{
		"--target=http://example.com",
		"--concurrency=2",
	}

	cfg, err := loader.Load(args)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.TargetURL != "http://example.com" {
		t.Errorf("TargetURL = %q, want http://example.com", cfg.TargetURL)
	}
	if cfg.Concurrency != 2 {
		t.Errorf("Concurrency = %d, want 2", cfg.Concurrency)
	}
}

func TestParseLoadPatterns(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{
			"name":     "ramp-up",
			"type":     "ramp",
			"from_rps": 10,
			"to_rps":   100,
			"duration": "1m",
		},
	}

	patterns, err := parseLoadPatterns(input)
	if err != nil {
		t.Fatalf("parseLoadPatterns() error = %v", err)
	}

	if len(patterns) != 1 {
		t.Fatalf("len(patterns) = %d, want 1", len(patterns))
	}

	p := patterns[0]
	if p.Name != "ramp-up" {
		t.Errorf("Name = %q, want ramp-up", p.Name)
	}
	if p.Type != "ramp" {
		t.Errorf("Type = %q, want ramp", p.Type)
	}
	if p.FromRPS != 10 {
		t.Errorf("FromRPS = %d, want 10", p.FromRPS)
	}
	if p.ToRPS != 100 {
		t.Errorf("ToRPS = %d, want 100", p.ToRPS)
	}
	if p.Duration != time.Minute {
		t.Errorf("Duration = %v, want 1m", p.Duration)
	}
}

func TestParseEndpoints(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{
			"name":   "home",
			"url":    "http://example.com",
			"weight": 5,
		},
	}

	endpoints, err := parseEndpoints(input)
	if err != nil {
		t.Fatalf("parseEndpoints() error = %v", err)
	}

	if len(endpoints) != 1 {
		t.Fatalf("len(endpoints) = %d, want 1", len(endpoints))
	}

	e := endpoints[0]
	if e.Name != "home" {
		t.Errorf("Name = %q, want home", e.Name)
	}
	if e.URL != "http://example.com" {
		t.Errorf("URL = %q, want http://example.com", e.URL)
	}
	if e.Weight != 5 {
		t.Errorf("Weight = %d, want 5", e.Weight)
	}
}

func TestLoad_WithHARFile(t *testing.T) {
	// Create a test HAR file
	harData := `{
		"log": {
			"version": "1.2",
			"creator": {"name": "test", "version": "1.0"},
			"entries": [
				{
					"request": {
						"method": "GET",
						"url": "http://example.com/api/users",
						"headers": [
							{"name": "Content-Type", "value": "application/json"}
						]
					},
					"response": {"status": 200, "headers": [], "content": {}}
				}
			]
		}
	}`

	tmpFile, err := os.CreateTemp("", "test-*.har")
	if err != nil {
		t.Fatalf("Failed to create temp HAR file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(harData); err != nil {
		t.Fatalf("Failed to write HAR data: %v", err)
	}
	tmpFile.Close()

	loader := NewLoader()
	args := []string{
		"--target=http://example.com",
		"--har=" + tmpFile.Name(),
	}

	cfg, err := loader.Load(args)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// The loader should store the HAR file path
	if cfg.HARFile != tmpFile.Name() {
		t.Errorf("HARFile = %q, want %q", cfg.HARFile, tmpFile.Name())
	}

	// The actual conversion happens in the cmd layer, not in loader
	// This test just verifies the file path is stored correctly
}

func TestLoad_HARWithFilter(t *testing.T) {
	harData := `{
		"log": {
			"version": "1.2",
			"creator": {"name": "test", "version": "1.0"},
			"entries": [
				{
					"request": {
						"method": "GET",
						"url": "http://api.example.com/users",
						"headers": []
					},
					"response": {"status": 200, "headers": [], "content": {}}
				},
				{
					"request": {
						"method": "POST",
						"url": "http://api.example.com/create",
						"headers": []
					},
					"response": {"status": 201, "headers": [], "content": {}}
				},
				{
					"request": {
						"method": "GET",
						"url": "http://other.com/data",
						"headers": []
					},
					"response": {"status": 200, "headers": [], "content": {}}
				}
			]
		}
	}`

	tmpFile, err := os.CreateTemp("", "test-*.har")
	if err != nil {
		t.Fatalf("Failed to create temp HAR file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(harData); err != nil {
		t.Fatalf("Failed to write HAR data: %v", err)
	}
	tmpFile.Close()

	loader := NewLoader()
	args := []string{
		"--target=http://example.com",
		"--har=" + tmpFile.Name(),
		"--har-filter=host:api.example.com;method:GET",
	}

	cfg, err := loader.Load(args)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// The loader should store both HAR file and filter
	if cfg.HARFile != tmpFile.Name() {
		t.Errorf("HARFile = %q, want %q", cfg.HARFile, tmpFile.Name())
	}
	if cfg.HARFilter != "host:api.example.com;method:GET" {
		t.Errorf("HARFilter = %q, want host:api.example.com;method:GET", cfg.HARFilter)
	}
}

func TestLoad_HARMergeWithConfig(t *testing.T) {
	harData := `{
		"log": {
			"version": "1.2",
			"creator": {"name": "test", "version": "1.0"},
			"entries": [
				{
					"request": {
						"method": "GET",
						"url": "http://example.com/api",
						"headers": []
					},
					"response": {"status": 200, "headers": [], "content": {}}
				}
			]
		}
	}`

	tmpFile, err := os.CreateTemp("", "test-*.har")
	if err != nil {
		t.Fatalf("Failed to create temp HAR file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(harData); err != nil {
		t.Fatalf("Failed to write HAR data: %v", err)
	}
	tmpFile.Close()

	loader := NewLoader()
	args := []string{
		"--target=http://example.com",
		"--har=" + tmpFile.Name(),
	}

	cfg, err := loader.Load(args)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// The loader should store the HAR file path
	// The actual merging with endpoints happens in the cmd layer
	if cfg.HARFile != tmpFile.Name() {
		t.Errorf("HARFile = %q, want %q", cfg.HARFile, tmpFile.Name())
	}
}

func TestLoad_HARFileNotFound(t *testing.T) {
	loader := NewLoader()
	args := []string{
		"--target=http://example.com",
		"--har=/nonexistent/path/file.har",
	}

	_, err := loader.Load(args)
	if err == nil {
		t.Errorf("Load() expected error for missing HAR file, got nil")
	}

	if !strings.Contains(err.Error(), "failed to open") && !strings.Contains(err.Error(), "no such file") {
		t.Errorf("Load() error = %v, want error about missing file", err)
	}
}

func TestLoad_HARInvalidFile(t *testing.T) {
	// Create invalid HAR file (not JSON)
	tmpFile, err := os.CreateTemp("", "test-*.har")
	if err != nil {
		t.Fatalf("Failed to create temp HAR file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString("invalid json content"); err != nil {
		t.Fatalf("Failed to write HAR data: %v", err)
	}
	tmpFile.Close()

	loader := NewLoader()
	args := []string{
		"--target=http://example.com",
		"--har=" + tmpFile.Name(),
	}

	_, err = loader.Load(args)
	if err == nil {
		t.Errorf("Load() expected error for invalid HAR file, got nil")
	}

	if !strings.Contains(err.Error(), "parse") && !strings.Contains(err.Error(), "JSON") {
		t.Errorf("Load() error = %v, want parse/JSON error", err)
	}
}

func TestParseHARFilter(t *testing.T) {
	tests := []struct {
		name     string
		filter   string
		wantHost []string
		wantMeth []string
	}{
		{
			name:     "empty filter",
			filter:   "",
			wantHost: nil,
			wantMeth: nil,
		},
		{
			name:     "single host",
			filter:   "host:example.com",
			wantHost: []string{"example.com"},
			wantMeth: nil,
		},
		{
			name:     "multiple hosts",
			filter:   "host:api.example.com,cdn.example.com",
			wantHost: []string{"api.example.com", "cdn.example.com"},
			wantMeth: nil,
		},
		{
			name:     "single method",
			filter:   "method:GET",
			wantHost: nil,
			wantMeth: []string{"GET"},
		},
		{
			name:     "multiple methods",
			filter:   "method:GET,POST",
			wantHost: nil,
			wantMeth: []string{"GET", "POST"},
		},
		{
			name:     "combined filters",
			filter:   "host:example.com;method:GET,POST",
			wantHost: []string{"example.com"},
			wantMeth: []string{"GET", "POST"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := parseHARFilter(tt.filter)
			if !sliceEqual(opts["hosts"], tt.wantHost) {
				t.Errorf("hosts = %v, want %v", opts["hosts"], tt.wantHost)
			}
			if !sliceEqual(opts["methods"], tt.wantMeth) {
				t.Errorf("methods = %v, want %v", opts["methods"], tt.wantMeth)
			}
		})
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
