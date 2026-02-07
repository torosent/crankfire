# Copilot Instructions for Crankfire

## Build, Test, and Lint

```bash
# Build
go build -o build/crankfire ./cmd/crankfire

# Unit tests (with race detector)
go test -race ./...

# Single package
go test -race ./internal/runner/

# Single test
go test -race -run TestRunnerRespectsTotalRequests ./internal/runner/

# Integration tests (require build tag)
go test -v -tags=integration -race -timeout 15m ./cmd/crankfire/

# Coverage
go test -race -coverprofile=coverage.txt -covermode=atomic ./...
```

## Architecture

Crankfire is a multi-protocol CLI load tester (HTTP, WebSocket, SSE, gRPC) built as a single binary.

### Core flow

1. `cmd/crankfire/main.go` → `run(args)` parses config, builds dependencies, runs the test
2. `internal/config/` loads and merges CLI flags + YAML/JSON config files (no Cobra — custom loader with `spf13/pflag` + `viper`)
3. `internal/runner/` is the protocol-agnostic execution engine: manages workers, rate limiting (`x/time/rate`), load patterns (ramp/step/spike), and arrival models (uniform/Poisson)
4. Protocol requesters (`cmd/crankfire/{http,websocket,sse,grpc}_requester.go`) implement `runner.Requester` — a single-method interface:
   ```go
   type Requester interface { Do(ctx context.Context) error }
   ```
5. `cmd/crankfire/requester_factory.go` switches on `config.Protocol` to instantiate the correct requester

### Cross-cutting concerns

- **Auth** (`internal/auth/`): `Provider` interface injects auth headers. Implementations: static token, OAuth2/OIDC.
- **Feeders** (`internal/feeder/`): CSV/JSON data sources provide per-request template values.
- **Variables** (`internal/variables/`): Per-worker key/value store for extracted response values.
- **Extractors** (`internal/extractor/`): JSONPath/regex extraction from responses, stored into variables.
- **Placeholders** (`internal/placeholders/`): `{{field}}` template substitution in URLs, headers, and bodies.

These compose as: Feeder → Variables → Placeholders → Request → Extractor → Variables.

### Key packages

- `internal/metrics/` — Aggregate stats with HDR histograms, sharded (32) for concurrency.
- `internal/clientmetrics/` — Per-connection telemetry for stateful protocols (WS/SSE/gRPC).
- `internal/pool/` — Generic keyed connection pool (`Poolable` interface) for persistent-connection protocols.
- `internal/threshold/` — CI/CD pass/fail assertions on metrics (e.g., `http_req_duration:p95 < 500`).
- `internal/tracing/` — OpenTelemetry SDK init, span-per-request helpers, W3C trace context propagation (HTTP headers + gRPC metadata).
- `internal/output/` and `internal/dashboard/` — JSON, HTML report, and live terminal dashboard output.

### Decorator pattern

Requesters are wrapped with decorators for logging, retry, and endpoint selection — all composing over the `Requester` interface.

## Conventions

- **Tests use slice-based table-driven pattern** with `t.Run` subtests and `got`/`want` naming.
- **Fake implementations** (e.g., `fakeRequester`) are defined in test files for interface testing — no mock generation libraries.
- **Integration tests** are gated by `//go:build integration` and live in `cmd/crankfire/`. They spin up real HTTP/WebSocket/gRPC test servers.
- **Unit tests** use `_test` external package suffix (e.g., `package runner_test`).
- **Standard library assertions only** — no testify or go-cmp; use `t.Errorf`/`t.Fatalf` with direct comparisons.
- **Error wrapping** uses `fmt.Errorf("...: %w", err)` throughout.
- **Interfaces are minimal** — `Requester` is 1 method, `Provider` is 3 methods. Defined by consumers.
- **`baseRequesterHelper`** in `cmd/crankfire/base_requester.go` provides shared auth/feeder/metrics/tracing logic for all protocol requesters.
- **Tracing is opt-in** — disabled by default, enabled by setting `--tracing-endpoint`. Supports OTLP gRPC and HTTP exporters, W3C trace context propagation across all protocols, and standard `OTEL_*` env vars.
- **Environment secrets**: `CRANKFIRE_AUTH_CLIENT_SECRET`, `CRANKFIRE_AUTH_PASSWORD`, `CRANKFIRE_AUTH_STATIC_TOKEN`. Never hardcode credentials.
