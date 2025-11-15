# Usage Examples

This document provides practical examples for using Crankfire in various scenarios.

## Basic Examples

### Simple GET Request

Test a simple endpoint with 100 requests:

```bash
crankfire --target https://httpbin.org/get --total 100
```

### Concurrent Load Test

Run 1000 requests with 50 concurrent workers:

```bash
crankfire --target https://httpbin.org/get \
  --concurrency 50 \
  --total 1000
```

### Duration-Based Test

Run for 30 seconds with 10 workers:

```bash
crankfire --target https://httpbin.org/get \
  --concurrency 10 \
  --duration 30s
```

## POST Requests

### POST with Inline Body

```bash
crankfire --target https://httpbin.org/post \
  --method POST \
  --header "Content-Type=application/json" \
  --body '{"user":"test","action":"create"}' \
  --total 50
```

### POST with Body from File

Create `payload.json`:
```json
{
  "user": "test",
  "email": "test@example.com"
}
```

Run test:
```bash
crankfire --target https://httpbin.org/post \
  --method POST \
  --header "Content-Type=application/json" \
  --body-file payload.json \
  --concurrency 10 \
  --total 100
```

## Rate Limiting

### Fixed Rate Test

100 requests per second for 1 minute:

```bash
crankfire --target https://api.example.com \
  --rate 100 \
  --duration 1m \
  --concurrency 20
```

### Gradually Increasing Load

Start with low rate, then increase:

```bash
# Phase 1: 10 req/s
crankfire --target https://api.example.com --rate 10 --duration 30s

# Phase 2: 50 req/s
crankfire --target https://api.example.com --rate 50 --duration 30s

# Phase 3: 100 req/s
crankfire --target https://api.example.com --rate 100 --duration 30s
```

## Advanced Workload Modeling

Describe coordinated phases, arrival distributions, and weighted endpoints in a single config file instead of scripting separate runs.

```yaml
target: https://api.example.com
concurrency: 40
load_patterns:
  - name: warmup
    type: ramp
    from_rps: 20
    to_rps: 400
    duration: 5m
  - name: soak
    type: step
    steps:
      - rps: 400
        duration: 8m
      - rps: 600
        duration: 4m
  - name: spike
    type: spike
    rps: 1200
    duration: 20s
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
    body: '{"status":"new"}'
```

- **Load patterns**: `ramp`, `step`, and `spike` share a single timeline. Each step specifies its own duration; ramps interpolate between `from_rps` and `to_rps` across the provided duration.
- **Arrival models**: Defaults to uniform pacing. Switch to Poisson globally with `arrival:
  model: poisson` or `--arrival-model poisson` for more realistic inter-arrival gaps.
- **Endpoints**: Define multiple endpoints with relative `weight` values. Each endpoint inherits the global target URL, method, headers, and body unless explicitly overridden.

Endpoint weighting automatically feeds the collector. The JSON report, progress ticker, and dashboard expose per-endpoint totals, RPS, and latency percentiles so you can compare how each route behaves during the same run.

## Configuration Files

### JSON Configuration

Create `loadtest.json`:
```json
{
  "target": "https://api.example.com/v1/users",
  "method": "GET",
  "headers": {
    "Authorization": "Bearer your-token-here",
    "Accept": "application/json"
  },
  "concurrency": 25,
  "rate": 50,
  "duration": "2m",
  "timeout": "10s",
  "retries": 2
}
```

Run:
```bash
crankfire --config loadtest.json
```

### YAML Configuration

Create `loadtest.yaml`:
```yaml
target: https://api.example.com/v1/users
method: POST
headers:
  Content-Type: application/json
  Authorization: Bearer your-token-here
body: '{"name":"John Doe","email":"john@example.com"}'
concurrency: 30
rate: 75
duration: 5m
timeout: 15s
retries: 3
```

Run:
```bash
crankfire --config loadtest.yaml
```

### Override Config with Flags

Start with config but override specific values:

```bash
crankfire --config loadtest.json \
  --concurrency 100 \
  --duration 10m
```

## Authentication

### Bearer Token

```bash
crankfire --target https://api.example.com/protected \
  --header "Authorization=Bearer eyJhbGc..." \
  --total 100
```

### API Key

```bash
crankfire --target https://api.example.com/data \
  --header "X-API-Key=your-api-key" \
  --total 100
```

