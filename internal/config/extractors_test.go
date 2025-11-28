package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torosent/crankfire/internal/config"
)

func TestConfig_WithExtractors(t *testing.T) {
	t.Run("parse extractors from YAML config", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		content := strings.Join([]string{
			"target: https://api.example.com",
			"endpoints:",
			"  - name: get-user",
			"    url: https://api.example.com/users/123",
			"    method: GET",
			"    weight: 100",
			"    extractors:",
			"      - jsonpath: $.user.id",
			"        var: user_id",
			"      - regex: '\"email\":\\s*\"([^\"]+)\"'",
			"        var: user_email",
			"      - jsonpath: $.user.status",
			"        var: user_status",
			"        on_error: true",
		}, "\n")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		loader := config.NewLoader()
		cfg, err := loader.Load([]string{"--config", path})
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if len(cfg.Endpoints) != 1 {
			t.Fatalf("Expected 1 endpoint, got %d", len(cfg.Endpoints))
		}

		ep := cfg.Endpoints[0]
		if len(ep.Extractors) != 3 {
			t.Fatalf("Expected 3 extractors, got %d", len(ep.Extractors))
		}

		// Check first extractor (JSONPath)
		if ep.Extractors[0].JSONPath != "$.user.id" {
			t.Errorf("Extractors[0].JSONPath = %q, want $.user.id", ep.Extractors[0].JSONPath)
		}
		if ep.Extractors[0].Variable != "user_id" {
			t.Errorf("Extractors[0].Variable = %q, want user_id", ep.Extractors[0].Variable)
		}
		if ep.Extractors[0].Regex != "" {
			t.Errorf("Extractors[0].Regex should be empty, got %q", ep.Extractors[0].Regex)
		}
		if ep.Extractors[0].OnError {
			t.Errorf("Extractors[0].OnError = true, want false")
		}

		// Check second extractor (Regex)
		if ep.Extractors[1].Regex != `"email":\s*"([^"]+)"` {
			t.Errorf("Extractors[1].Regex = %q, want email regex", ep.Extractors[1].Regex)
		}
		if ep.Extractors[1].Variable != "user_email" {
			t.Errorf("Extractors[1].Variable = %q, want user_email", ep.Extractors[1].Variable)
		}
		if ep.Extractors[1].JSONPath != "" {
			t.Errorf("Extractors[1].JSONPath should be empty, got %q", ep.Extractors[1].JSONPath)
		}

		// Check third extractor (with OnError flag)
		if !ep.Extractors[2].OnError {
			t.Errorf("Extractors[2].OnError = false, want true")
		}
		if ep.Extractors[2].JSONPath != "$.user.status" {
			t.Errorf("Extractors[2].JSONPath = %q, want $.user.status", ep.Extractors[2].JSONPath)
		}
		if ep.Extractors[2].Variable != "user_status" {
			t.Errorf("Extractors[2].Variable = %q, want user_status", ep.Extractors[2].Variable)
		}
	})

	t.Run("parse extractors from JSON config", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.json")
		content := `{
			"target": "https://api.example.com",
			"endpoints": [
				{
					"name": "search-products",
					"url": "https://api.example.com/products",
					"method": "GET",
					"weight": 50,
					"extractors": [
						{
							"jsonpath": "$.products[0].id",
							"var": "first_product_id"
						}
					]
				}
			]
		}`
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		loader := config.NewLoader()
		cfg, err := loader.Load([]string{"--config", path})
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if len(cfg.Endpoints) != 1 {
			t.Fatalf("Expected 1 endpoint, got %d", len(cfg.Endpoints))
		}

		ep := cfg.Endpoints[0]
		if len(ep.Extractors) != 1 {
			t.Fatalf("Expected 1 extractor, got %d", len(ep.Extractors))
		}

		if ep.Extractors[0].JSONPath != "$.products[0].id" {
			t.Errorf("JSONPath = %q, want $.products[0].id", ep.Extractors[0].JSONPath)
		}
		if ep.Extractors[0].Variable != "first_product_id" {
			t.Errorf("Variable = %q, want first_product_id", ep.Extractors[0].Variable)
		}
	})

	t.Run("endpoint with no extractors is valid", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		content := strings.Join([]string{
			"target: https://api.example.com",
			"endpoints:",
			"  - name: simple-get",
			"    url: https://api.example.com/status",
			"    method: GET",
			"    weight: 100",
		}, "\n")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		loader := config.NewLoader()
		cfg, err := loader.Load([]string{"--config", path})
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if len(cfg.Endpoints) != 1 {
			t.Fatalf("Expected 1 endpoint, got %d", len(cfg.Endpoints))
		}

		ep := cfg.Endpoints[0]
		if len(ep.Extractors) != 0 {
			t.Errorf("Expected no extractors, got %d", len(ep.Extractors))
		}
	})
}

