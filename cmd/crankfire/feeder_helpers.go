package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/torosent/crankfire/internal/config"
	feederpkg "github.com/torosent/crankfire/internal/feeder"
	"github.com/torosent/crankfire/internal/httpclient"
)

type sharedFeeder struct {
	inner feederpkg.Feeder
}

func buildDataFeeder(cfg *config.Config) (httpclient.Feeder, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	path := strings.TrimSpace(cfg.Feeder.Path)
	if path == "" {
		return nil, nil
	}

	feederType := strings.ToLower(strings.TrimSpace(cfg.Feeder.Type))
	var (
		inner feederpkg.Feeder
		err   error
	)
	switch feederType {
	case "csv":
		inner, err = feederpkg.NewCSVFeeder(path)
	case "json":
		inner, err = feederpkg.NewJSONFeeder(path)
	default:
		return nil, fmt.Errorf("unsupported feeder type %q", cfg.Feeder.Type)
	}
	if err != nil {
		return nil, err
	}

	return &sharedFeeder{inner: inner}, nil
}

func (s *sharedFeeder) Next(ctx context.Context) (map[string]string, error) {
	if s == nil || s.inner == nil {
		return nil, fmt.Errorf("feeder not configured")
	}
	record, err := s.inner.Next(ctx)
	if err != nil {
		return nil, err
	}
	// Copy to avoid exposing internal map references
	out := make(map[string]string, len(record))
	for k, v := range record {
		out[k] = v
	}
	return out, nil
}

func (s *sharedFeeder) Close() error {
	if s == nil || s.inner == nil {
		return nil
	}
	return s.inner.Close()
}

func (s *sharedFeeder) Len() int {
	if s == nil || s.inner == nil {
		return 0
	}
	return s.inner.Len()
}

func nextFeederRecord(ctx context.Context, fd httpclient.Feeder) (map[string]string, error) {
	if fd == nil {
		return nil, nil
	}
	return fd.Next(ctx)
}
