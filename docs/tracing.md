---
layout: default
title: OpenTelemetry Tracing
---
{% raw %}

# OpenTelemetry Tracing

Crankfire can create an OpenTelemetry span for each request it sends, export the spans over OTLP, and propagate W3C trace context to the system under test. Your backend traces then line up with the load test that caused them.

## Quick start

```bash
crankfire --target https://api.example.com \
  --concurrency 20 --duration 1m \
  --tracing-endpoint localhost:4317 \
  --tracing-insecure \
  --tracing-sample-rate 1.0
```

> **Note:** Always set the sample rate explicitly, either `--tracing-sample-rate` or `sample_rate:`. When it is omitted, the effective value is currently `0`, which samples **no** spans, even though `--help` shows a default of 1. Trace context is still propagated, but marked as not sampled.

## Configuration

Tracing is enabled **only** when an endpoint is set, with `--tracing-endpoint` or `tracing.endpoint`.

| Flag | Config key (`tracing.*`) | Effective default | Description |
|---|---|---|---|
| `--tracing-endpoint` | `endpoint` | none (tracing off) | OTLP collector address as `host:port`, without a scheme |
| `--tracing-protocol` | `protocol` | `grpc` | OTLP transport: `grpc` or `http` (HTTP uses `/v1/traces`) |
| `--tracing-service-name` | `service_name` | `$OTEL_SERVICE_NAME`, else `crankfire` | `service.name` resource attribute |
| `--tracing-sample-rate` | `sample_rate` | `0` when omitted (see the note above) | `0.0`–`1.0`; `1` = always, between 0 and 1 = trace-ID ratio |
| `--tracing-insecure` | `insecure` | `false` | Plaintext connection to the collector (no TLS) |
| none | `propagate` | `true` when tracing is enabled | Inject W3C `traceparent`/`tracestate` and `baggage` into outgoing requests |

```yaml
target: https://api.example.com
concurrency: 20
duration: 1m
tracing:
  endpoint: localhost:4317
  protocol: grpc
  service_name: checkout-load-test
  sample_rate: 0.1
  insecure: true
  propagate: true
```

Sampling is parent-based. A sample rate outside `[0, 1]` is an error.

### Environment variables

- `OTEL_SERVICE_NAME` is used when no service name is configured.
- The OTLP exporter library also reads the standard `OTEL_EXPORTER_OTLP_*` variables, such as `OTEL_EXPORTER_OTLP_HEADERS`. Use them for collector authentication headers. Crankfire has no flag for exporter headers.
- `OTEL_EXPORTER_OTLP_ENDPOINT` alone does **not** turn tracing on, because tracing is enabled only by `--tracing-endpoint` or `tracing.endpoint`.

## What gets traced

- **One client span per request**, with no child spans for DNS, TLS, connect or read.
- **Span name:** `<protocol> <endpoint>`, for example `http list-products` or `grpc helloworld.Greeter/SayHello`. When there is no endpoint name it is `<protocol> request`, for example `websocket request`.
- **Attributes:** `rpc.system` is the protocol (`http`, `websocket`, `sse`, `grpc`). `crankfire.endpoint` is the endpoint name when one exists. HTTP spans also carry `http.response.status_code`.
- **Errors** are recorded on the span and set its status to `Error`.

The span covers the same interval as the latency Crankfire measures: building the request, the auth token lookup, the call itself, and reading the response.

**Propagation** works on every protocol:

| Protocol | Where trace context is injected |
|---|---|
| HTTP | Request headers |
| WebSocket | Handshake request headers |
| SSE | Stream request headers |
| gRPC | Outgoing metadata |

To export spans without touching the target's traces, set `propagate: false`. This option exists only in the config file.

Spans are exported in batches. Pending spans are flushed when the run ends, with a 5-second shutdown timeout.

## Backend recipes

### Jaeger (local)

```bash
docker run --rm -d --name jaeger -p 16686:16686 -p 4317:4317 -p 4318:4318 \
  jaegertracing/all-in-one:latest

crankfire --target http://localhost:8080 --total 500 \
  --tracing-endpoint localhost:4317 --tracing-insecure --tracing-sample-rate 1
```

Open <http://localhost:16686> and look for the `crankfire` service.

### OTLP over HTTP

```bash
crankfire --target https://api.example.com --duration 30s \
  --tracing-endpoint localhost:4318 --tracing-protocol http --tracing-insecure \
  --tracing-sample-rate 1
```

### OpenTelemetry Collector or Grafana Tempo

Point Crankfire at the collector's OTLP receiver:

```yaml
tracing:
  endpoint: otel-collector.internal:4317
  protocol: grpc
  sample_rate: 0.2
```

Leave out `insecure` when the receiver uses TLS.

### Honeycomb (or any vendor that authenticates with headers)

```bash
export OTEL_EXPORTER_OTLP_HEADERS="x-honeycomb-team=$HONEYCOMB_API_KEY"
crankfire --target https://api.example.com --duration 1m \
  --tracing-endpoint api.honeycomb.io:443 --tracing-sample-rate 0.1
```

## Tips

- **Sample at high rates.** Exporting a span for every request at thousands of RPS can load both Crankfire and the collector, so use a fractional `sample_rate`.
- **Keep the load test separate in your backend.** Give it its own service name so its spans don't mix with production client traffic.
- **Correlate with metrics.** Crankfire's own metrics, such as the report percentiles, do not depend on tracing. Tracing is for following individual requests through your backend.

See also: [Configuration](configuration.md), [Protocols](protocols.md).
{% endraw %}
