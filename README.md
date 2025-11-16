# Crankfire

An optimized command-line load testing tool written in Go for HTTP endpoints, WebSocket, Server-Sent Events (SSE), and gRPC.

## Features

- **Live Terminal Dashboard**: Real-time metrics with sparkline latency graphs, RPS counters, protocol-specific metrics, and error breakdown
- **Multi-Protocol Support**: HTTP, WebSocket, SSE, and gRPC
- **Authentication Flows**: OAuth2 Client Credentials, Resource Owner Password, OIDC Implicit, and Auth Code flows
- **Data Feeders**: CSV and JSON data injection with placeholder substitution
- **Flexible Configuration**: Command-line flags, JSON, or YAML config files
- **Concurrency Control**: Configurable number of parallel workers
- **Rate Limiting**: Control requests per second
- **Metrics**: High accuracy Min/Max/Mean + P50/P90/P99 using HDR Histogram
- **Adaptive Retry Logic**: Conditional retries with exponential backoff + jitter
- **Multiple Output Formats**: Human-readable or structured JSON (includes error breakdown and protocol metrics)
- **Real-time Progress**: Lightweight periodic CLI updates with protocol status
- **Advanced Workload Modeling**: Ramp/step/spike phases, Poisson arrivals, and weighted endpoint mixes

## Feature Matrix

| Feature | HTTP | WebSocket | SSE | gRPC |
|---------|:----:|:---------:|:---:|:----:|
| **Basic Load Testing** | ✅ | ✅ | ✅ | ✅ |
| **Authentication** | ✅ | ✅ | ✅ | ✅ |
| **Data Feeders** | ✅ | ✅ | ✅ | ✅ |
| **Retries** | ✅ | ❌ | ❌ | ✅ |
| **Protocol-Specific Metrics** | - | Messages sent/received, bytes | Events received, bytes | Calls, responses |
| **Dashboard Support** | ✅ | ✅ | ✅ | ✅ |
| **JSON Output** | ✅ | ✅ | ✅ | ✅ |
| **Rate Limiting** | ✅ | ✅ | ✅ | ✅ |
| **Arrival Models** | ✅ | ✅ | ✅ | ✅ |
| **Load Patterns** | ✅ | ✅ | ✅ | ✅ |
| **Multiple Endpoints** | ✅ | ✅ | ✅ | ✅ |

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

For more installation options including Docker and pre-built binaries, see [INSTALLATION.md](docs/INSTALLATION.md).

## Quick Start

Basic load test:

```bash
crankfire --target https://example.com --concurrency 10 --total 100
```

With live terminal dashboard:

```bash
crankfire --target https://api.example.com --concurrency 20 --rate 100 --duration 30s --dashboard
```

With rate limiting:

```bash
crankfire --target https://api.example.com --concurrency 20 --rate 100 --duration 30s
```

Using a config file:

```bash
crankfire --config loadtest.json
```

## Command-Line Options

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

### Advanced Workload Modeling

Describe multi-phase workloads, realistic arrival processes, and weighted endpoint mixes directly in configuration files. Phases execute on a single shared timeline, while global caps like `duration` and `total` still provide safety limits.

```yaml
target: https://api.example.com
concurrency: 40
load_patterns:
  - name: warmup
    type: ramp
    from_rps: 10
    to_rps: 400
    duration: 5m
  - name: soak
    type: step
    steps:
      - rps: 400
        duration: 10m
      - rps: 600
        duration: 10m
  - name: spike
    type: spike
    rps: 1000
    duration: 30s
arrival:
  model: poisson
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

Endpoints inherit the global target URL, method, headers, and body unless you override them per entry. Weights are relative (they do not need to sum to 100); Crankfire randomly selects an endpoint using those proportions. Arrival modeling defaults to uniform spacing but can be switched to Poisson either in config or via `--arrival-model`.

#### Arrival Models

Crankfire’s scheduler releases work according to the selected arrival model:

- `uniform` (default) spreads permits evenly over time, producing a steady request cadence that is ideal when you want predictable background load.
- `poisson` samples inter-arrival gaps from an exponential distribution to create natural clustering. Average RPS stays aligned with your plan, but individual requests arrive in bursts that better resemble independent users.

Both models honor global constraints such as `rate`, `total`, `duration`, and any load pattern phases. Switching models only changes when a worker is allowed to fire—not the total number of requests you configured.

### Headers

You can supply headers via repeatable `--header` flags or in config files.

CLI format uses `Key=Value` (no spaces around `=`):

```bash
crankfire --target https://api.example.com \
  --header "Authorization=Bearer token123" \
  --header "Content-Type=application/json" \
  --header "X-Trace-Id=req-42"
