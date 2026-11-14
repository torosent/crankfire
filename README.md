# Crankfire

[![GitHub release (latest by date)](https://img.shields.io/github/v/release/torosent/crankfire)](https://github.com/torosent/crankfire/releases)
[![Go Build](https://github.com/torosent/crankfire/actions/workflows/release.yml/badge.svg)](https://github.com/torosent/crankfire/actions/workflows/release.yml)
[![codecov](https://codecov.io/gh/torosent/crankfire/branch/main/graph/badge.svg)](https://codecov.io/gh/torosent/crankfire)

Crankfire is a command-line tool for load testing HTTP, WebSocket, SSE and gRPC services.

You describe the traffic you want, using flags or a YAML/JSON file, and Crankfire sends it to your service. It measures response times and errors, and shows the results in the terminal, as JSON, or as an HTML report. Each run tests one protocol. Crankfire is a single program: you do not need a cluster, a server or a web UI.

**Documentation:** https://torosent.github.io/crankfire/

## Why use Crankfire?

Use Crankfire when a simple `curl` loop is not enough, but a large load-testing platform is too much.

- **Four protocols:** HTTP, WebSocket, SSE and gRPC all use the same configuration format and the same reports.
- **Realistic traffic:** increase the load step by step, hold it steady, or add short spikes. Requests can arrive at an even rate or at random intervals (Poisson).
- **Accurate numbers:** latency percentiles (P50, P90, P95, P99), statistics for each endpoint, and error counts grouped by status code.
- **Live view or JSON:** watch the test in your terminal, or save the results as JSON for scripts and CI.
- **Authentication and test data:** OAuth2 and static tokens, plus values read from CSV or JSON files.
- **Request chaining:** take a value from one response (for example a token) and use it in later requests.
- **HAR import:** record a session in your browser and replay it as a load test.
- **Saved tests and suites:** keep tests in a terminal UI, group them into suites, compare runs, and run them on a schedule.
- **One binary:** written in Go, with no runtime dependencies.

## Feature support

| Feature | HTTP | WebSocket | SSE | gRPC |
|---------|:----:|:---------:|:---:|:----:|
| Basic load testing | ✅ | ✅ | ✅ | ✅ |
| Rate limits, arrival models, load patterns | ✅ | ✅ | ✅ | ✅ |
| Authentication | ✅ | ✅ | ✅ | ✅ |
| Data feeders | ✅ | ✅ | ✅ | ✅ |
| Request chaining | ✅ | — | — | — |
| Several weighted endpoints | ✅ | — | — | — |
| HAR import | ✅ | — | — | — |
| Retries | ✅ | — | — | — |
| Thresholds (pass/fail rules) | ✅ | ✅ | ✅ | ✅ |
| Live dashboard, JSON and HTML reports | ✅ | ✅ | ✅ | ✅ |
| OpenTelemetry tracing | ✅ | ✅ | ✅ | ✅ |
| Extra protocol metrics | — | Messages and bytes sent/received | Events and bytes received | Messages and bytes sent/received, status code |

## What you can use it for

- **Performance testing:** measure response times and throughput.
- **Load testing:** simulate many users at the same time.
- **Stress testing:** find the point where your service starts to fail.
- **CI/CD checks:** fail a build when latency or errors get worse.
- **Failure analysis:** see which status codes and errors appear under load.

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

### Go

```bash
go install github.com/torosent/crankfire/cmd/crankfire@latest
```

### Release archives

Download `crankfire_<os>_<arch>.tar.gz` (or `.zip` for Windows) from the [releases page](https://github.com/torosent/crankfire/releases).

### Build from source

```bash
git clone https://github.com/torosent/crankfire.git
cd crankfire
go build -o build/crankfire ./cmd/crankfire
```

## Quick start

Send 100 requests with 10 workers:

```bash
crankfire --target https://example.com --concurrency 10 --total 100
```

Send 100 requests per second for 30 seconds and watch the live dashboard:

```bash
crankfire \
  --target https://api.example.com \
  --concurrency 20 \
  --rate 100 \
  --duration 30s \
  --dashboard
```

Send POST requests with a JSON body:

```bash
crankfire \
  --target https://api.example.com/users \
  --method POST \
  --body '{"name":"crankfire"}' \
  --header "Content-Type=application/json" \
  --total 100
```

Use a config file:

```bash
crankfire --config loadtest.yaml
```

Save an HTML report:

```bash
crankfire --target https://example.com --total 1000 --html-output report.html
```

Replay a browser recording (HAR file):

```bash
crankfire --har recording.har --har-filter "host:api.example.com" --total 100
```

The [Getting Started guide](https://torosent.github.io/crankfire/getting-started.html) explains each step in more detail.

## Command-line options

Note that headers use `Key=Value`, not `Key: Value`.

| Flag | Description | Default |
|------|-------------|---------|
| `--target` | URL to test (`host:port` for gRPC). Not needed with `--har`. | — |
| `--method` | HTTP method, such as GET, POST, PUT or DELETE | GET |
| `--header` | Add an HTTP header as `Key=Value`. You can repeat this flag. | — |
| `--body` | Request body as text | — |
| `--body-file` | Read the request body from a file | — |
| `--concurrency`, `-c` | Number of workers that send requests in parallel | 1 |
| `--rate`, `-r` | Maximum requests per second (0 = no limit) | 0 |
| `--duration`, `-d` | How long the test runs, such as `30s` or `1m` | 0 (no limit) |
| `--total`, `-t` | Total number of requests | 0 (no limit) |
| `--timeout` | Timeout for each HTTP request | 30s |
| `--graceful-shutdown` | How long to wait for running requests when the test ends normally (negative = stop at once). `Ctrl+C` always stops them at once. | 5s |
| `--retries` | How many times to retry a failed HTTP request (status 429, 5xx or network errors; timeouts are not retried) | 0 |
| `--arrival-model` | How requests are spaced: `uniform` (even) or `poisson` (random) | uniform |
| `--dashboard` | Show the live terminal dashboard | false |
| `--json-output` | Print the results as JSON | false |
| `--html-output` | Write an HTML report to this file | — |
| `--log-errors` | Print each failed HTTP request to stderr | false |
| `--config` | Path to a YAML or JSON config file | — |
| `--har` | HAR file to import as endpoints | — |
| `--har-filter` | Which HAR entries to keep, such as `host:example.com` or `method:GET,POST` | — |
| `--feeder-path` | CSV or JSON file with test data | — |
| `--feeder-type` | Type of the feeder file: `csv` or `json` | — |
| `--protocol` | `http`, `websocket`, `sse` or `grpc` | http |
| `--ws-messages` | WebSocket message to send. You can repeat this flag. Put JSON messages (with quotes or commas) in the config file. | — |
| `--ws-message-interval` | Pause after each WebSocket message | 0 |
| `--ws-receive-timeout` | How long to read WebSocket replies (0 = do not read) | 0 |
| `--ws-handshake-timeout` | Timeout for the WebSocket handshake | 30s |
| `--sse-read-timeout` | How long to read SSE events in each request | 0 (stream ends after 30s) |
| `--sse-max-events` | Maximum SSE events per request (0 means 100) | 0 |
| `--grpc-proto-file` | `.proto` file that defines the service | — |
| `--grpc-service` | Full service name, such as `myapp.OrderService` | — |
| `--grpc-method` | Method name, such as `CreateOrder` | — |
| `--grpc-message` | Request message as JSON | — |
| `--grpc-metadata` | gRPC metadata as `key=value` | — |
| `--grpc-timeout` | Timeout for each gRPC call | none |
| `--grpc-tls` | Use TLS for gRPC | false |
| `--grpc-insecure` | With `--grpc-tls`, do not check the server certificate | false |
| `--threshold` | A pass/fail rule, such as `http_req_duration:p95 < 500`. You can repeat this flag. | — |
| `--tracing-endpoint` | OTLP collector address, such as `localhost:4317` | — |
| `--tracing-protocol` | OTLP transport: `grpc` or `http` | grpc |
| `--tracing-service-name` | Service name for the traces | crankfire |
| `--tracing-sample-rate` | Share of requests to trace, from 0.0 to 1.0. **Set it yourself**, because the current default is 0 (no traces). | 0 |
| `--tracing-insecure` | Connect to the collector without TLS | false |

> `--help` shows some defaults that only apply when you pass the flag. The table above shows the values that Crankfire really uses.

For every option and config key, see the [Configuration reference](https://torosent.github.io/crankfire/configuration.html).

## Thresholds (pass/fail rules for CI)

Thresholds let a test pass or fail automatically. This is useful in CI to catch slow or failing releases.

```bash
crankfire --target https://api.example.com \
  --concurrency 50 \
  --duration 1m \
  --threshold "http_req_duration:p95 < 500" \
  --threshold "http_req_failed:rate < 0.01" \
  --threshold "http_requests:rate > 100"
```

You can also put them in a config file:

```yaml
target: https://api.example.com
concurrency: 50
duration: 1m
thresholds:
  - "http_req_duration:p95 < 500"   # 95% of requests take less than 500 ms
  - "http_req_failed:rate < 0.01"   # less than 1% of requests fail
  - "http_requests:rate > 100"      # more than 100 requests per second
```

**Exit code:** Crankfire exits with `0` only when all thresholds pass **and** no request failed. Otherwise it exits with `1`. See the [Thresholds guide](https://torosent.github.io/crankfire/thresholds.html) for all metrics and CI examples.

## Config file

The same settings can be written as JSON or YAML.

JSON:

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
  "graceful_shutdown": "5s",
  "retries": 3
}
```

YAML:

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
graceful_shutdown: 5s
retries: 3
```

Flags that you pass on the command line override the values in the file.

## Examples

### 1. Warm up, then hold the load

Increase the load from 10 to 100 requests per second over 30 seconds, then keep it at 100 for 2 minutes.

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

### 2. Several endpoints with weights

Send about 80% of requests to one endpoint and 20% to another, like real users who browse more than they buy.

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

### 3. Test data from a CSV file

Each request takes the next row of the file. When the file ends, Crankfire starts again from the first row.

**orders.csv**

```csv
product_id,quantity
101,2
102,1
103,5
```

**config.yaml**

```yaml
target: https://api.example.com/orders
method: POST
feeder:
  type: csv
  path: ./orders.csv
body: '{"product_id": "{{product_id}}", "quantity": {{quantity}}}'
```

### 4. Request chaining

Take values from one response and use them in later requests, for example log in and then use the token.

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
    path: /users/{{user_id|me}}
    headers:
      Authorization: "Bearer {{auth_token}}"
```

Each worker keeps its own variables. Crankfire picks endpoints at random by weight, so a worker may call `get-profile` before `login`. That is why the example uses a default value (`{{user_id|me}}`). The [Request Chaining guide](https://torosent.github.io/crankfire/request-chaining.html) explains this in detail.

### 5. gRPC

Load test a gRPC service using its `.proto` file. Use the full service name, including the package.

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

### 6. OpenTelemetry tracing

Crankfire can create a trace span for each request and send the trace context to your service. You can then find the load-test requests in your tracing tool.

```bash
crankfire --target https://api.example.com \
  --concurrency 20 \
  --duration 1m \
  --tracing-endpoint localhost:4317 \
  --tracing-insecure \
  --tracing-sample-rate 1.0
```

Or in a config file:

```yaml
target: https://api.example.com
concurrency: 20
duration: 1m
tracing:
  endpoint: localhost:4317
  protocol: grpc
  service_name: my-load-test
  sample_rate: 0.1
  insecure: true
```

Tracing starts only when you set an endpoint. Crankfire reads `OTEL_SERVICE_NAME`, and the exporter reads the standard `OTEL_EXPORTER_OTLP_*` variables (for example, headers). The [Tracing guide](https://torosent.github.io/crankfire/tracing.html) has setups for Jaeger, an OpenTelemetry Collector and Honeycomb.

## Output

### Text report

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
  P95:             81ms
  P99:             112ms

Status Buckets:
  HTTP 404: 1
  HTTP 500: 1
```

"Status Buckets" lists failed requests only, grouped by status code.

### JSON report

```bash
crankfire --config loadtest.yaml --json-output
```

```json
{
  "total": 1000,
  "successes": 992,
  "failures": 8,
  "requests_per_sec": 97.6,
  "min_latency_ms": 12.4,
  "max_latency_ms": 156.8,
  "mean_latency_ms": 45.2,
  "p50_latency_ms": 42.1,
  "p90_latency_ms": 68.3,
  "p95_latency_ms": 81.0,
  "p99_latency_ms": 112.7,
  "status_buckets": {
    "http": { "503": 8 }
  },
  "duration_ms": 10245.3,
  "endpoints": {
    "list-products": {
      "total": 800,
      "successes": 800,
      "failures": 0,
      "requests_per_sec": 78.1,
      "p99_latency_ms": 95.1
    },
    "create-order": {
      "total": 200,
      "successes": 192,
      "failures": 8,
      "requests_per_sec": 19.5,
      "p99_latency_ms": 180.4,
      "status_buckets": {
        "http": { "503": 8 }
      }
    }
  }
}
```

The endpoint entries are shortened here. In the real output, each endpoint has the same latency fields as the top level. The `endpoints` key appears only when you use `endpoints` or a HAR file.

### HTML report

```bash
crankfire --target https://example.com --total 1000 --html-output report.html
```

The report is a single HTML file. It contains:

- Summary cards with the main numbers
- Charts of requests per second and latency over time
- A table of latency percentiles
- Threshold results
- Results for each endpoint

The charts load a JavaScript library from a CDN, so they need an internet connection.

<img width="1455" height="858" alt="HTML report: summary and charts" src="https://github.com/user-attachments/assets/15837533-7e60-4553-9104-4bfdf16b9d79" />

<img width="1455" height="858" alt="HTML report: latency and endpoint tables" src="https://github.com/user-attachments/assets/f8b81f9a-0b16-41b7-9654-49cc47ac9af4" />

## Live dashboard

Add `--dashboard` to watch the test while it runs:

```bash
crankfire --target https://api.example.com \
  --concurrency 20 \
  --rate 100 \
  --duration 60s \
  --dashboard
```

The dashboard shows:

- Elapsed time, total requests and success rate
- Current requests per second, compared with your target rate
- A latency chart and latency percentiles
- Progress through your load patterns
- Results for each endpoint
- Failed requests grouped by status code

Press `q`, `Esc` or `Ctrl+C` to stop the test early. Requests that are still running are cancelled and counted as failures, so a stopped test usually exits with code `1`. You cannot use `--dashboard` and `--json-output` together.

<img width="1018" height="696" alt="Live terminal dashboard" src="https://github.com/user-attachments/assets/4f2a30f1-aed7-4a38-b8bf-e37d37e43611" />

## Terminal UI

`crankfire tui` opens a terminal app where you can save load tests ("sessions"), run them, and look at past results.

```bash
crankfire tui [--data-dir <path>]
```

Crankfire stores sessions and results in `~/.crankfire/`. To use another folder, set `$CRANKFIRE_DATA_DIR` or pass `--data-dir`. Each run creates a folder `runs/<session-id>/<timestamp>/` with `run.json`, `result.json` and `report.html`.

Main keys:

- **Session list:** `n` new, `e` edit, `i` import, `r` run, `h` history, `d` delete, `Enter` details, `s` sets, `/` filter by tag, `q` quit
- **Edit screen:** `Tab`/`Shift+Tab` move between fields, `F2` switch to YAML, `Ctrl+S` save, `Esc` cancel
- **Run screen:** `q`, `Esc` or `Ctrl+C` stop the run
- **History:** `o` open the HTML report, `Esc` back

The [TUI guide](https://torosent.github.io/crankfire/tui.html) explains every screen.

## Sets (test suites)

A set groups saved sessions into stages. Stages run one after another, and the tests inside a stage run at the same time. You can add thresholds, compare two runs, and run sets on a schedule with `crankfire daemon`.

```bash
crankfire set run <set-id> --html set-report.html
crankfire set diff <run-a> <run-b>
crankfire daemon
```

See the [Sets guide](https://torosent.github.io/crankfire/sets.html).

## Responsible use

⚠️ **Important:** Crankfire can send a lot of traffic. Use it with care.

**You must:**

- Only test systems that you own, or that you have written permission to test.
- Check that you have permission before every test.
- Know that load testing without permission can break terms of service, usage policies or laws.
- Think about the effect on production systems and on third-party services.

**Keep secrets safe:**

- Never commit passwords, tokens or keys to version control.
- Put secrets in environment variables: `CRANKFIRE_AUTH_CLIENT_SECRET`, `CRANKFIRE_AUTH_PASSWORD`, `CRANKFIRE_AUTH_STATIC_TOKEN`. Crankfire uses them when the matching field in the `auth` block of the config file is empty.
- Do not put secrets in command-line flags. Other users can see them in the shell history and the process list.
- Use short-lived tokens with as few permissions as possible.
- Use `oauth2_client_credentials` instead of the older `oauth2_resource_owner` (password) flow.
- For gRPC in production, keep certificate checks on (`insecure: false`).

**Increase the load slowly:**

- Start with a low rate and few workers.
- Increase the load step by step and watch the health of the system you test.

Misuse can cause outages, closed accounts, legal action or criminal charges. The authors and contributors are not responsible for misuse of this software.

## License

MIT
