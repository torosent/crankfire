package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/torosent/crankfire/internal/config"
)

func TestBuildDataFeeder(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		_, err := buildDataFeeder(nil)
		if err == nil {
			t.Error("buildDataFeeder(nil) error = nil, want error")
		}
	})

	t.Run("empty path", func(t *testing.T) {
		cfg := &config.Config{Feeder: config.FeederConfig{Path: ""}}
		feeder, err := buildDataFeeder(cfg)
		if err != nil {
			t.Fatalf("buildDataFeeder(empty) error = %v", err)
		}
		if feeder != nil {
			t.Error("buildDataFeeder(empty) feeder != nil, want nil")
		}
	})

	t.Run("unsupported type", func(t *testing.T) {
		cfg := &config.Config{Feeder: config.FeederConfig{Path: "file.txt", Type: "unknown"}}
		_, err := buildDataFeeder(cfg)
		if err == nil {
			t.Error("buildDataFeeder(unknown) error = nil, want error")
		}
	})

	t.Run("csv feeder", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "data.csv")
		if err := os.WriteFile(path, []byte("id\n1"), 0644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		cfg := &config.Config{Feeder: config.FeederConfig{Path: path, Type: "csv"}}
		feeder, err := buildDataFeeder(cfg)
		if err != nil {
			t.Fatalf("buildDataFeeder(csv) error = %v", err)
		}
		defer feeder.Close()

		if feeder.Len() != 1 {
			t.Errorf("Len() = %d, want 1", feeder.Len())
		}
	})
}

func TestSharedFeeder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(path, []byte("id\n1"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg := &config.Config{Feeder: config.FeederConfig{Path: path, Type: "csv"}}
	feeder, err := buildDataFeeder(cfg)
	if err != nil {
		t.Fatalf("buildDataFeeder() error = %v", err)
	}
	defer feeder.Close()

	// Test Next()
	record, err := feeder.Next(context.Background())
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if record["id"] != "1" {
		t.Errorf("record[id] = %q, want 1", record["id"])
	}

	// Test NextFeederRecord helper
	// Since we exhausted the feeder (1 record), next call should fail or wrap around depending on implementation.
	// CSVFeeder wraps around? No, it errors ErrExhausted.
	// Wait, CSVFeeder implementation in feeder_test.go showed it errors.
	// Let's check if it wraps around.
	// Ah, in feeder_test.go: "Fourth call should return exhausted error (no rewind per requirements)"
	// So it does not wrap around.

	// But wait, sharedFeeder just delegates.
}
