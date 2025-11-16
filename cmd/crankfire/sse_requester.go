package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/sse"
)

type sseRequester struct {
	cfg       *config.SSEConfig
	target    string
	headers   map[string]string
	collector *metrics.Collector
}

func newSSERequester(cfg *config.Config, collector *metrics.Collector) *sseRequester {
	return &sseRequester{
		cfg:       &cfg.SSE,
		target:    cfg.TargetURL,
		headers:   cfg.Headers,
		collector: collector,
	}
}

func (s *sseRequester) Do(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	start := time.Now()
	meta := &metrics.RequestMetadata{Protocol: "sse"}

	// Create SSE client config
	sseCfg := sse.Config{
		URL:     s.target,
		Headers: makeHeaders(s.headers),
		Timeout: s.cfg.ReadTimeout,
	}

	client := sse.NewClient(sseCfg)

	// Connect to SSE endpoint
	if err := client.Connect(ctx); err != nil {
		meta = annotateStatus(meta, "sse", sseStatusCode(err))
		s.collector.RecordRequest(time.Since(start), err, meta)
		return fmt.Errorf("sse connect: %w", err)
	}
	defer client.Close()

	// Read events until max events reached or timeout
	eventsRead := 0
	maxEvents := s.cfg.MaxEvents
	if maxEvents <= 0 {
		maxEvents = 100 // Default to prevent infinite reads
	}

	readCtx := ctx
	if s.cfg.ReadTimeout > 0 {
		var cancel context.CancelFunc
		readCtx, cancel = context.WithTimeout(ctx, s.cfg.ReadTimeout)
		defer cancel()
	}

	for eventsRead < maxEvents {
		_, err := client.ReadEvent(readCtx)
		if err != nil {
			// Check if it's a context error (expected timeout/cancellation)
			if readCtx.Err() != nil {
				break
			}
			// Other errors are failures
			meta = annotateStatus(meta, "sse", fallbackStatusCode(err))
			s.collector.RecordRequest(time.Since(start), err, meta)
			return fmt.Errorf("read event: %w", err)
		}
		eventsRead++

		if ctx.Err() != nil {
			break
		}
	}

	// Get metrics and record
	sseMetrics := client.Metrics()
	latency := time.Since(start)

	// Record as successful request with SSE-specific metadata
	meta.CustomMetrics = map[string]interface{}{
		"connection_duration_ms": sseMetrics.ConnectionDuration.Milliseconds(),
		"events_received":        sseMetrics.EventsReceived,
		"bytes_received":         sseMetrics.BytesReceived,
	}

	s.collector.RecordRequest(latency, nil, meta)
	return nil
}

func sseStatusCode(err error) string {
	if err == nil {
		return ""
	}
	if statusErr, ok := err.(*sse.StatusError); ok {
		return strconv.Itoa(statusErr.Code)
	}
	return fallbackStatusCode(err)
}
