package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/torosent/crankfire/internal/auth"
	"github.com/torosent/crankfire/internal/httpclient"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/placeholders"
	"github.com/torosent/crankfire/internal/tracing"
	"go.opentelemetry.io/otel/trace"
)

// makeHeaders converts a map[string]string to http.Header
func makeHeaders(headers map[string]string) http.Header {
	h := make(http.Header)
	for k, v := range headers {
		h.Set(k, v)
	}
	return h
}

// baseRequesterHelper provides shared functionality for all requester types.
type baseRequesterHelper struct {
	collector *metrics.Collector
	auth      auth.Provider
	feeder    httpclient.Feeder
	tracing   *tracing.Provider
}

// tracer returns the OTel tracer, or a no-op if tracing is not configured.
func (b *baseRequesterHelper) tracer() trace.Tracer {
	if b.tracing == nil {
		return trace.NewNoopTracerProvider().Tracer("crankfire")
	}
	return b.tracing.Tracer()
}

// shouldPropagate returns whether W3C trace headers should be injected.
func (b *baseRequesterHelper) shouldPropagate() bool {
	if b.tracing == nil {
		return false
	}
	return b.tracing.ShouldPropagate()
}

// prepareContext ensures context is not nil and returns it.
func (b *baseRequesterHelper) prepareContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// initRequest initializes common request state including context, timing, and metadata.
func (b *baseRequesterHelper) initRequest(ctx context.Context, protocol string) (context.Context, time.Time, *metrics.RequestMetadata) {
	ctx = b.prepareContext(ctx)
	start := time.Now()
	meta := &metrics.RequestMetadata{Protocol: protocol}
	return ctx, start, meta
}

// getFeederRecord retrieves the next record from the feeder if configured.
func (b *baseRequesterHelper) getFeederRecord(ctx context.Context) (map[string]string, error) {
	return nextFeederRecord(ctx, b.feeder)
}

// prepareHeaders applies placeholders to headers and injects auth if configured.
func (b *baseRequesterHelper) prepareHeaders(ctx context.Context, baseHeaders map[string]string, record map[string]string) (http.Header, error) {
	headerMap := placeholders.ApplyToMap(baseHeaders, record)
	headers := makeHeaders(headerMap)
	if err := ensureAuthHeader(ctx, b.auth, headers); err != nil {
		return nil, fmt.Errorf("auth header: %w", err)
	}
	return headers, nil
}

// recordError records an error to metrics with appropriate status code annotation.
func (b *baseRequesterHelper) recordError(start time.Time, meta *metrics.RequestMetadata, protocol, context string, err error) error {
	meta = annotateStatus(meta, protocol, fallbackStatusCode(err))
	b.collector.RecordRequest(time.Since(start), err, meta)
	return fmt.Errorf("%s: %w", context, err)
}

// recordSuccess records a successful request to metrics.
func (b *baseRequesterHelper) recordSuccess(start time.Time, meta *metrics.RequestMetadata) {
	b.collector.RecordRequest(time.Since(start), nil, meta)
}
