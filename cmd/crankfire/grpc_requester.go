package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/grpcclient"
	"github.com/torosent/crankfire/internal/metrics"
)

type grpcRequester struct {
	cfg       *config.GRPCConfig
	target    string
	collector *metrics.Collector
}

func newGRPCRequester(cfg *config.Config, collector *metrics.Collector) *grpcRequester {
	return &grpcRequester{
		cfg:       &cfg.GRPC,
		target:    cfg.TargetURL,
		collector: collector,
	}
}

func (g *grpcRequester) Do(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	start := time.Now()

	// Create gRPC client config
	grpcCfg := grpcclient.Config{
		Target:   g.target,
		Service:  g.cfg.Service,
		Method:   g.cfg.Method,
		Metadata: g.cfg.Metadata,
		Timeout:  g.cfg.Timeout,
		UseTLS:   g.cfg.TLS,
		Insecure: g.cfg.Insecure,
	}

	client, err := grpcclient.NewClient(grpcCfg)
	if err != nil {
		g.collector.RecordRequest(time.Since(start), err, nil)
		return fmt.Errorf("create grpc client: %w", err)
	}

	// Connect to gRPC server
	connectCtx := ctx
	if g.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		connectCtx, cancel = context.WithTimeout(ctx, g.cfg.Timeout)
		defer cancel()
	}

	if err := client.Connect(connectCtx); err != nil {
		g.collector.RecordRequest(time.Since(start), err, nil)
		return fmt.Errorf("grpc connect: %w", err)
	}
	defer client.Close()

	// Parse message payload from JSON
	var req interface{}
	if g.cfg.Message != "" {
		var msgData map[string]interface{}
		if err := json.Unmarshal([]byte(g.cfg.Message), &msgData); err != nil {
			g.collector.RecordRequest(time.Since(start), err, nil)
			return fmt.Errorf("parse message: %w", err)
		}
		req = msgData
	} else {
		// Empty message
		req = map[string]interface{}{}
	}

	// Make the RPC call
	callCtx := ctx
	if g.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, g.cfg.Timeout)
		defer cancel()
	}

	_, err = client.Invoke(callCtx, req)
	latency := time.Since(start)

	if err != nil {
		g.collector.RecordRequest(latency, err, nil)
		return fmt.Errorf("grpc invoke: %w", err)
	}

	// Get metrics and record
	grpcMetrics := client.Metrics()

	// Record as successful request with gRPC-specific metadata
	meta := &metrics.RequestMetadata{
		Protocol: "grpc",
		CustomMetrics: map[string]interface{}{
			"messages_sent":     grpcMetrics.MessagesSent,
			"messages_received": grpcMetrics.MessagesRecv,
			"bytes_sent":        grpcMetrics.BytesSent,
			"bytes_received":    grpcMetrics.BytesRecv,
			"status_code":       grpcMetrics.StatusCode,
		},
	}

	g.collector.RecordRequest(latency, nil, meta)
	return nil
}
