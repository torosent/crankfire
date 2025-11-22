# Configuration & CLI Reference

Crankfire can be configured entirely via CLI flags, config files, or a combination of both. This page focuses on how to structure configuration and how the important flags work.

## Configuration Sources

1. **CLI flags** – quickest way to tweak a run.
2. **Config files (JSON/YAML)** – best for reusable scenarios.
3. **Environment variables** – mainly for secrets (auth).

Flags always override config file values.

## Core Concepts

- **Target**: The URL or endpoint you are testing.
- **Concurrency**: Number of workers in parallel.
- **Rate**: Requests per second (0 = unbounded).
- **Duration vs Total**:
  - `--duration` limits by wall time.
  - `--total` limits by total number of requests.
- **Arrival model**: Uniform or Poisson spacing between requests.
- **Load patterns**: Ramp, step, spike phases on a shared timeline.

## Minimal Config Examples

### JSON


```json
{
  "target": "https://api.example.com/users",
  "concurrency": 10
  "rate": 100
}

### YAML


```



## Key CLI Flags

| Flag | Description |
|------|-------------|
| `--target` | Target URL (required). |
| `--method` | HTTP method (GET, POST, etc.). |
| `--concurrency`, `-c` | Number of parallel workers. |
| `--rate`, `-r` | Requests per second (0 = unlimited). |
| `--duration`, `-d` | Test duration (e.g. `30s`, `5m`). |
| `--total`, `-t` | Total requests (alternative to duration). |
| `--timeout` | Per-request timeout. |
| `--retries` | Number of retries with backoff. |
| `--arrival-model` | `uniform` or `poisson`. |
| `--config` | Path to JSON/YAML config. |
| `--json-output` | Emit a machine-readable JSON report. |
| `--html-output` | Generate a standalone HTML report. |
| `--dashboard` | Enable live terminal dashboard. |

For the full flag list, see the CLI help or the README.


## Arrival Models



Crankfire supports two arrival models:


- `uniform` – even spacing for a smooth request rate.
- `poisson` – random gaps drawn from an exponential distribution for realistic burstiness.


Configure globally with:

```bash
crankfire --arrival-model poisson ...
```

or

arrival:
  model: poisson
arrival:

## Load Patterns

Load patterns let you describe multi-phase tests in a single run.

```yaml
load_patterns:
  - name: warmup
    type: ramp
    from_rps: 10
    to_rps: 200
    duration: 5m
  - name: soak
    type: step
    steps:
      - rps: 200
        duration: 10m
      - rps: 300
        duration: 10m
  - name: spike
    type: spike
    rps: 800
    duration: 30s
```

All phases share a single timeline and respect global caps such as `total` and `duration` when set.

## Endpoints and Weights

Define multiple endpoints and let Crankfire sample between them using weights:

```yaml
target: https://api.example.com

endpoints:
  - name: list-users
    weight: 60
    path: /users
  - name: user-detail
    weight: 30
    path: /users/{id}
  - name: create-order
    weight: 10
    path: /orders
    method: POST
    body: '{"status":"pending"}'
```

Weights are relative; they do not have to sum to 100. Percentiles and status buckets are reported per endpoint in JSON output and the dashboard.

> **Scope:** Endpoint weighting is currently limited to HTTP runs. WebSocket, SSE, and gRPC modes ignore `endpoints` because each worker maintains a single connection.

## Headers

Use CLI:

```bash
crankfire --header "Authorization=Bearer token" --header "X-Env=prod" ...
```

or config:

```yaml
headers:
  Authorization: Bearer token
  X-Env: prod
```

Rules:

- Keys are canonicalized.
- Empty values are allowed.
- Last duplicate wins.
- CLI overrides config.

## Protocol Selection

Pick the protocol per run with the `protocol` flag or config field:

```yaml
protocol: grpc # http (default), websocket, sse, or grpc
```

Each run uses one protocol mode so Crankfire can collect protocol-aware metrics. To compare multiple protocols, run them one after another (or with separate configs) instead of mixing them in a single execution.

## Authentication Block

Configure OAuth2/OIDC helpers via the `auth` section (or rely on the same structure inside JSON configs). Auth is intentionally file-driven so secrets stay out of shell history.

```yaml
auth:
  type: oauth2_client_credentials
  token_url: https://idp.example.com/oauth/token
  client_id: svc-client
  client_secret: ${CRANKFIRE_AUTH_CLIENT_SECRET}
  scopes:
    - api:read
    - api:write
  refresh_before_expiry: 45s
```

Crankfire fetches tokens before the run, refreshes them ahead of expiry, and injects `Authorization: Bearer ...` headers for HTTP/WebSocket/SSE traffic or gRPC metadata.

## Feeder Block

Use `feeder` (or the `--feeder-*` flags) to drive each request with CSV/JSON records.

```yaml
feeder:
  path: ./users.csv
  type: csv
```

Placeholder syntax (`{{field}}`) is available in URLs, headers, HTTP bodies, WebSocket/SSE messages, and gRPC message JSON.

## gRPC Configuration

When `protocol: grpc`, configure call details under `grpc`:

```yaml
protocol: grpc
target: orders.example.com:50051

grpc:
  proto_file: ./api/orders.proto
  service: orders.OrderService
  method: CreateOrder
  message: '{"order_id":"{{order_id}}"}'
  metadata:
    x-tenant-id: tenant-a
  timeout: 5s
  tls: true
  insecure: false
```

- `proto_file` should resolve to the `.proto` that defines the service. Crankfire parses the descriptor at runtime, so you do not need generated Go code.
- `message` must match the request type defined in the proto. The JSON is transformed into a dynamic message before the RPC.
- `metadata` entries become lowercase gRPC metadata headers and can use feeder placeholders.
- TLS options map directly to the CLI flags.

## Combining Config and Flags

Typical workflow:

1. Put the baseline scenario in a config file.
2. Override concurrency, rate, or duration from the CLI per run.

Example:

```bash
crankfire --config loadtest.yaml --concurrency 100 --duration 10m
```
