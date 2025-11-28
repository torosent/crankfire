package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/torosent/crankfire/internal/auth"
	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/httpclient"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/placeholders"
	"github.com/torosent/crankfire/internal/pool"
	"github.com/torosent/crankfire/internal/sse"
)

type sseRequester struct {
	cfg       *config.SSEConfig
	target    string
	headers   map[string]string
	collector *metrics.Collector
	auth      auth.Provider
	feeder    httpclient.Feeder
	connPool  *pool.ConnectionPool
	helper    baseRequesterHelper
}

func newSSERequester(cfg *config.Config, collector *metrics.Collector, provider auth.Provider, feeder httpclient.Feeder) *sseRequester {
	return &sseRequester{
		cfg:       &cfg.SSE,
		target:    cfg.TargetURL,
		headers:   cfg.Headers,
		collector: collector,
		auth:      provider,
		feeder:    feeder,
		connPool:  pool.NewConnectionPool(cfg.Concurrency),
		helper: baseRequesterHelper{
			collector: collector,
			auth:      provider,
			feeder:    feeder,
		},
	}
}

func (s *sseRequester) Do(ctx context.Context) error {
	ctx, start, meta := s.helper.initRequest(ctx, "sse")

	record, err := s.helper.getFeederRecord(ctx)
	if err != nil {
		return s.helper.recordError(start, meta, "sse", "feeder", err)
	}

	target := s.target
	if len(record) > 0 {
		target = placeholders.Apply(target, record)
	}

	requestHeaders, err := s.helper.prepareHeaders(ctx, s.headers, record)
	if err != nil {
		return s.helper.recordError(start, meta, "sse", "auth", err)
	}

	// Get or create client from pool
	poolKey := pool.MakePoolKey(target, requestHeaders)

	factory := func() pool.Poolable {
		sseCfg := sse.Config{
			URL:     target,
			Headers: requestHeaders,
			Timeout: s.cfg.ReadTimeout,
		}
		return sse.NewClient(sseCfg)
	}

	poolable, reused := s.connPool.Get(poolKey, factory)
	client := poolable.(*sse.Client)

	// Connect if not reused
	if !reused {
		if err := client.Connect(ctx); err != nil {
			return s.helper.recordError(start, meta, "sse", "connect", err)
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
				newPoolable, ok := s.connPool.RetryStaleConnection(ctx, client, factory)
				if !ok {
					opErr = fmt.Errorf("reconnect failed")
					break
				}
				client = newPoolable.(*sse.Client)
				reused = false
				continue
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
	s.connPool.Put(poolKey, client)

	s.collector.RecordRequest(latency, nil, meta)
	return nil
}

// Close releases all SSE connections held in the connection pool.
func (s *sseRequester) Close() error {
	return s.connPool.Close()
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
