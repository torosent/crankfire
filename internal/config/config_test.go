package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/config"
)

func TestParseFlagsDefaults(t *testing.T) {
	loader := config.NewLoader()

	cfg, err := loader.Load([]string{})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.TargetURL != "" {
		t.Errorf("TargetURL = %q, want empty", cfg.TargetURL)
	}
	if cfg.Method != "GET" {
		t.Errorf("Method = %q, want GET", cfg.Method)
	}
	if cfg.Concurrency != 1 {
		t.Errorf("Concurrency = %d, want 1", cfg.Concurrency)
	}
	if cfg.Rate != 0 {
		t.Errorf("Rate = %d, want 0", cfg.Rate)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %s, want 30s", cfg.Timeout)
	}
	if cfg.Retries != 0 {
		t.Errorf("Retries = %d, want 0", cfg.Retries)
	}
	if cfg.JSONOutput {
		t.Errorf("JSONOutput = true, want false")
	}
	if len(cfg.Headers) != 0 {
		t.Errorf("Headers len = %d, want 0", len(cfg.Headers))
	}
	if cfg.Arrival.Model != config.ArrivalModelUniform {
		t.Errorf("Arrival model = %q, want uniform", cfg.Arrival.Model)
	}
}

func TestMethodVariantsAndFallback(t *testing.T) {
	loader := config.NewLoader()

	t.Run("fallback to GET when method not provided", func(t *testing.T) {
		cfg, err := loader.Load([]string{"--target", "http://example.com"})
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.Method != "GET" {
			t.Fatalf("Method = %q, want GET", cfg.Method)
		}
	})

	t.Run("accepts common verbs including PATCH/PUT/DELETE", func(t *testing.T) {
		verbs := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
		for _, verb := range verbs {
			t.Run(verb, func(t *testing.T) {
				cfg, err := loader.Load([]string{"--target", "http://example.com", "--method", verb})
				if err != nil {
					t.Fatalf("Load() error = %v", err)
				}
				if cfg.Method != verb {
					t.Fatalf("Method = %q, want %q", cfg.Method, verb)
				}
			})
		}
	})
}

func TestLoadConfigFileJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{
		"target": "https://api.example.com",
		"method": "PUT",
		"headers": {"Content-Type": "application/json"},
		"body": "{\"foo\":\"bar\"}",
		"concurrency": 10,
		"rate": 100,
		"duration": "2m",
		"total": 500,
		"timeout": "45s",
		"retries": 3,
		"jsonOutput": true
	}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--config", path, "--method", "PATCH", "--header", "Authorization=Bearer token"})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.TargetURL != "https://api.example.com" {
		t.Errorf("TargetURL = %q, want https://api.example.com", cfg.TargetURL)
	}
	if cfg.Method != "PATCH" {
		t.Errorf("Method = %q, want PATCH", cfg.Method)
	}
	if cfg.Headers["Content-Type"] != "application/json" {
		t.Errorf("Headers[Content-Type] = %q, want application/json", cfg.Headers["Content-Type"])
	}
	if cfg.Headers["Authorization"] != "Bearer token" {
		t.Errorf("Headers[Authorization] = %q, want Bearer token", cfg.Headers["Authorization"])
	}
	if cfg.Body != `{"foo":"bar"}` {
		t.Errorf("Body = %q, want {\"foo\":\"bar\"}", cfg.Body)
	}
	if cfg.Concurrency != 10 {
		t.Errorf("Concurrency = %d, want 10", cfg.Concurrency)
	}
	if cfg.Rate != 100 {
		t.Errorf("Rate = %d, want 100", cfg.Rate)
	}
	if cfg.Duration != 2*time.Minute {
		t.Errorf("Duration = %s, want 2m", cfg.Duration)
	}
	if cfg.Total != 500 {
		t.Errorf("Total = %d, want 500", cfg.Total)
	}
	if cfg.Timeout != 45*time.Second {
		t.Errorf("Timeout = %s, want 45s", cfg.Timeout)
	}
	if cfg.Retries != 3 {
		t.Errorf("Retries = %d, want 3", cfg.Retries)
	}
	if !cfg.JSONOutput {
		t.Errorf("JSONOutput = false, want true")
	}
}

func TestLoadConfigFileYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := strings.Join([]string{
		"target: https://service.example.com",
		"method: POST",
		"headers:",
		"  X-Env: staging",
		"concurrency: 4",
		"rate: 20",
		"duration: 30s",
		"timeout: 15s",
		"total: 40",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--config", path})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.TargetURL != "https://service.example.com" {
		t.Errorf("TargetURL = %q, want https://service.example.com", cfg.TargetURL)
	}
	if cfg.Method != "POST" {
		t.Errorf("Method = %q, want POST", cfg.Method)
	}
	if cfg.Headers["X-Env"] != "staging" {
		t.Errorf("Headers[X-Env] = %q, want staging", cfg.Headers["X-Env"])
	}
	if cfg.Concurrency != 4 {
		t.Errorf("Concurrency = %d, want 4", cfg.Concurrency)
	}
	if cfg.Rate != 20 {
		t.Errorf("Rate = %d, want 20", cfg.Rate)
	}
	if cfg.Duration != 30*time.Second {
		t.Errorf("Duration = %s, want 30s", cfg.Duration)
	}
	if cfg.Timeout != 15*time.Second {
		t.Errorf("Timeout = %s, want 15s", cfg.Timeout)
	}
	if cfg.Total != 40 {
		t.Errorf("Total = %d, want 40", cfg.Total)
	}
}

