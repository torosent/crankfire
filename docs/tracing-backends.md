# Trace Backends

Crankfire emits OpenTelemetry traces over OTLP. Set `--tracing-endpoint`
(or env `OTEL_EXPORTER_OTLP_ENDPOINT`) and Crankfire will export one root
span per request, with child spans for connect / TLS / write / read where
applicable.

This page lists copy-pasteable configurations for the most common backends.

## 1. Tempo (Grafana Cloud)

Grafana Cloud Tempo accepts OTLP gRPC over TLS.

```bash
crankfire run \
  --tracing-endpoint=tempo-prod-04-prod-us-east-0.grafana.net:443 \
  --tracing-protocol=grpc \
  --tracing-header="authorization=Basic $GRAFANA_BASIC_AUTH"
```

`$GRAFANA_BASIC_AUTH` should be base64(`<instance-id>:<api-key>`).

**What you'll see:** in Grafana Cloud Tempo, search for service `crankfire`.
Each test request becomes a root span; HTTP requests show child spans for
`http.connect`, `http.tls`, `http.write_request`, `http.read_response`.

## 2. Tempo (self-hosted)

Bare Tempo with no auth, OTLP gRPC on 4317:

```bash
crankfire run \
  --tracing-endpoint=tempo.observability.svc:4317 \
  --tracing-protocol=grpc \
  --tracing-insecure
```

## 3. Jaeger (OTLP receiver)

Jaeger 1.35+ accepts OTLP natively (4317/4318):

```bash
crankfire run \
  --tracing-endpoint=jaeger.observability.svc:4317 \
  --tracing-protocol=grpc \
  --tracing-insecure
```

## 4. Honeycomb

OTLP HTTP at api.honeycomb.io:

```bash
crankfire run \
  --tracing-endpoint=https://api.honeycomb.io:443 \
  --tracing-protocol=http \
  --tracing-header="x-honeycomb-team=$HONEYCOMB_API_KEY" \
  --tracing-header="x-honeycomb-dataset=crankfire"
```

## 5. OpenTelemetry Collector sidecar

Run a collector locally; let it fan out to your real backend:

```bash
crankfire run \
  --tracing-endpoint=localhost:4317 \
  --tracing-protocol=grpc \
  --tracing-insecure
```

Sample collector config:

```yaml
receivers:
  otlp: { protocols: { grpc: {} } }
exporters:
  tempo: { endpoint: tempo:4317, tls: { insecure: true } }
service:
  pipelines:
    traces: { receivers: [otlp], exporters: [tempo] }
```

## 6. Local Jaeger via Docker

Quick local exploration:

```bash
docker run -d --rm --name jaeger \
  -p 16686:16686 -p 4317:4317 \
  jaegertracing/all-in-one:1.57

crankfire run \
  --tracing-endpoint=localhost:4317 \
  --tracing-protocol=grpc \
  --tracing-insecure
```

Open `http://localhost:16686`, select service `crankfire`, click "Find
Traces".

---

## Notes

- Tracing is opt-in. Without `--tracing-endpoint` (or `OTEL_*` env), no
  traces are emitted.
- Span naming: `http.request`, `grpc.invoke`, `ws.send`, `sse.event`.
- All `OTEL_*` standard env variables are honored as documented in the
  OpenTelemetry SDK.
