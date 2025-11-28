package main

import (
	"path/filepath"
	"testing"

	"github.com/torosent/crankfire/internal/config"
)

// TestSampleConfigs_CanParse verifies all sample config files in scripts/ can be parsed
func TestSampleConfigs_CanParse(t *testing.T) {
	buildDir := filepath.Join("..", "..", "scripts")

	samples := []string{
		"sample.yml",
		"test-endpoints.yml",
		"auth-oauth2-sample.yml",
		"auth-oidc-sample.yml",
		"feeder-csv-sample.yml",
		"feeder-json-sample.json",
		"websocket-sample.yml",
		"sse-sample.json",
		"grpc-sample.yml",
		"chaining-sample.yml",
	}

	for _, sample := range samples {
		t.Run(sample, func(t *testing.T) {
			path := filepath.Join(buildDir, sample)

			loader := config.NewLoader()
			_, err := loader.Load([]string{"--config", path})
			if err != nil && err != config.ErrHelpRequested {
				t.Fatalf("Failed to parse %s: %v", sample, err)
			}

			// Just verify it parses successfully
			t.Logf("%s parsed successfully", sample)
		})
	}
}

// TestSampleConfigs_AuthFeatureCoverage verifies auth sample files have expected configuration
func TestSampleConfigs_AuthFeatureCoverage(t *testing.T) {
	buildDir := filepath.Join("..", "..", "scripts")

	testCases := []struct {
		name        string
		file        string
		authType    config.AuthType
		hasTokenURL bool
		hasScopes   bool
	}{
		{
			name:        "OAuth2ClientCredentials",
			file:        "auth-oauth2-sample.yml",
			authType:    config.AuthTypeOAuth2ClientCredentials,
			hasTokenURL: true,
			hasScopes:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(buildDir, tc.file)

			loader := config.NewLoader()
			cfg, err := loader.Load([]string{"--config", path})
			if err != nil {
				t.Fatalf("Failed to parse %s: %v", tc.file, err)
			}

			if cfg.Auth.Type == "" {
				t.Fatalf("%s missing auth configuration", tc.file)
			}

			if cfg.Auth.Type != tc.authType {
				t.Errorf("%s auth type = %s, want %s", tc.file, cfg.Auth.Type, tc.authType)
			}

			if tc.hasTokenURL && cfg.Auth.TokenURL == "" {
				t.Errorf("%s missing token_url", tc.file)
			}

			if tc.hasScopes && len(cfg.Auth.Scopes) == 0 {
				t.Errorf("%s missing scopes", tc.file)
			}
		})
	}
}

// TestSampleConfigs_FeederFeatureCoverage verifies feeder sample files have expected configuration
func TestSampleConfigs_FeederFeatureCoverage(t *testing.T) {
	buildDir := filepath.Join("..", "..", "scripts")

	testCases := []struct {
		name       string
		file       string
		feederType string
		hasPath    bool
	}{
		{
			name:       "CSVFeeder",
			file:       "feeder-csv-sample.yml",
			feederType: "csv",
			hasPath:    true,
		},
		{
			name:       "JSONFeeder",
			file:       "feeder-json-sample.json",
			feederType: "json",
			hasPath:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(buildDir, tc.file)

			loader := config.NewLoader()
			cfg, err := loader.Load([]string{"--config", path})
			if err != nil {
				t.Fatalf("Failed to parse %s: %v", tc.file, err)
			}

			if cfg.Feeder.Type == "" {
				t.Fatalf("%s missing feeder configuration", tc.file)
			}

			if cfg.Feeder.Type != tc.feederType {
				t.Errorf("%s feeder type = %s, want %s", tc.file, cfg.Feeder.Type, tc.feederType)
			}

			if tc.hasPath && cfg.Feeder.Path == "" {
				t.Errorf("%s missing path", tc.file)
			}
		})
	}
}

// TestSampleConfigs_ProtocolFeatureCoverage verifies protocol sample files have expected configuration
func TestSampleConfigs_ProtocolFeatureCoverage(t *testing.T) {
	buildDir := filepath.Join("..", "..", "scripts")

	testCases := []struct {
		name     string
		file     string
		protocol config.Protocol
	}{
		{
			name:     "WebSocket",
			file:     "websocket-sample.yml",
			protocol: config.ProtocolWebSocket,
		},
		{
			name:     "SSE",
			file:     "sse-sample.json",
			protocol: config.ProtocolSSE,
		},
		{
			name:     "gRPC",
			file:     "grpc-sample.yml",
			protocol: config.ProtocolGRPC,
		},
		{
			name:     "HTTP",
			file:     "sample.yml",
			protocol: config.ProtocolHTTP,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(buildDir, tc.file)

			loader := config.NewLoader()
			cfg, err := loader.Load([]string{"--config", path})
			if err != nil {
				t.Fatalf("Failed to parse %s: %v", tc.file, err)
			}

			// Verify protocol configuration (top-level or from TargetURL)
			protocol := cfg.Protocol
			if protocol == "" {
				protocol = config.ProtocolHTTP // Default
			}

			if protocol != tc.protocol {
				t.Errorf("%s protocol = %s, want %s", tc.file, protocol, tc.protocol)
			}
		})
	}
}

// TestSampleConfigs_RequiredFields verifies sample configs with endpoints have essential fields
func TestSampleConfigs_RequiredFields(t *testing.T) {
	buildDir := filepath.Join("..", "..", "scripts")

	// Only test samples that are expected to have endpoints
	samples := []string{
		"sample.yml",
		"test-endpoints.yml",
	}

	for _, sample := range samples {
		t.Run(sample, func(t *testing.T) {
			path := filepath.Join(buildDir, sample)

			loader := config.NewLoader()
			cfg, err := loader.Load([]string{"--config", path})
			if err != nil {
				t.Fatalf("Failed to parse %s: %v", sample, err)
			}

			// Check endpoints
			if len(cfg.Endpoints) == 0 {
				t.Errorf("%s: missing endpoints", sample)
			}

			// Check each endpoint has required fields
			for i, endpoint := range cfg.Endpoints {
				if endpoint.Name == "" {
					t.Errorf("%s: endpoint[%d] missing name", sample, i)
				}

				// Endpoints should have URL or Path
				if endpoint.URL == "" && endpoint.Path == "" {
					t.Errorf("%s: endpoint[%d] (%s) missing URL or Path", sample, i, endpoint.Name)
				}
			}

			// Check duration (if specified) is valid
			if cfg.Duration > 0 {
				t.Logf("%s: duration = %s", sample, cfg.Duration)
			}
		})
	}
}
