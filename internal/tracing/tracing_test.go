package tracing_test

import (
	"context"
	"net/http"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/tracing"
)

func setupTestTracer(t *testing.T) (*tracetest.InMemoryExporter, trace.Tracer) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
	})
	return exporter, tp.Tracer("test")
}

func TestInitDisabledByDefault(t *testing.T) {
	p, err := tracing.Init(context.Background(), config.TracingConfig{})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	if p.ShouldPropagate() {
		t.Error("ShouldPropagate() = true, want false when tracing disabled")
	}

	// Tracer should return a no-op (no panic)
	tracer := p.Tracer()
	ctx, span := tracer.Start(context.Background(), "test")
	span.End()
	if !span.SpanContext().TraceID().IsValid() == false {
		// no-op tracer returns invalid IDs, which is fine
		_ = ctx
	}
}

func TestInitWithEndpointEnablesTracing(t *testing.T) {
	// We can't actually connect to an endpoint in unit tests,
	// but we verify the provider is configured correctly.
	p, err := tracing.Init(context.Background(), config.TracingConfig{
		Endpoint:    "localhost:4317",
		Protocol:    "grpc",
		ServiceName: "test-service",
		SampleRate:  1.0,
		Insecure:    true,
	})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	if !p.ShouldPropagate() {
		t.Error("ShouldPropagate() = false, want true when tracing enabled")
	}
}

func TestInitHTTPProtocol(t *testing.T) {
	p, err := tracing.Init(context.Background(), config.TracingConfig{
		Endpoint: "localhost:4318",
		Protocol: "http",
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("Init() with http protocol error = %v", err)
	}
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	if !p.ShouldPropagate() {
		t.Error("ShouldPropagate() = false, want true")
	}
}

func TestInitUnsupportedProtocol(t *testing.T) {
	_, err := tracing.Init(context.Background(), config.TracingConfig{
		Endpoint: "localhost:4317",
		Protocol: "thrift",
		Insecure: true,
	})
	if err == nil {
		t.Fatal("Init() with unsupported protocol should return error")
	}
}

func TestInitInvalidSampleRate(t *testing.T) {
	tests := []struct {
		name string
		rate float64
	}{
		{"negative", -0.5},
		{"above one", 1.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tracing.Init(context.Background(), config.TracingConfig{
				Endpoint:   "localhost:4317",
				Protocol:   "grpc",
				Insecure:   true,
				SampleRate: tt.rate,
			})
			if err == nil {
				t.Fatalf("Init() with sample_rate=%g should return error", tt.rate)
			}
		})
	}
}

func TestShouldPropagateOverride(t *testing.T) {
	falseVal := false
	p, err := tracing.Init(context.Background(), config.TracingConfig{
		Endpoint:  "localhost:4317",
		Protocol:  "grpc",
		Insecure:  true,
		Propagate: &falseVal,
	})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	if p.ShouldPropagate() {
		t.Error("ShouldPropagate() = true, want false when explicitly disabled")
	}
}

func TestNilProviderSafety(t *testing.T) {
	var p *tracing.Provider
	if p.ShouldPropagate() {
		t.Error("nil provider ShouldPropagate() = true, want false")
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Errorf("nil provider Shutdown() error = %v", err)
	}
	// Tracer() on nil should return no-op, not panic
	tracer := p.Tracer()
	_, span := tracer.Start(context.Background(), "test")
	span.End()
}

func TestStartRequestSpan(t *testing.T) {
	exporter, tracer := setupTestTracer(t)

	tests := []struct {
		name         string
		protocol     string
		endpoint     string
		wantSpanName string
	}{
		{"http with endpoint", "http", "get-users", "http get-users"},
		{"http without endpoint", "http", "", "http request"},
		{"grpc with endpoint", "grpc", "MyService/GetData", "grpc MyService/GetData"},
		{"websocket", "websocket", "", "websocket request"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter.Reset()

			ctx, span := tracing.StartRequestSpan(context.Background(), tracer, tt.protocol, tt.endpoint)
			_ = ctx
			span.End()

			spans := exporter.GetSpans()
			if len(spans) != 1 {
				t.Fatalf("got %d spans, want 1", len(spans))
			}
			got := spans[0].Name
			if got != tt.wantSpanName {
				t.Errorf("span name = %q, want %q", got, tt.wantSpanName)
			}

			// Verify rpc.system attribute
			foundSystem := false
			for _, attr := range spans[0].Attributes {
				if string(attr.Key) == "rpc.system" && attr.Value.AsString() == tt.protocol {
					foundSystem = true
				}
			}
			if !foundSystem {
				t.Errorf("rpc.system attribute not found or incorrect")
			}
		})
	}
}

func TestEndSpanRecordsError(t *testing.T) {
	exporter, tracer := setupTestTracer(t)

	_, span := tracer.Start(context.Background(), "test-error")
	tracing.EndSpan(span, context.DeadlineExceeded)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	if spans[0].Status.Code != codes.Error {
		t.Errorf("span status code = %d, want %d (Error)", spans[0].Status.Code, codes.Error)
	}
}

func TestEndSpanOk(t *testing.T) {
	exporter, tracer := setupTestTracer(t)

	_, span := tracer.Start(context.Background(), "test-ok")
	tracing.EndSpan(span, nil)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	if spans[0].Status.Code != codes.Ok {
		t.Errorf("span status code = %d, want %d (Ok)", spans[0].Status.Code, codes.Ok)
	}
}

func TestInjectHTTPHeaders(t *testing.T) {
	_, tracer := setupTestTracer(t)

	ctx, span := tracer.Start(context.Background(), "test-inject")
	defer span.End()

	headers := make(http.Header)
	tracing.InjectHTTPHeaders(ctx, headers)

	got := headers.Get("Traceparent")
	if got == "" {
		t.Error("traceparent header not injected")
	}
	// traceparent format: version-traceid-spanid-flags (e.g., 00-abc123...-def456...-01)
	if len(got) < 55 {
		t.Errorf("traceparent header too short: %q", got)
	}
}

func TestInjectGRPCMetadata(t *testing.T) {
	_, tracer := setupTestTracer(t)

	ctx, span := tracer.Start(context.Background(), "test-grpc-inject")
	defer span.End()

	md := metadata.New(nil)
	tracing.InjectGRPCMetadata(ctx, md)

	vals := md.Get("traceparent")
	if len(vals) == 0 {
		t.Error("traceparent not injected into gRPC metadata")
	}
	if len(vals[0]) < 55 {
		t.Errorf("traceparent metadata too short: %q", vals[0])
	}
}

func TestInjectHTTPHeadersNoSpan(t *testing.T) {
	// Without a span in context, injection should not panic and not set traceparent
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
	))
	headers := make(http.Header)
	tracing.InjectHTTPHeaders(context.Background(), headers)

	got := headers.Get("Traceparent")
	if got != "" {
		t.Errorf("traceparent header should be empty without span, got %q", got)
	}
}
