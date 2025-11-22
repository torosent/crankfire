# Crankfire

High-signal load testing for HTTP, WebSocket, SSE, and gRPC from the CLI.

Crankfire lets you describe realistic workloads against these protocols using one cohesive config model (choose one protocol per run). It’s built for engineers who care about **proper arrival modeling, protocol‑aware metrics, and tight CI/CD integration**—without running a cluster or a web UI.

👉 **Docs:** https://torosent.github.io/crankfire/

## Why Crankfire?

Use Crankfire when you need more than a simple `curl` loop, but don’t want the overhead of a heavyweight load-testing platform.

- **Multi‑protocol coverage** – HTTP, WebSocket, SSE, and gRPC share the same configuration and reporting engine (select the protocol mode per run).
- **Realistic traffic patterns** – Ramp/step/spike load phases plus uniform or Poisson arrivals.
- **Production‑grade metrics** – HDR histogram percentiles (P50/P90/P99), per‑endpoint stats, and protocol‑specific error buckets.
- **Live dashboard or JSON** – Watch tests in your terminal, or export structured JSON for automation.
- **Auth & data built‑in** – OAuth2/OIDC helpers and CSV/JSON feeders for realistic test data.
- **Single binary** – Written in Go with minimal runtime dependencies.

See the [full feature overview in the docs](https://torosent.github.io/crankfire/).

## Feature Matrix

| Feature | HTTP | WebSocket | SSE | gRPC |
|---------|:----:|:---------:|:---:|:----:|
| **Basic Load Testing** | ✅ | ✅ | ✅ | ✅ |
| **Authentication** | ✅ | ✅ | ✅ | ✅ |
| **Data Feeders** | ✅ | ✅ | ✅ | ✅ |
| **Retries** | ✅ | ❌ | ❌ | ✅ |
| **Thresholds/Assertions** | ✅ | ✅ | ✅ | ✅ |
| **Protocol-Specific Metrics** | - | Messages sent/received, bytes | Events received, bytes | Calls, responses |
| **Dashboard Support** | ✅ | ✅ | ✅ | ✅ |
| **JSON Output** | ✅ | ✅ | ✅ | ✅ |
| **Rate Limiting** | ✅ | ✅ | ✅ | ✅ |
| **Arrival Models** | ✅ | ✅ | ✅ | ✅ |
| **Load Patterns** | ✅ | ✅ | ✅ | ✅ |
| **Multiple HTTP Endpoints** | ✅ | — | — | — |

## Use Cases

- **Performance Testing**: Measure API response times and throughput
- **Load Testing**: Simulate concurrent users and high traffic
- **Stress Testing**: Find breaking points (histogram percentile accuracy)
- **CI/CD Integration**: Automated performance regression testing with JSON output
- **Failure Analysis**: Status bucket breakdown highlights protocol-specific failure patterns

## Installation

### Homebrew (macOS and Linux)

```bash
brew tap torosent/crankfire
brew install crankfire
```

### Go Install

```bash
go install github.com/torosent/crankfire/cmd/crankfire@latest
```

### Build from Source

```bash
git clone https://github.com/torosent/crankfire.git
cd crankfire
go build -o build/crankfire ./cmd/crankfire
```

For more installation options including Docker and pre-built binaries, see [INSTALLATION.md](docs/INSTALLATION.md) (or the **Getting Started** docs).

## Quick Start

### One‑liner

```bash
crankfire --target https://example.com --concurrency 10 --total 100
```

### With Dashboard

```bash
crankfire \
  --target https://api.example.com \
  --concurrency 20 \
  --rate 100 \
  --duration 30s \
  --dashboard
```

### With Rate Limiting

```bash
crankfire \
  --target https://api.example.com \
  --concurrency 20 \
  --rate 100 \
  --duration 30s
```

### With a Config File

```bash
crankfire --config loadtest.yaml
```

See the [Getting Started guide](https://torosent.github.io/crankfire/getting-started.html) for a step‑by‑step walkthrough.

## Command-Line Options (Overview)

| Flag | Description | Default |
|------|-------------|---------|
| `--target` | Target URL to test | (required) |
| `--method` | HTTP method (GET, POST, PUT, DELETE, PATCH, etc.; case-insensitive, defaults to GET when omitted) | GET |
| `--header` | Add HTTP header (`Key=Value`, repeatable; last wins) | - |
| `--body` | Inline request body | - |
| `--body-file` | Path to request body file | - |
| `--concurrency`, `-c` | Number of parallel workers | 1 |
| `--rate`, `-r` | Requests per second (0=unlimited) | 0 |
| `--duration`, `-d` | Test duration (e.g., 30s, 1m) | 0 |
| `--total`, `-t` | Total number of requests | 0 |
| `--timeout` | Per-request timeout | 30s |
| `--retries` | Number of retry attempts | 0 |
| `--arrival-model` | Arrival model (`uniform` or `poisson`) | uniform |
| `--json-output` | Output results as JSON | false |
| `--dashboard` | Show live terminal dashboard | false |
| `--log-errors` | Log each failed request to stderr | false |
| `--config` | Path to config file (JSON/YAML) | - |
| `--feeder-path` | Path to CSV/JSON file for per-request data injection | - |
| `--feeder-type` | Feeder file type (`csv` or `json`) | - |
| `--protocol` | Protocol mode (`http`, `websocket`, or `sse`) | http |
| `--ws-messages` | WebSocket messages to send (repeatable) | - |
| `--ws-message-interval` | Interval between WebSocket messages | 0 |
| `--ws-receive-timeout` | WebSocket receive timeout | 10s |
| `--ws-handshake-timeout` | WebSocket handshake timeout | 30s |
| `--sse-read-timeout` | SSE read timeout | 30s |
| `--sse-max-events` | Max SSE events to read (0=unlimited) | 0 |
| `--grpc-proto-file` | Path to .proto file for gRPC | - |
| `--grpc-service` | gRPC service name (e.g., `myapp.Service`) | - |
| `--grpc-method` | gRPC method name (e.g., `GetData`) | - |
| `--grpc-message` | JSON message payload for gRPC (supports templates) | - |
| `--grpc-metadata` | gRPC metadata (repeatable, `Key=Value`) | - |
| `--grpc-timeout` | gRPC call timeout | 30s |
| `--grpc-tls` | Use TLS for gRPC | false |
| `--grpc-insecure` | Skip TLS certificate verification | false |
| `--threshold` | Performance threshold (repeatable, e.g., `http_req_duration:p95 < 500`) | - |

For a complete CLI reference and configuration guide, see [Configuration & CLI Reference](https://torosent.github.io/crankfire/configuration.html).

## Thresholds (CI/CD Integration)

Define pass/fail criteria for your load tests. Perfect for catching performance regressions in CI pipelines.

```bash
crankfire --target https://api.example.com \
  --concurrency 50 \
  --duration 1m \
  --threshold "http_req_duration:p95 < 500" \
  --threshold "http_req_failed:rate < 0.01" \
  --threshold "http_requests:rate > 100"
```

Or in a config file:

```yaml
target: https://api.example.com
concurrency: 50
duration: 1m
thresholds:
  - "http_req_duration:p95 < 500"   # 95th percentile under 500ms
  - "http_req_failed:rate < 0.01"   # Less than 1% failures
  - "http_requests:rate > 100"      # At least 100 RPS
```

The test exits with code 1 if any threshold fails, making it ideal for CI/CD gates. See [Thresholds Documentation](https://torosent.github.io/crankfire/thresholds.html) for details.

## Configuration File

Example JSON config:

```json
{
  "target": "https://api.example.com/users",
  "method": "POST",
  "headers": {
    "Content-Type": "application/json",
    "Authorization": "Bearer token123"
  },
  "body": "{\"name\":\"test\"}",
  "concurrency": 50,
  "rate": 200,
  "duration": "1m",
  "timeout": "5s",
  "retries": 3
}
```

Example YAML config:

```yaml
target: https://api.example.com/users
method: POST
headers:
  Content-Type: application/json
  Authorization: Bearer token123
body: '{"name":"test"}'
concurrency: 50
rate: 200
duration: 1m
timeout: 5s
retries: 3
```

## Output Examples

### Human-Readable

```
--- Load Test Results ---
Total Requests:    1000
Successful:        998
Failed:            2
Duration:          10.2s
Requests/sec:      98.04

Latency:
  Min:             12ms
  Max:             156ms
  Mean:            45ms
  P50:             42ms
  P90:             68ms
  P99:             112ms

Status Buckets:
  HTTP 404: 1
  HTTP 500: 1
```

### JSON Output

```bash
crankfire --target https://example.com --total 100 --json-output
```

```json
{
  "total": 100,
  "successes": 100,
  "failures": 0,
  "min_latency_ms": 12.4,
  "max_latency_ms": 156.8,
  "mean_latency_ms": 45.2,
  "p50_latency_ms": 42.1,
  "p90_latency_ms": 68.3,
  "p99_latency_ms": 112.7,
  "duration_ms": 10245.3,
  "requests_per_sec": 9.76,
  "status_buckets": {
    "http": {
      "404": 2,
      "500": 1
    }
  },
  "endpoints": {
    "list-users": {
      "total": 600,
      "successes": 600,
      "failures": 0,
      "p99_latency_ms": 95.1,
      "requests_per_sec": 58.6
    },
    "create-order": {
      "total": 400,
      "successes": 392,
      "failures": 8,
      "p99_latency_ms": 180.4,
      "requests_per_sec": 39.1,
      "status_buckets": {
        "http": {
          "503": 8
        }
      }
    }
  }
}
```

## Live Terminal Dashboard

The dashboard provides a real-time, interactive view of your load test with the following features:

- **Sparkline Latency Graph**: Visual representation of latency over time
- **RPS Gauge**: Current requests per second with percentage indicator
- **Metrics Table**: Real-time statistics including total requests, success/failure counts, and latency percentiles
- **Status Buckets**: Protocol-specific failure codes with counts
- **Endpoint Breakdown**: Weighted endpoint view with live share, RPS, and tail latency
- **Test Summary**: Elapsed time, total requests, and success rate

### Usage

```bash
crankfire --target https://api.example.com \
  --concurrency 20 \
  --rate 100 \
  --duration 60s \
  --dashboard
```

### Dashboard Layout

```
┌ Test Summary ──────────────────────────────────────────────────┐
│ Elapsed: 30s | Total: 3000 | Success Rate: 98.5%               │
└────────────────────────────────────────────────────────────────┘

┌ Requests Per Second -┐  ┌ Metrics ──────────────────────────────┐
│ 100.5 RPS            │  │ Total Requests    3000                │
│ ████████████░░░░░░   │  │ Successes         2955                │
│ 85%                  │  │ Failures          45                  │
└──────────────────────┘  │ Success Rate      98.5%               │
                          │ Min Latency       12.45ms             │
                          │ Mean Latency      45.23ms             │
                          │ P99 Latency       156.78ms            │
                          └───────────────────────────────────────┘

┌ Real-time Latency ─────────────────────────────────────────────┐
│ Latency (ms)                                                   │
│ ▂▃▄▅▃▂▃▄▅▆▅▄▃▂▃▄▅▆▇▆▅▄▃▂▃▄▅▆▅▄▃▂                               │
└────────────────────────────────────────────────────────────────┘

┌ Endpoints ───────────────────────────────────────────────────────────────────┐
│ list-users  | 60.0% | RPS 240.0 | P99 120.3ms | Err 2 | Status HTTP 404 x2   │
│ create-order| 40.0% | RPS 160.0 | P99 210.5ms | Err 12 | Status HTTP 503 x12 │
└──────────────────────────────────────────────────────────────────────────────┘

┌ Status Buckets ────────────────────────────────────────────────┐
│ HTTP 404 38                                                    │
│ HTTP 503 7                                                     │
└────────────────────────────────────────────────────────────────┘
```

## Responsible Use and Legal Notice

⚠️ **IMPORTANT**: Crankfire is a powerful load testing tool that can generate significant traffic against target systems.

**You MUST:**
- Only use Crankfire against systems you own or have explicit written permission to test
- Verify you have authorization before running any load tests
- Be aware that unauthorized load testing may violate terms of service, acceptable use policies, or laws
- Consider the impact on production systems and third-party services

**Security Best Practices:**
- Never commit authentication credentials to version control
- Use environment variables for secrets: `CRANKFIRE_AUTH_CLIENT_SECRET`, `CRANKFIRE_AUTH_PASSWORD`, `CRANKFIRE_AUTH_STATIC_TOKEN`
- Avoid passing secrets via CLI flags (they appear in shell history and process lists)
- Use short-lived tokens with minimal scopes
- Prefer `oauth2_client_credentials` over legacy `oauth2_resource_owner` password flow
- For gRPC, always enable TLS verification in production (`insecure: false`)

**Rate Limiting:**
- Start with conservative rate and concurrency settings
- Gradually increase load while monitoring target system health

Misuse of this tool may result in service disruptions, account termination, legal action, or criminal charges. The authors and contributors are not responsible for misuse of this software.

## License

MIT