func TestArrivalModelFlagOverride(t *testing.T) {
	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--target", "http://example.com", "--arrival-model", "poisson"})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Arrival.Model != config.ArrivalModelPoisson {
		t.Fatalf("Arrival model = %q, want poisson", cfg.Arrival.Model)
	}
}

func TestLoadPatternsAndArrivalFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "complex.yaml")
	content := strings.Join([]string{
		"target: https://api.example.com",
		"load_patterns:",
		"  - name: warmup",
		"    type: ramp",
		"    from_rps: 10",
		"    to_rps: 200",
		"    duration: 2m",
		"  - type: step",
		"    steps:",
		"      - rps: 100",
		"        duration: 1m",
		"      - rps: 200",
		"        duration: 30s",
		"  - name: spike",
		"    type: spike",
		"    rps: 500",
		"    duration: 15s",
		"arrival:",
		"  model: poisson",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--config", path})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.LoadPatterns) != 3 {
		t.Fatalf("LoadPatterns len = %d, want 3", len(cfg.LoadPatterns))
	}
	if cfg.LoadPatterns[0].Type != config.LoadPatternTypeRamp {
		t.Errorf("first pattern type = %q, want ramp", cfg.LoadPatterns[0].Type)
	}
	if cfg.LoadPatterns[1].Type != config.LoadPatternTypeStep || len(cfg.LoadPatterns[1].Steps) != 2 {
		t.Errorf("step pattern not parsed correctly: %+v", cfg.LoadPatterns[1])
	}
	if cfg.LoadPatterns[2].Type != config.LoadPatternTypeSpike || cfg.LoadPatterns[2].RPS != 500 {
		t.Errorf("spike pattern missing fields: %+v", cfg.LoadPatterns[2])
	}
	if cfg.Arrival.Model != config.ArrivalModelPoisson {
		t.Errorf("Arrival model = %q, want poisson", cfg.Arrival.Model)
	}
}

func TestEndpointsParsingFromJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "endpoints.json")
	json := `{
		"target": "https://api.example.com",
		"endpoints": [
			{"name":"list-users","weight":60,"path":"/users","method":"get"},
			{"name":"user-detail","weight":30,"url":"https://api.example.com/users/{id}","headers":{"x-trace-id":"abc"}},
			{"name":"create-order","weight":10,"method":"POST","body":"{\"foo\":\"bar\"}"}
		]
	}`
	if err := os.WriteFile(path, []byte(json), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--config", path})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Endpoints) != 3 {
		t.Fatalf("Endpoints len = %d, want 3", len(cfg.Endpoints))
	}
	if cfg.Endpoints[0].Method != "GET" {
		t.Errorf("first endpoint method = %q, want GET", cfg.Endpoints[0].Method)
	}
	if cfg.Endpoints[0].Path != "/users" {
		t.Errorf("first endpoint path = %q, want /users", cfg.Endpoints[0].Path)
	}
	if cfg.Endpoints[1].Headers["X-Trace-Id"] != "abc" {
		t.Errorf("headers not canonicalized: %+v", cfg.Endpoints[1].Headers)
	}
	if cfg.Endpoints[2].Weight != 10 {
		t.Errorf("weight = %d, want 10", cfg.Endpoints[2].Weight)
	}
}

func TestFlagBodyOverridesConfigBodyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"bodyFile":"payload.json"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--config", path, "--body", "inline"})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Body != "inline" {
		t.Errorf("Body = %q, want inline", cfg.Body)
	}
	if cfg.BodyFile != "" {
		t.Errorf("BodyFile = %q, want empty", cfg.BodyFile)
	}
}

func TestFlagBodyFileOverridesConfigBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"body":"inline-config"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--config", path, "--body-file", "payload.txt"})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.BodyFile != "payload.txt" {
		t.Errorf("BodyFile = %q, want payload.txt", cfg.BodyFile)
	}
	if cfg.Body != "" {
		t.Errorf("Body = %q, want empty", cfg.Body)
	}
}

func TestConfigValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		have config.Config
		want []string
	}{
		{
			name: "missing target",
			have: config.Config{},
			want: []string{"target"},
		},
		{
			name: "negative values",
			have: config.Config{
				TargetURL:   "https://example.com",
				Concurrency: -1,
				Rate:        -5,
				Total:       -10,
				Timeout:     -1,
				Retries:     -1,
			},
			want: []string{"concurrency", "rate", "total", "timeout", "retries"},
		},
		{
			name: "body conflict",
			have: config.Config{
				TargetURL: "https://example.com",
				Body:      "inline",
				BodyFile:  "payload.json",
			},
			want: []string{"body"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.have.Validate()
			if err == nil {
				t.Fatalf("Validate() error = nil, want error")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("Validate() error %q missing %q", err.Error(), want)
				}
			}
		})
	}
}

