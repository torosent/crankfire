# Thresholds

Thresholds allow you to define performance criteria that must be met for a load test to pass. This is essential for CI/CD integration where you want automated tests to fail if performance degrades.

## Overview

When a threshold fails, `crankfire` exits with a non-zero exit code, making it easy to integrate into continuous integration pipelines. Thresholds are evaluated after the test completes and results are displayed alongside the regular metrics.

## Threshold Format

Thresholds follow this format:

```
metric:aggregate operator value
```

For example:
- `http_req_duration:p95 < 500` - 95th percentile latency must be under 500ms
- `http_req_failed:rate < 0.01` - Failure rate must be under 1%
- `http_requests:rate > 100` - Must achieve at least 100 requests per second

## Supported Metrics

### http_req_duration

Measures request latency in milliseconds.

**Supported aggregates:**
- `p50` - 50th percentile (median)
- `p90` - 90th percentile
- `p95` - 95th percentile
- `p99` - 99th percentile
- `avg` or `mean` - Average latency
- `min` - Minimum latency
- `max` - Maximum latency

**Examples:**
```yaml
thresholds:
  - "http_req_duration:p95 < 500"
  - "http_req_duration:p99 < 1000"
  - "http_req_duration:avg < 200"
  - "http_req_duration:max < 2000"
```

### http_req_failed

Measures request failures.

**Supported aggregates:**
- `rate` - Failure rate as a decimal (0.0 to 1.0)
- `count` - Total number of failures

**Examples:**
```yaml
thresholds:
  - "http_req_failed:rate < 0.01"    # Less than 1% failures
  - "http_req_failed:count < 10"     # Less than 10 total failures
```

### http_requests

Measures request throughput and count.

**Supported aggregates:**
- `rate` - Requests per second
- `count` - Total number of requests

**Examples:**
```yaml
thresholds:
  - "http_requests:rate > 100"       # At least 100 RPS
  - "http_requests:count > 1000"     # At least 1000 total requests
```

## Supported Operators

- `<` - Less than
- `<=` - Less than or equal to
- `>` - Greater than
- `>=` - Greater than or equal to
- `==` - Equal to

## Configuration

### YAML Configuration

```yaml
target: https://api.example.com
method: GET
concurrency: 50
duration: 1m
thresholds:
  - "http_req_duration:p95 < 500"
  - "http_req_duration:p99 < 1000"
  - "http_req_failed:rate < 0.01"
  - "http_requests:rate > 100"
```

### JSON Configuration

```json
{
  "target": "https://api.example.com",
  "method": "GET",
  "concurrency": 50,
  "duration": "1m",
  "thresholds": [
    "http_req_duration:p95 < 500",
    "http_req_duration:p99 < 1000",
    "http_req_failed:rate < 0.01",
    "http_requests:rate > 100"
  ]
}
```

### Command Line

Use the `--threshold` flag (repeatable):

```bash
crankfire --target https://api.example.com \
  --concurrency 50 \
  --duration 1m \
  --threshold "http_req_duration:p95 < 500" \
  --threshold "http_req_failed:rate < 0.01"
```

## Output

### Human-Readable Output

When thresholds are defined, the output includes a "Thresholds" section:

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

Thresholds:
  ✓ http_req_duration:p95 < 500: 90.50 < 500.00
  ✓ http_req_duration:p99 < 1000: 112.00 < 1000.00
  ✓ http_req_failed:rate < 0.01: 0.00 < 0.01
  ✓ http_requests:rate > 50: 98.04 > 50.00

Threshold Summary: 4/4 passed
```

A ✓ indicates a passing threshold, while a ✗ indicates a failure.

### JSON Output

With `--json-output`, thresholds are included in the JSON response:

```json
{
  "total": 1000,
  "successes": 998,
  "failures": 2,
  "requests_per_sec": 98.04,
  "p95_latency_ms": 90.50,
  "p99_latency_ms": 112.00,
  "thresholds": {
    "total": 4,
    "passed": 4,
    "failed": 0,
    "results": [
      {
        "threshold": "http_req_duration:p95 < 500",
        "metric": "http_req_duration",
        "aggregate": "p95",
        "operator": "<",
        "expected": 500,
        "actual": 90.50,
        "pass": true
      },
      {
        "threshold": "http_req_duration:p99 < 1000",
        "metric": "http_req_duration",
        "aggregate": "p99",
        "operator": "<",
        "expected": 1000,
        "actual": 112.00,
        "pass": true
      },
      {
        "threshold": "http_req_failed:rate < 0.01",
        "metric": "http_req_failed",
        "aggregate": "rate",
        "operator": "<",
        "expected": 0.01,
        "actual": 0.002,
        "pass": true
      },
      {
        "threshold": "http_requests:rate > 50",
        "metric": "http_requests",
        "aggregate": "rate",
        "operator": ">",
        "expected": 50,
        "actual": 98.04,
        "pass": true
      }
    ]
  }
}
```

## Exit Codes

- **Exit code 0**: All thresholds passed (or no thresholds defined)
- **Exit code 1**: One or more thresholds failed, or other error occurred

This makes it easy to use in CI/CD pipelines:

```bash
#!/bin/bash
crankfire --config loadtest.yaml
if [ $? -eq 0 ]; then
  echo "✓ Performance test passed"