```

Behavior:
- Keys are canonicalized (e.g. `content-type` -> `Content-Type`).
- Empty values are allowed: `--header "X-Empty="`.
- Duplicate keys: last value wins.
- Headers from flags override values defined in config files.
- Newlines or control characters in keys/values are rejected.

Config file headers use standard JSON/YAML maps:

```json
"headers": {
  "Authorization": "Bearer token123",
  "X-Env": "prod"
}
```

```yaml
headers:
  Authorization: Bearer token123
  X-Env: prod
```

Merging rules when both config and flags specify the same key: the last CLI flag overrides the config value.

Examples (override):

```bash
crankfire --config base.json \
  --header "Authorization=Bearer override-token"
```

To clear or intentionally send an empty value:

```bash
crankfire --header "X-Debug="
```

If you accidentally use a colon format (`Key: Value`), it will be treated as the full key (including the colon). Always prefer `Key=Value`.


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
  "errors": {
    "*runner.HTTPError": 2,
    "*context.deadlineExceeded": 1
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
      "errors": {
        "*runner.HTTPError": 8
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
- **Error Breakdown**: List of errors by type with count
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

┌ Endpoints ─────────────────────────────────────────────────────┐
│ list-users  | 60.0% | RPS 240.0 | P99 120.3ms | Err 2          │
│ create-order| 40.0% | RPS 160.0 | P99 210.5ms | Err 12         │
└────────────────────────────────────────────────────────────────┘

┌ Error Breakdown ───────────────────────────────────────────────┐
│ *runner.HTTPError: 38                                          │
│ context.deadlineExceededError: 7                               │
└────────────────────────────────────────────────────────────────┘
```

### Keyboard Controls

- `q` or `Ctrl+C`: Exit the dashboard
- Terminal automatically resizes with window

**Note**: The dashboard and JSON output are mutually exclusive. Use `--dashboard` for interactive monitoring or `--json-output` for automation.

## Authentication

Crankfire provides built-in authentication helpers that automatically manage token acquisition, refresh, and header injection.

### Supported Auth Types

- **OAuth2 Client Credentials**: Automated token fetch and refresh using client ID and secret
- **OAuth2 Resource Owner Password**: Token flow with username and password
- **OIDC Implicit**: Static token injection for OpenID Connect implicit flow
- **OIDC Authorization Code**: Static token injection for OIDC auth code flow

### Basic Usage

Manual header injection (no automated refresh):

```bash
crankfire --target https://api.example.com \
  --header "Authorization=Bearer token123" \
  --total 100
```

### OAuth2 Client Credentials

Config file (YAML):

```yaml
target: https://api.example.com
auth:
  type: oauth2_client_credentials
  token_url: https://idp.example.com/oauth/token
  client_id: your-client-id
  client_secret: your-client-secret
  scopes:
    - read
    - write
  refresh_before_expiry: 30s
concurrency: 20
duration: 5m
```

### OAuth2 Resource Owner Password

Config file (JSON):

```json
{
  "target": "https://api.example.com",
  "auth": {
    "type": "oauth2_resource_owner",
    "token_url": "https://idp.example.com/oauth/token",
    "client_id": "client-id",
    "client_secret": "client-secret",
    "username": "user@example.com",
    "password": "userpass",
    "scopes": ["api"]
  },
  "concurrency": 10,
  "total": 1000
}
```

### OIDC Implicit/Auth Code with Static Token

For pre-configured or manually obtained tokens:

```yaml
target: https://api.example.com
auth:
  type: oidc_implicit
  static_token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
concurrency: 15
duration: 2m
```

### Auth Configurationtion

Authentication must be configured via config files (JSON/YAML). Auth settings are not available as CLI flags to avoid exposing secrets in shell history and process lists.

