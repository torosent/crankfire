---
layout: default
title: Thresholds Quick Reference
---

# Thresholds Quick Reference

## Basic Syntax
```
metric:aggregate operator value
```

## Metrics

| Metric | Description | Example |
|--------|-------------|---------|
| `http_req_duration` | Request latency (ms) | `http_req_duration:p95 < 500` |
| `http_req_failed` | Request failures | `http_req_failed:rate < 0.01` |
| `http_requests` | Request throughput | `http_requests:rate > 100` |

## Aggregates

| Aggregate | Applies To | Description |
|-----------|------------|-------------|
| `p50` | latency | 50th percentile (median) |
| `p90` | latency | 90th percentile |
| `p95` | latency | 95th percentile |  
| `p99` | latency | 99th percentile |
| `avg` | latency | Average |
| `min` | latency | Minimum |
| `max` | latency | Maximum |
| `rate` | failures, requests | Rate (decimal or RPS) |
| `count` | failures, requests | Total count |

## Operators
`<` `<=` `>` `>=` `==`

## Common Patterns

### Latency SLA
```yaml
thresholds:
  - "http_req_duration:p50 < 100"   # Fast median
  - "http_req_duration:p95 < 500"   # Reasonable tail
  - "http_req_duration:p99 < 1000"  # Acceptable outliers
```

### Reliability
```yaml
thresholds:
  - "http_req_failed:rate < 0.01"   # < 1% errors
```

### Throughput
```yaml
thresholds:
  - "http_requests:rate > 100"      # At least 100 RPS
```

### Complete Example
```yaml
target: https://api.example.com
concurrency: 50
duration: 1m
thresholds:
  - "http_req_duration:p95 < 500"
  - "http_req_duration:p99 < 1000"
  - "http_req_failed:rate < 0.01"
  - "http_requests:rate > 100"
```

## CLI Usage
```bash
# Single threshold
crankfire --target https://api.example.com \
  --threshold "http_req_duration:p95 < 500"

# Multiple thresholds
crankfire --target https://api.example.com \
  --threshold "http_req_duration:p95 < 500" \
  --threshold "http_req_failed:rate < 0.01" \
  --threshold "http_requests:rate > 100"

# From config file
crankfire --config loadtest.yaml
```

## Exit Codes
- **0**: All thresholds passed
- **1**: One or more thresholds failed

## CI/CD Example
```bash
#!/bin/bash
set -e  # Exit on any error

crankfire --config perf-test.yaml

echo "✓ Performance test passed"
```
