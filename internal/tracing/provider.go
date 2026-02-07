// Package tracing provides OpenTelemetry initialization and W3C trace context propagation.
package tracing

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/torosent/crankfire/internal/config"
)

// Provider wraps the OTel TracerProvider and provides convenience methods.
type Provider struct {
	tp        *sdktrace.TracerProvider
	tracer    trace.Tracer
	propagate bool
}

// Init creates an OTel TracerProvider from config. Returns a no-op provider if tracing is disabled.
func Init(ctx context.Context, cfg config.TracingConfig) (*Provider, error) {
	if !cfg.Enabled() {
		return &Provider{propagate: false}, nil
	}

	serviceName := cfg.ServiceName
	if serviceName == "" {
		if envName := os.Getenv("OTEL_SERVICE_NAME"); envName != "" {
			serviceName = envName
		} else {
			serviceName = "crankfire"
		}
	}

	endpoint := cfg.Endpoint
	if endpoint == "" {
		if envEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); envEndpoint != "" {
			endpoint = envEndpoint
		}
	}
	if endpoint == "" {
		return &Provider{propagate: cfg.ShouldPropagate()}, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("tracing resource: %w", err)
	}

	exporter, err := newExporter(ctx, cfg, endpoint)
	if err != nil {
		return nil, fmt.Errorf("tracing exporter: %w", err)
	}

	if cfg.SampleRate < 0 || cfg.SampleRate > 1.0 {
		return nil, fmt.Errorf("tracing sample_rate must be between 0.0 and 1.0, got %g", cfg.SampleRate)
	}

	sampler := sdktrace.AlwaysSample()
	if cfg.SampleRate > 0 && cfg.SampleRate < 1.0 {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	} else if cfg.SampleRate == 0 {
		sampler = sdktrace.NeverSample()
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sampler)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Provider{
		tp:        tp,
		tracer:    tp.Tracer("crankfire"),
		propagate: cfg.ShouldPropagate(),
	}, nil
}

// Tracer returns the configured tracer. Returns a no-op tracer if tracing is disabled.
func (p *Provider) Tracer() trace.Tracer {
	if p == nil || p.tracer == nil {
		return noop.NewTracerProvider().Tracer("crankfire")
	}
	return p.tracer
}

// ShouldPropagate returns whether W3C trace headers should be injected.
func (p *Provider) ShouldPropagate() bool {
	if p == nil {
		return false
	}
	return p.propagate
}

// Shutdown flushes pending spans and shuts down the provider.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil || p.tp == nil {
		return nil
	}
	return p.tp.Shutdown(ctx)
}

func newExporter(ctx context.Context, cfg config.TracingConfig, endpoint string) (sdktrace.SpanExporter, error) {
	protocol := strings.ToLower(cfg.Protocol)
	if protocol == "" {
		protocol = "grpc"
	}

	switch protocol {
	case "grpc":
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(endpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		return otlptracegrpc.New(ctx, opts...)

	case "http":
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(endpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		return otlptracehttp.New(ctx, opts...)

	default:
		return nil, fmt.Errorf("unsupported OTLP protocol %q: use \"grpc\" or \"http\"", protocol)
	}
}
