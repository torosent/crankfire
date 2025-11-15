# Crankfire

An optimized command-line load testing tool written in Go for HTTP endpoints.

## Features

- **Live Terminal Dashboard**: Real-time metrics with sparkline latency graphs, RPS counters, and error breakdown
- **Flexible Configuration**: Command-line flags, JSON, or YAML config files
- **Concurrency Control**: Configurable number of parallel workers
- **Rate Limiting**: Control requests per second
- **Smart Metrics (Histogram)**: High accuracy Min/Max/Mean + P50/P90/P99 using HDR Histogram
- **Adaptive Retry Logic**: Conditional retries with exponential backoff + jitter
- **Multiple Output Formats**: Human-readable or structured JSON (includes error breakdown)
- **Real-time Progress**: Lightweight periodic CLI updates
- **Advanced Workload Modeling**: Ramp/step/spike phases, Poisson arrivals, and weighted endpoint mixes

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

## License

MIT