func TestConfigValidationAdvancedErrors(t *testing.T) {
	t.Run("invalid ramp pattern", func(t *testing.T) {
		cfg := config.Config{
			TargetURL: "https://api.example.com",
			LoadPatterns: []config.LoadPattern{
				{Type: config.LoadPatternTypeRamp, FromRPS: 10, ToRPS: 100},
			},
		}
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "duration") {
			t.Fatalf("expected duration error, got %v", err)
		}
	})

	t.Run("endpoint weight", func(t *testing.T) {
		cfg := config.Config{
			TargetURL: "https://api.example.com",
			Endpoints: []config.Endpoint{{Name: "bad", Weight: 0}},
		}
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "weight") {
			t.Fatalf("expected weight error, got %v", err)
		}
	})
}

// ---- Headers specific tests ----

func TestLoader_HeaderFlagParsing(t *testing.T) {
	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--target", "http://example.com", "--header", "Content-Type=application/json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.Headers["Content-Type"]; got != "application/json" {
		t.Fatalf("expected Content-Type=application/json, got %q", got)
	}
}

func TestLoader_MultipleHeaderFlags(t *testing.T) {
	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--target", "http://example.com", "--header", "Content-Type=application/json", "--header", "X-Trace-Id=abc123", "--header", "X-Trace-Id=overwritten"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Headers["Content-Type"] != "application/json" {
		t.Fatalf("unexpected Content-Type: %q", cfg.Headers["Content-Type"])
	}
	// last value wins
	if cfg.Headers["X-Trace-Id"] != "overwritten" {
		t.Fatalf("expected X-Trace-Id to be 'overwritten', got %q", cfg.Headers["X-Trace-Id"])
	}
}

func TestLoader_HeaderFlagInvalidFormat(t *testing.T) {
	loader := config.NewLoader()
	_, err := loader.Load([]string{"--target", "http://example.com", "--header", "MissingEquals"})
	if err == nil {
		t.Fatalf("expected error for invalid header format")
	}
}

func TestLoader_HeaderKeyCanonical(t *testing.T) {
	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--target", "http://example.com", "--header", "content-type=application/json", "--header", "x-custom-header=value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := cfg.Headers["content-type"]; ok {
		t.Fatalf("raw lowercase key should be canonicalized")
	}
	if cfg.Headers["Content-Type"] != "application/json" {
		t.Fatalf("expected canonical Content-Type, got %q", cfg.Headers["Content-Type"])
	}
	if cfg.Headers["X-Custom-Header"] != "value" {
		t.Fatalf("expected X-Custom-Header=value, got %q", cfg.Headers["X-Custom-Header"])
	}
}

func TestLoader_HeadersFromJSONConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")
	json := `{
		"target": "http://example.com",
		"headers": {"Authorization": "Bearer token123", "X-Env": "prod"}
	}`
	if err := os.WriteFile(path, []byte(json), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--config", path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Headers["Authorization"] != "Bearer token123" {
		t.Fatalf("expected Authorization header, got %q", cfg.Headers["Authorization"])
	}
	if cfg.Headers["X-Env"] != "prod" {
		t.Fatalf("expected X-Env=prod, got %q", cfg.Headers["X-Env"])
	}
}

func TestLoader_HeadersFromYAMLConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	yaml := `target: http://example.com
headers:
  Authorization: Bearer t456
  X-Env: staging
`
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--config", path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Headers["Authorization"] != "Bearer t456" {
		t.Fatalf("expected Authorization header, got %q", cfg.Headers["Authorization"])
	}
	if cfg.Headers["X-Env"] != "staging" {
		t.Fatalf("expected X-Env=staging, got %q", cfg.Headers["X-Env"])
	}
}

func TestLoader_HeaderFlagOverridesConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")
	json := `{"target":"http://example.com","headers":{"X-Env":"prod","X-Trace-Id":"initial"}}`
	if err := os.WriteFile(path, []byte(json), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--config", path, "--header", "X-Env=staging", "--header", "X-Trace-Id=override"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Headers["X-Env"] != "staging" {
		t.Fatalf("expected X-Env overridden to staging, got %q", cfg.Headers["X-Env"])
	}
	if cfg.Headers["X-Trace-Id"] != "override" {
		t.Fatalf("expected X-Trace-Id override, got %q", cfg.Headers["X-Trace-Id"])
	}
}

func TestLoader_HeadersWithSpecialCharsAndEmptyValue(t *testing.T) {
	loader := config.NewLoader()
	cfg, err := loader.Load([]string{"--target", "http://example.com", "--header", "X-Sig=ab:c:def==", "--header", "X-Empty="})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Headers["X-Sig"] != "ab:c:def==" {
		t.Fatalf("expected X-Sig value preserved, got %q", cfg.Headers["X-Sig"])
	}
	if cfg.Headers["X-Empty"] != "" {
		t.Fatalf("expected X-Empty to be empty string, got %q", cfg.Headers["X-Empty"])
	}
}
