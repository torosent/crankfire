package tracing

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
)

// StartRequestSpan starts a new span for a request operation.
func StartRequestSpan(ctx context.Context, tracer trace.Tracer, protocol, endpoint string) (context.Context, trace.Span) {
	spanName := protocol + " request"
	if endpoint != "" {
		spanName = protocol + " " + endpoint
	}
	ctx, span := tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
	)
	span.SetAttributes(
		attribute.String("rpc.system", protocol),
	)
	if endpoint != "" {
		span.SetAttributes(attribute.String("crankfire.endpoint", endpoint))
	}
	return ctx, span
}

// EndSpan finishes a span, recording error status if applicable.
func EndSpan(span trace.Span, err error, attrs ...attribute.KeyValue) {
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

// InjectHTTPHeaders injects W3C trace context into HTTP headers.
func InjectHTTPHeaders(ctx context.Context, headers http.Header) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(headers))
}

// grpcMetadataCarrier adapts grpc metadata.MD to the OTel TextMapCarrier interface.
type grpcMetadataCarrier metadata.MD

func (c grpcMetadataCarrier) Get(key string) string {
	vals := metadata.MD(c).Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func (c grpcMetadataCarrier) Set(key, value string) {
	metadata.MD(c).Set(key, value)
}

func (c grpcMetadataCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// InjectGRPCMetadata injects W3C trace context into gRPC metadata.
func InjectGRPCMetadata(ctx context.Context, md metadata.MD) {
	otel.GetTextMapPropagator().Inject(ctx, grpcMetadataCarrier(md))
}