### Basic Auth

```bash
# Use base64 encoded credentials
crankfire --target https://api.example.com/admin \
  --header "Authorization=Basic dXNlcjpwYXNz" \
  --total 50
```

## Reliability Testing

### With Retries

Test flaky endpoints with automatic retries:

```bash
crankfire --target https://flaky-api.example.com \
  --retries 5 \
  --timeout 5s \
  --total 200
```

### Long Timeout for Slow APIs

```bash
crankfire --target https://slow-api.example.com \
  --timeout 60s \
  --concurrency 5 \
  --total 50
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Performance Test
on: [push]
jobs:
  load-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v3
        with:
          go-version: '1.21'
      - name: Install crankfire
        run: go install github.com/torosent/crankfire/cmd/crankfire@latest
      - name: Run load test
        run: |
          crankfire --target ${{ secrets.API_URL }} \
            --header "Authorization: Bearer ${{ secrets.API_TOKEN }}" \
            --total 1000 \
            --concurrency 50 \
            --json-output > results.json
      - name: Check performance
        run: |
          p99=$(jq -r '.p99_latency_ms' results.json)
          if (( $(echo "$p99 > 200" | bc -l) )); then
            echo "P99 latency too high: ${p99}ms"
            exit 1
          fi
```

### JSON Output Processing

Extract metrics with jq:

```bash
crankfire --target https://api.example.com \
  --total 1000 \
  --json-output | jq '{
    total: .total,
    success_rate: (.successes / .total * 100),
    avg_latency: .mean_latency_ms,
    p99_latency: .p99_latency_ms
  }'
```

## Monitoring and Debugging

### Verbose Failure Logging

Non-JSON mode logs errors to stderr:

```bash
crankfire --target https://api.example.com \
  --total 100 \
  2> errors.log
```

### Track Progress in Real-time

Progress updates appear on stderr every second:

```bash
crankfire --target https://api.example.com \
  --duration 5m \
  --concurrency 50
# Shows: Requests: 1234 | Successes: 1230 | Failures: 4 | RPS: 41.2
```

## Performance Benchmarking

### Compare Two Endpoints

```bash
# Test endpoint A
crankfire --target https://api-a.example.com/users \
  --total 1000 \
  --json-output > results-a.json

# Test endpoint B
crankfire --target https://api-b.example.com/users \
  --total 1000 \
  --json-output > results-b.json

# Compare
jq -s '{a: .[0].mean_latency_ms, b: .[1].mean_latency_ms}' \
  results-a.json results-b.json
```

### Stress Test to Find Limits

Gradually increase concurrency:

```bash
for c in 10 25 50 100 200 500; do
  echo "Testing with concurrency: $c"
  crankfire --target https://api.example.com \
    --concurrency $c \
    --total 1000 \
    --json-output | jq '{concurrency: '$c', rps: .requests_per_sec, p99: .p99_latency_ms}'
done
```

## Tips

1. **Start Small**: Begin with low concurrency and gradually increase
2. **Monitor Target**: Watch your target system's resources during tests
3. **Use Rate Limiting**: Prevent overwhelming your own or external APIs
4. **Enable Retries**: For flaky networks or services
5. **JSON Output**: Pipe to `jq` for automated analysis
6. **Config Files**: Reuse test configurations across environments

## Headers

Crankfire supports headers via repeatable `--header` flags or config file maps.

CLI format (use `Key=Value`):

```bash
crankfire --target https://api.example.com \
  --header "Authorization=Bearer token123" \
  --header "Content-Type=application/json" \
  --header "X-Trace-Id=req-99"
```

Config examples:

```json
"headers": {"Authorization": "Bearer token123", "X-Env": "prod"}
```

```yaml
headers:
  Authorization: Bearer token123
  X-Env: prod
```

Rules:
- Keys canonicalized (e.g. `x-custom-id` -> `X-Custom-Id`).
- Empty values allowed: `--header "X-Empty="`.
- Last duplicate wins.
- CLI flags override config values.
- Invalid newline characters in keys/values are rejected.

Override example:

```bash
crankfire --config base.yaml \
  --header "Authorization=Bearer override"
```

Sending empty value:

```bash
crankfire --header "X-Feature-Flag="
```

Avoid colon syntax (`Key: Value`). Always use `Key=Value`.