For quick testing without auth, use manual header injection:

```bash
crankfire --target https://api.example.com \
  --header "Authorization=Bearer token123" \
  --total 100
```

**Note**: Auth tokens are never persisted between runs. Each execution rehydrates credentials fresh. Use environment variables (`CRANKFIRE_AUTH_CLIENT_SECRET`, `CRANKFIRE_AUTH_PASSWORD`, `CRANKFIRE_AUTH_STATIC_TOKEN`) for secrets instead of hardcoding in config files.

## Data Feeders

Data feeders enable per-request data injection for realistic traffic patterns with varied payloads. Supports CSV and JSON formats with deterministic round-robin selection.

### Features

- **CSV/JSON Support**: Load data from structured files
- **Placeholder Substitution**: Inject fields into URLs, headers, and request bodies using `{{field_name}}` syntax
- **Deterministic Round-Robin**: Ensures predictable, sequential record selection across concurrent requests
- **Exhaustion Handling**: Test automatically aborts when feeder data is exhausted (no rewind)

### CSV Feeder Example

Data file (`users.csv`):

```csv
user_id,email,name,role
101,alice@example.com,Alice Smith,admin
102,bob@example.com,Bob Johnson,user
103,charlie@example.com,Charlie Brown,user
```

Config file (`loadtest.yml`):

```yaml
target: https://api.example.com/users/{{user_id}}
method: GET
concurrency: 10
rate: 50
duration: 30s

feeder:
  path: ./users.csv
  type: csv

headers:
  X-User-Email: "{{email}}"
  X-User-Role: "{{role}}"
```

CLI usage:

```bash
crankfire --config loadtest.yml \
  --feeder-path ./users.csv \
  --feeder-type csv
```

### JSON Feeder Example

Data file (`products.json`):

```json
[
  {"product_id": "p1001", "name": "Widget", "price": "49.99"},
  {"product_id": "p1002", "name": "Gadget", "price": "79.99"}
]
```

Config file with request body placeholders:

```json
{
  "target": "https://api.example.com/products",
  "method": "POST",
  "concurrency": 5,
  "rate": 20,
  "duration": "1m",
  "feeder": {
    "path": "./products.json",
    "type": "json"
  },
  "headers": {
    "Content-Type": "application/json"
  },
  "body": "{\"id\": \"{{product_id}}\", \"name\": \"{{name}}\", \"price\": {{price}}}"
}
```

### Placeholder Syntax

Use `{{field_name}}` in any of these locations:

- **Target URL**: `https://api.example.com/users/{{user_id}}`
- **Query Parameters**: `https://api.example.com/search?q={{query}}&limit={{limit}}`
- **Header Values**: `X-User-ID: {{user_id}}`
- **Request Body**: `{"user": "{{email}}", "action": "{{action}}"}`

If a field is missing from the record, the placeholder is left unchanged.

### Exhaustion Behavior

When the feeder runs out of records, the test **aborts immediately** with an error. This ensures data completeness and prevents requests without proper data injection.

To test with limited data:

```bash
# Will stop after 5 requests (CSV has 5 rows)
crankfire --config loadtest.yml --total 1000
```

## WebSocket Testing

Crankfire supports WebSocket protocol testing with connection management, message sending, and metrics tracking.

### Basic WebSocket Test

```bash
crankfire --protocol websocket \
  --target ws://localhost:8080/ws \
  --ws-messages '{"type":"ping"}' \
  --ws-messages '{"type":"subscribe","channel":"updates"}' \
  --ws-message-interval 1s \
  --concurrency 10 \
  --duration 30s
```

### WebSocket Config File

```yaml
protocol: websocket
target: ws://localhost:8080/ws
concurrency: 10
rate: 50
duration: 1m

websocket:
  messages:
    - '{"type": "ping"}'
    - '{"type": "subscribe", "channel": "updates"}'
    - '{"type": "get_data", "id": 123}'
  message_interval: 1s      # Delay between messages
  receive_timeout: 10s      # Timeout for receiving responses
  handshake_timeout: 30s    # WebSocket handshake timeout

headers:
  Authorization: "Bearer token123"
  X-Client-ID: "loadtest-client"
```

### WebSocket Metrics

