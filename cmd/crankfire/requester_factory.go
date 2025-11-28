package main

import (
	"context"
	"fmt"

	"github.com/torosent/crankfire/internal/auth"
	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/httpclient"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/runner"
)

// feederAdapter adapts any interface with the Next/Close/Len methods to httpclient.Feeder
type feederAdapter interface {
	Next(context.Context) (map[string]string, error)
	Close() error
	Len() int
}

// NewRequesterFromConfig creates a runner.Requester based on the configuration protocol.
// This function is exported for use in integration tests.
func NewRequesterFromConfig(cfg *config.Config, collector *metrics.Collector, provider auth.Provider, feeder feederAdapter) (runner.Requester, error) {
	// Determine protocol from config
	protocol := cfg.Protocol
	if protocol == "" {
		// Default to HTTP if not specified
		protocol = "http"
	}

	switch protocol {
	case "http", "https":
		return newHTTPRequesterWithFeeder(cfg, collector, provider, feeder)
	case "grpc":
		return newGRPCRequester(cfg, collector, provider, feeder), nil
	case "websocket", "ws", "wss":
		return newWebSocketRequester(cfg, collector, provider, feeder), nil
	case "sse":
		return newSSERequester(cfg, collector, provider, feeder), nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}
}

// newHTTPRequesterWithFeeder creates an HTTP requester with optional auth and feeder support.
func newHTTPRequesterWithFeeder(cfg *config.Config, collector *metrics.Collector, provider auth.Provider, feeder feederAdapter) (*httpRequester, error) {
	builder, err := newHTTPRequestBuilder(cfg, provider, feeder)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request builder: %w", err)
	}

	client := httpclient.NewClient(cfg.Timeout)

	return &httpRequester{
		client:    client,
		builder:   builder,
		collector: collector,
		helper: baseRequesterHelper{
			collector: collector,
			auth:      provider,
			feeder:    feeder,
		},
	}, nil
}
