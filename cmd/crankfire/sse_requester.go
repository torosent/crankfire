package main

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/torosent/crankfire/internal/auth"
	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/httpclient"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/sse"
)

type sseRequester struct {
	cfg         *config.SSEConfig
	target      string
	headers     map[string]string
	collector   *metrics.Collector
	auth        auth.Provider
	feeder      httpclient.Feeder
	concurrency int
	pools       sync.Map // map[string]chan *sse.Client
}

func newSSERequester(cfg *config.Config, collector *metrics.Collector, provider auth.Provider, feeder httpclient.Feeder) *sseRequester {
	return &sseRequester{
		cfg:         &cfg.SSE,
		target:      cfg.TargetURL,
		headers:     cfg.Headers,
		collector:   collector,
		auth:        provider,
		feeder:      feeder,
		concurrency: cfg.Concurrency,
	}
}

func (s *sseRequester) Do(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	start := time.Now()
	meta := &metrics.RequestMetadata{Protocol: "sse"}

	record, err := nextFeederRecord(ctx, s.feeder)
	if err != nil {
		meta := annotateStatus(meta, "sse", fallbackStatusCode(err))
		s.collector.RecordRequest(time.Since(start), err, meta)
		return fmt.Errorf("sse feeder: %w", err)
	}

	target := s.target
	if len(record) > 0 {
		target = applyPlaceholders(target, record)
	}

	headerMap := applyPlaceholdersToMap(s.headers, record)
	requestHeaders := makeHeaders(headerMap)
	if err := ensureAuthHeader(ctx, s.auth, requestHeaders); err != nil {
		meta := annotateStatus(meta, "sse", fallbackStatusCode(err))
		s.collector.RecordRequest(time.Since(start), err, meta)
		return fmt.Errorf("sse auth header: %w", err)
	}

	// Get or create pool for this target+headers combination
	poolKey := makePoolKey(target, requestHeaders)
	poolVal, _ := s.pools.LoadOrStore(poolKey, make(chan *sse.Client, s.concurrency))
	pool := poolVal.(chan *sse.Client)

	var client *sse.Client
	var reused bool

	// Try to get an existing connection from the pool
	select {
	case client = <-pool:
		reused = true
	default:
		// Create new client if pool is empty
		sseCfg := sse.Config{
			URL:     target,
			Headers: requestHeaders,
			Timeout: s.cfg.ReadTimeout,
		}
		client = sse.NewClient(sseCfg)
	}

	// Connect if not reused
	if !reused {
		if err := client.Connect(ctx); err != nil {
			meta = annotateStatus(meta, "sse", sseStatusCode(err))
			s.collector.RecordRequest(time.Since(start), err, meta)
			return fmt.Errorf("sse connect: %w", err)
		}
	}

	// Capture metrics before operation to calculate delta
	startMetrics := client.Metrics()

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

	var opErr error

	for eventsRead < maxEvents {
		_, err := client.ReadEvent(readCtx)
		if err != nil {
			// If this is a reused connection and we failed on the first read,
			// it's likely a stale connection. Try to reconnect once.
			if reused && eventsRead == 0 {
				client.Close()
				if connErr := client.Connect(ctx); connErr == nil {
					reused = false
					// Update baseline metrics if needed, though cumulative counters persist.
					// We just continue the loop to retry the read.
					continue
				} else {
					opErr = fmt.Errorf("reconnect failed: %w", connErr)
					break
				}
			}

			// Check if it's a context error (expected timeout/cancellation)
			if readCtx.Err() != nil {
				break
			}
			// Other errors are failures
			opErr = fmt.Errorf("read event: %w", err)
			break
		}
		eventsRead++

		if ctx.Err() != nil {
			opErr = ctx.Err()
			break
		}
	}

	// Calculate delta metrics
	endMetrics := client.Metrics()
	latency := time.Since(start)

	// Record as successful request with SSE-specific metadata
	meta.CustomMetrics = map[string]interface{}{
		"connection_duration_ms": endMetrics.ConnectionDuration.Milliseconds(),
		"events_received":        endMetrics.EventsReceived - startMetrics.EventsReceived,
		"bytes_received":         endMetrics.BytesReceived - startMetrics.BytesReceived,
	}

	if opErr != nil {
		// If error occurred, close client and do not return to pool
		client.Close()
		meta = annotateStatus(meta, "sse", fallbackStatusCode(opErr))
		s.collector.RecordRequest(latency, opErr, meta)
		return opErr
	}

	// If successful, return client to pool
	select {
	case pool <- client:
		// Returned to pool
	default:
		// Pool full (shouldn't happen if sized correctly, but safe to close)
		client.Close()
	}

	s.collector.RecordRequest(latency, nil, meta)
	return nil
}

// Close releases all SSE connections held in the pools.
func (s *sseRequester) Close() error {
	s.pools.Range(func(key, value interface{}) bool {
		if pool, ok := value.(chan *sse.Client); ok {
			close(pool)
			for client := range pool {
				client.Close()
			}
		}
		return true
	})
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