func TestConfig_Validate_InvalidExtractor_NoSource(t *testing.T) {
	cfg := config.Config{
		TargetURL:   "https://api.example.com",
		Concurrency: 1,
		Endpoints: []config.Endpoint{
			{
				Name:   "bad-extractor",
				URL:    "https://api.example.com/users/1",
				Weight: 1,
				Extractors: []config.Extractor{
					{
						Variable: "user_id",
						// Missing both JSONPath and Regex
					},
				},
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for extractor with no source, got nil")
	}
	if !strings.Contains(err.Error(), "either jsonpath or regex") {
		t.Errorf("Validate() error = %q, want substring 'either jsonpath or regex'", err.Error())
	}
}

func TestConfig_Validate_InvalidExtractor_BothSources(t *testing.T) {
	cfg := config.Config{
		TargetURL:   "https://api.example.com",
		Concurrency: 1,
		Endpoints: []config.Endpoint{
			{
				Name:   "bad-extractor",
				URL:    "https://api.example.com/users/1",
				Weight: 1,
				Extractors: []config.Extractor{
					{
						JSONPath: "$.user.id",
						Regex:    "[0-9]+",
						Variable: "user_id",
						// Has both JSONPath and Regex
					},
				},
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for extractor with both sources, got nil")
	}
	if !strings.Contains(err.Error(), "either jsonpath or regex") {
		t.Errorf("Validate() error = %q, want substring 'either jsonpath or regex'", err.Error())
	}
}

func TestConfig_Validate_InvalidExtractor_NoVariable(t *testing.T) {
	cfg := config.Config{
		TargetURL:   "https://api.example.com",
		Concurrency: 1,
		Endpoints: []config.Endpoint{
			{
				Name:   "bad-extractor",
				URL:    "https://api.example.com/users/1",
				Weight: 1,
				Extractors: []config.Extractor{
					{
						JSONPath: "$.user.id",
						Variable: "", // Missing variable name
					},
				},
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for extractor with no variable, got nil")
	}
	if !strings.Contains(err.Error(), "var is required") {
		t.Errorf("Validate() error = %q, want substring 'var is required'", err.Error())
	}
}

func TestConfig_Validate_InvalidExtractor_InvalidVariableName(t *testing.T) {
	cases := []struct {
		name     string
		varName  string
		wantPass bool
	}{
		{
			name:     "valid simple name",
			varName:  "user_id",
			wantPass: true,
		},
		{
			name:     "valid with underscores",
			varName:  "_internal_id",
			wantPass: true,
		},
		{
			name:     "valid with numbers",
			varName:  "user_id_123",
			wantPass: true,
		},
		{
			name:     "valid starting with underscore",
			varName:  "_id",
			wantPass: true,
		},
		{
			name:     "invalid starting with number",
			varName:  "123_id",
			wantPass: false,
		},
		{
			name:     "invalid with hyphen",
			varName:  "user-id",
			wantPass: false,
		},
		{
			name:     "invalid with dot",
			varName:  "user.id",
			wantPass: false,
		},
		{
			name:     "invalid with space",
			varName:  "user id",
			wantPass: false,
		},
		{
			name:     "invalid empty",
			varName:  "",
			wantPass: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Config{
				TargetURL:   "https://api.example.com",
				Concurrency: 1,
				Endpoints: []config.Endpoint{
					{
						Name:   "test",
						URL:    "https://api.example.com/users/1",
						Weight: 1,
						Extractors: []config.Extractor{
							{
								JSONPath: "$.user.id",
								Variable: tc.varName,
							},
						},
					},
				},
			}

			err := cfg.Validate()
			if tc.wantPass {
				if err != nil {
					t.Errorf("Validate() expected no error for %q, got %v", tc.varName, err)
				}
			} else {
				if err == nil {
					t.Errorf("Validate() expected error for invalid variable name %q, got nil", tc.varName)
				} else {
					errMsg := err.Error()
					// Empty string should give "var is required", non-empty invalid should give "valid identifier"
					if tc.varName == "" {
						if !strings.Contains(errMsg, "var is required") {
							t.Errorf("Validate() error = %q, want substring 'var is required'", errMsg)
						}
					} else {
						if !strings.Contains(errMsg, "valid identifier") {
							t.Errorf("Validate() error = %q, want substring 'valid identifier'", errMsg)
						}
					}
				}
			}
		})
	}
}