WebSocket tests track:
- **Connection Duration**: Total time connected
- **Messages Sent/Received**: Count of exchanged messages
- **Bytes Sent/Received**: Total data transferred
- **Errors**: Connection failures and message errors

## Server-Sent Events (SSE) Testing

Test SSE endpoints with event streaming and connection duration tracking.

### Basic SSE Test

```bash
crankfire --protocol sse \
  --target http://localhost:8080/events \
  --sse-read-timeout 30s \
  --sse-max-events 100 \
  --concurrency 20 \
  --duration 2m
```

### SSE Config File

```json
{
  "protocol": "sse",
  "target": "http://localhost:8080/events",
  "concurrency": 20,
  "rate": 100,
  "duration": "2m",
  "sse": {
    "read_timeout": "30s",
    "max_events": 100
  },
  "headers": {
    "Accept": "text/event-stream",
    "Authorization": "Bearer token123"
  }
}
```

### SSE Metrics

SSE tests track:
- **Connection Duration**: Total streaming time
- **Events Received**: Count of SSE events
- **Bytes Received**: Total data streamed
- **Errors**: Connection failures and read errors

## gRPC Testing

Test gRPC services with support for proto file compilation, message templates, and metadata injection.

### Basic gRPC Test

```bash
crankfire --protocol grpc \
  --target localhost:50051 \
  --grpc-proto-file ./hello.proto \
  --grpc-service helloworld.Greeter \
  --grpc-method SayHello \
  --grpc-message '{"name":"LoadTest"}' \
  --concurrency 10 \
  --duration 30s
```

### gRPC Config File

```yaml
protocol: grpc
target: localhost:50051
concurrency: 20
rate: 100
duration: 2m

grpc:
  proto_file: ./api/service.proto
  service: myapp.UserService
  method: GetUser
  message: '{"user_id": "{{user_id}}"}'
  timeout: 5s
  tls: true
  insecure: false
  metadata:
    authorization: "Bearer token123"
    x-request-id: "loadtest-{{request_id}}"

feeder:
  path: ./users.csv
  type: csv
```

### gRPC Metrics

gRPC tests track:
- **Calls**: Total gRPC calls made
- **Responses**: Successful responses received
- **Errors**: Call failures and timeouts
- **Latency**: Per-call latency distribution

### gRPC with TLS

```yaml
protocol: grpc
target: api.example.com:443
grpc:
  proto_file: ./service.proto
  service: api.Service
  method: Process
  message: '{"data": "test"}'
  tls: true
  insecure: false  # Verify TLS certificates
```

### gRPC with Data Injection

Combine feeders with gRPC message templates:

```yaml
protocol: grpc
target: localhost:50051
grpc:
  proto_file: ./orders.proto
  service: orders.OrderService
  method: CreateOrder
  message: |
    {
      "order_id": "{{order_id}}",
      "customer_email": "{{email}}",
      "amount": {{amount}},
      "items": [
        {"product_id": "{{product_id}}", "quantity": {{quantity}}}
      ]
    }
feeder:
  path: ./orders.csv
  type: csv
```

## Advanced Usage

### POST with body from file

```bash
crankfire --target https://api.example.com/data \
  --method POST \
  --header "Content-Type: application/json" \
  --body-file request.json \
  --concurrency 10 \
  --total 1000
```

### Rate-limited test

```bash
crankfire --target https://api.example.com \
  --rate 50 \
  --duration 2m \
  --concurrency 5
```

### With adaptive retries and custom timeout

```bash
crankfire --target https://flaky-api.example.com \
  --timeout 10s \
  --retries 3 \
  --total 500
```

## Use Cases

- **Performance Testing**: Measure API response times and throughput
- **Load Testing**: Simulate concurrent users and high traffic
- **Stress Testing**: Find breaking points (histogram percentile accuracy)
- **CI/CD Integration**: Automated performance regression testing with JSON output
- **Failure Analysis**: Error breakdown object highlights dominant failure classes

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
- Be aware that rates exceeding 1000 RPS or concurrency above 500 can cause service disruption

Misuse of this tool may result in service disruptions, account termination, legal action, or criminal charges. The authors and contributors are not responsible for misuse of this software.

## License

MIT