else
  echo "✗ Performance test failed"
  exit 1
fi
```

## CI/CD Integration Examples

### GitHub Actions

```yaml
name: Performance Tests

on: [push, pull_request]

jobs:
  performance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Install crankfire
        run: |
          curl -L https://github.com/torosent/crankfire/releases/latest/download/crankfire-linux-amd64 -o crankfire
          chmod +x crankfire
      
      - name: Run performance test
        run: |
          ./crankfire --config perf-test.yaml
          
      - name: Upload results
        if: always()
        uses: actions/upload-artifact@v3
        with:
          name: performance-results
          path: results.json
```

### GitLab CI

```yaml
performance_test:
  stage: test
  image: alpine:latest
  before_script:
    - apk add --no-cache curl
    - curl -L https://github.com/torosent/crankfire/releases/latest/download/crankfire-linux-amd64 -o crankfire
    - chmod +x crankfire
  script:
    - ./crankfire --config perf-test.yaml --json-output > results.json
  artifacts:
    when: always
    paths:
      - results.json
    reports:
      junit: results.json
```

### Jenkins

```groovy
pipeline {
    agent any
    
    stages {
        stage('Performance Test') {
            steps {
                sh '''
                    curl -L https://github.com/torosent/crankfire/releases/latest/download/crankfire-linux-amd64 -o crankfire
                    chmod +x crankfire
                    ./crankfire --config perf-test.yaml --json-output > results.json
                '''
            }
            post {
                always {
                    archiveArtifacts artifacts: 'results.json', fingerprint: true
                }
            }
        }
    }
}
```

## Best Practices

### Start Conservative

Begin with generous thresholds and tighten them as you understand your system's baseline:

```yaml
thresholds:
  # Start with generous thresholds
  - "http_req_duration:p95 < 2000"
  - "http_req_failed:rate < 0.05"
```

After establishing baseline performance, tighten gradually:

```yaml
thresholds:
  # Tightened after establishing baseline
  - "http_req_duration:p95 < 500"
  - "http_req_failed:rate < 0.01"
```

### Use Multiple Percentiles

Don't rely on just one percentile. Use a combination to catch different types of degradation:

```yaml
thresholds:
  - "http_req_duration:p50 < 100"   # Median should be fast
  - "http_req_duration:p90 < 200"   # Most requests should be fast
  - "http_req_duration:p99 < 500"   # Even outliers should be reasonable
```

### Combine Different Metrics

Use both latency and failure rate thresholds:

```yaml
thresholds:
  - "http_req_duration:p95 < 500"
  - "http_req_failed:rate < 0.01"
  - "http_requests:rate > 100"
```

### Environment-Specific Thresholds

Use different threshold files for different environments:

```bash
# Development - more lenient
crankfire --config perf-test.yaml --threshold "http_req_duration:p95 < 1000"

# Production - strict
crankfire --config perf-test.yaml --threshold "http_req_duration:p95 < 200"
```

## Troubleshooting

### Threshold Not Evaluating

If a threshold doesn't seem to be evaluating:

1. **Check the format**: Ensure it follows `metric:aggregate operator value`
2. **Verify the metric name**: Must be exactly `http_req_duration`, `http_req_failed`, or `http_requests`
3. **Check the aggregate**: Must be one of the supported aggregates for that metric
4. **Validate the operator**: Must be `<`, `<=`, `>`, `>=`, or `==`

### Flaky Thresholds

If thresholds fail inconsistently:

1. **Increase test duration** to get more stable statistics
2. **Use higher percentiles** (p99 instead of p50) for more forgiving thresholds
3. **Add more concurrency** to smooth out variance
4. **Check network stability** in your CI environment

### Threshold Too Strict

If legitimate changes cause threshold failures:

1. **Re-baseline** your thresholds based on current performance
2. **Use ranges** with both upper and lower bounds
3. **Consider percentages** instead of absolute values for scaling scenarios

## Examples

### Basic API Test

```yaml
target: https://api.example.com/users
method: GET
concurrency: 20
duration: 30s
thresholds:
  - "http_req_duration:p95 < 200"
  - "http_req_failed:rate < 0.01"
```

### High-Throughput Test

```yaml
target: https://api.example.com/search
method: POST
body: '{"query":"test"}'
concurrency: 100
rate: 1000
duration: 1m
thresholds:
  - "http_req_duration:p99 < 1000"
  - "http_req_failed:rate < 0.05"
  - "http_requests:rate > 800"
```

### Strict SLA Test

```yaml
target: https://api.example.com/critical
method: GET
concurrency: 50
duration: 5m
thresholds:
  - "http_req_duration:p50 < 50"
  - "http_req_duration:p90 < 100"
  - "http_req_duration:p95 < 150"
  - "http_req_duration:p99 < 300"
  - "http_req_failed:rate < 0.001"
  - "http_requests:rate > 200"
```
