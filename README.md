# Crankfire

[![Go Report Card](https://goreportcard.com/badge/github.com/torosent/crankfire)](https://goreportcard.com/report/github.com/torosent/crankfire)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/torosent/crankfire)](https://github.com/torosent/crankfire/releases)
[![Go Build](https://github.com/torosent/crankfire/actions/workflows/release.yml/badge.svg)](https://github.com/torosent/crankfire/actions/workflows/release.yml)
[![codecov](https://codecov.io/gh/torosent/crankfire/branch/main/graph/badge.svg)](https://codecov.io/gh/torosent/crankfire)


High-signal load testing for HTTP, WebSocket, SSE, and gRPC from the CLI.

Crankfire lets you describe realistic workloads against these protocols using one cohesive config model (choose one protocol per run). It’s built for engineers who care about **proper arrival modeling, protocol‑aware metrics, and tight CI/CD integration**—without running a cluster or a web UI.

👉 **Docs:** https://torosent.github.io/crankfire/

## Why Crankfire?

Use Crankfire when you need more than a simple `curl` loop, but don’t want the overhead of a heavyweight load-testing platform.

- **Multi‑protocol coverage** – HTTP, WebSocket, SSE, and gRPC share the same configuration and reporting engine (select the protocol mode per run).
- **Realistic traffic patterns** – Ramp/step/spike load phases plus uniform or Poisson arrivals.
- **Production‑grade metrics** – HDR histogram percentiles (P50/P90/P95/P99), per‑endpoint stats, and protocol‑specific error buckets.
- **Live dashboard or JSON** – Watch tests in your terminal, or export structured JSON for automation.
- **Auth & data built‑in** – OAuth2/OIDC helpers and CSV/JSON feeders for realistic test data.
- **Request chaining** – Extract values from responses (JSON path, regex) and use them in subsequent requests.
- **HAR import** – Record browser sessions and replay them as load tests with automatic filtering.
- **Single binary** – Written in Go with minimal runtime dependencies.

See the [full feature overview in the docs](https://torosent.github.io/crankfire/).

## Feature Matrix

| Feature | HTTP | WebSocket | SSE | gRPC |
|---------|:----:|:---------:|:---:|:----:|
| **Basic Load Testing** | ✅ | ✅ | ✅ | ✅ |
| **Authentication** | ✅ | ✅ | ✅ | ✅ |
| **Data Feeders** | ✅ | ✅ | ✅ | ✅ |
| **Request Chaining** | ✅ | — | — | — |
| **HAR Import** | ✅ | — | — | — |
| **Thresholds/Assertions** | ✅ | ✅ | ✅ | ✅ |
| **Retries** | ✅ | ❌ | ❌ | ❌ |
| **Protocol-Specific Metrics** | - | Messages sent/received, bytes | Events received, bytes | Calls, responses |
| **Dashboard Support** | ✅ | ✅ | ✅ | ✅ |
| **HTML Report** | ✅ | ✅ | ✅ | ✅ |
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
### Docker

```bash
docker run ghcr.io/torosent/crankfire --target https://example.com --total 100
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

### POST Request

```bash
crankfire \
  --target https://api.example.com/users \
  --method POST \
  --body '{"name":"crankfire"}' \
  --header "Content-Type=application/json" \
  --total 100
```

### With a Config File

```bash
crankfire --config loadtest.yaml
```

### Generate HTML Report

```bash
crankfire --target https://example.com --total 1000 --html-output report.html
```

### From a HAR File

Record a browser session and replay it as a load test:

```bash
crankfire --har recording.har --har-filter "host:api.example.com" --total 100
```

See the [Getting Started guide](https://torosent.github.io/crankfire/getting-started.html) for a step‑by‑step walkthrough.

## Command-Line Options (Overview)

| Flag | Description | Default |
|------|-------------|---------|
| `--target` | Target URL to test | (required unless using `--har`) |
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
| `--html-output` | Generate HTML report to the specified file path | - |
| `--json-output` | Output results as JSON | false |
| `--dashboard` | Show live terminal dashboard | false |
| `--log-errors` | Log each failed request to stderr | false |
| `--config` | Path to config file (JSON/YAML) | - |
| `--har` | Path to HAR file to import as endpoints | - |
| `--har-filter` | Filter HAR entries (e.g., `host:example.com` or `method:GET,POST`) | - |
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

## Advanced Usage Scenarios

### 1. Realistic Traffic Pattern (Ramp-up & Sustained Load)

Simulate a realistic traffic curve with a warm-up phase followed by a sustained load period.

```yaml
target: https://api.example.com
concurrency: 50
load_patterns:
  - name: "warmup"
    type: ramp
    from_rps: 10
    to_rps: 100
    duration: 30s
  - name: "sustained"
    type: step
    steps:
      - rps: 100
        duration: 2m
```

### 2. Multi-Endpoint API Test (Weighted)

Distribute traffic across multiple endpoints to simulate real user behavior (e.g., 80% browsing, 20% purchasing).

```yaml
target: https://api.example.com
concurrency: 20
endpoints:
  - name: "list-products"
    weight: 8
    method: GET
    path: /products
  - name: "create-order"
    weight: 2
    method: POST
    path: /orders
    body: '{"product_id": "123", "quantity": 1}'
```

### 3. Data-Driven Testing (CSV Feeder)

Inject dynamic data from a CSV file into request bodies or URLs.

**orders.csv**:
```csv
product_id,quantity
101,2
102,1
103,5
```

**config.yaml**:
```yaml
target: https://api.example.com/orders
method: POST
feeder:
  type: csv
  path: ./orders.csv
body: '{"product_id": "{{.product_id}}", "quantity": {{.quantity}}}'
```

### 4. Request Chaining (Extract & Reuse)

Extract values from responses and use them in subsequent requests. Perfect for workflows like login → use token → access protected resources.

```yaml
target: https://api.example.com
concurrency: 10
endpoints:
  - name: "login"
    weight: 1
    method: POST
    path: /auth/login
    body: '{"email": "user@example.com", "password": "secret"}'
    extractors:
      - jsonpath: $.token
        var: auth_token
      - jsonpath: $.user.id
        var: user_id

  - name: "get-profile"
    weight: 3
    method: GET
    path: /users/{{user_id}}
    headers:
      Authorization: "Bearer {{auth_token}}"
```

See the [Request Chaining documentation](https://torosent.github.io/crankfire/request-chaining) for JSON path, regex extraction, defaults, and error handling.

### 5. gRPC Load Test

Load test a gRPC service using a Protocol Buffers definition.

```yaml
target: localhost:50051
protocol: grpc
grpc:
  proto_file: ./proto/service.proto
  service: myapp.OrderService
  method: CreateOrder
  message: '{"user_id": "123", "item": "book"}'
  timeout: 2s
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

### HTML Report

Generate a standalone HTML report with interactive charts and detailed statistics.

```bash
crankfire --target https://example.com --total 1000 --html-output report.html
```

The report includes:
- **Summary Cards**: Key metrics at a glance
- **Interactive Charts**: RPS and latency percentiles over time
- **Latency Statistics**: Detailed percentile breakdown
- **Threshold Results**: Pass/fail status for configured thresholds
- **Endpoint Breakdown**: Per-endpoint performance metrics

<img width="1455" height="858" alt="Image1" src="https://github.com/user-attachments/assets/15837533-7e60-4553-9104-4bfdf16b9d79" />

<img width="1455" height="858" alt="Image2" src="https://github.com/user-attachments/assets/f8b81f9a-0b16-41b7-9654-49cc47ac9af4" />

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

<img width="1018" height="696" alt="Image" src="https://github.com/user-attachments/assets/4f2a30f1-aed7-4a38-b8bf-e37d37e43611" />

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
