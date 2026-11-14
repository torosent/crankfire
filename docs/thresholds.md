---
layout: default
title: Thresholds
---
{% raw %}

# Thresholds

Thresholds turn a load test into a pass/fail check. Crankfire evaluates them against the final run statistics and fails the run if any of them is not met, which makes it easy to gate a CI pipeline on latency, error rate or throughput.

This page covers thresholds for a **single run** (`crankfire --threshold ...`). Sets use a different, structured threshold format; see [Sets → Thresholds](sets.md#set-thresholds).

## Syntax

```
<metric>:<aggregate> <operator> <value>
```

- Lowercase only. Spaces around the operator are optional.
- `value` is a non-negative decimal number (`500`, `0.01`, `.5`). Negative numbers and exponents are not accepted.
- Latency values are **milliseconds**.

Pass them on the command line (repeatable) or as a list in the config file:

```bash
crankfire --target https://api.example.com --concurrency 20 --duration 1m \
  --threshold "http_req_duration:p95 < 500" \
  --threshold "http_req_failed:rate < 0.01"
```

```yaml
thresholds:
  - "http_req_duration:p95 < 500"
  - "http_req_duration:p99 < 1000"
  - "http_req_failed:rate < 0.01"
  - "http_requests:rate > 100"
```

> **Note:** `--threshold` is a comma-separated list flag, so never put a comma inside one value. Use one `--threshold` per rule.

A single string under `thresholds:` is also accepted and treated as a one-item list. `--threshold` flags **replace** the config file's list rather than adding to it.

## Metrics and aggregates

| Metric | Aggregate | Meaning |
|---|---|---|
| `http_req_duration` | `p50`, `p90`, `p95`, `p99` | Latency percentile (ms) |
| `http_req_duration` | `avg`, `min`, `max` | Mean / minimum / maximum latency (ms) |
| `http_req_failed` | `rate` | Failed ÷ total requests (0–1). `0` when no requests ran |
| `http_req_failed` | `count` | Number of failed requests |
| `http_requests` | `rate` | Requests per second over the whole run (reported as `0` when the run took less than 100 ms) |
| `http_requests` | `count` | Total requests recorded |

Despite the `http_` prefix, these metrics are computed from the same collector for **every protocol**: WebSocket, SSE and gRPC runs can use them too.

> **Note:** `mean` is not accepted; use `avg`. A metric/aggregate pair that parses but is not in the table above (for example `http_requests:p95`) does not abort the run. It is reported as a failed threshold with an `unsupported aggregate` message.

## Operators

`<`, `<=`, `>`, `>=`, `==`. The `<=`, `>=` and `==` comparisons use a small epsilon (1e-9), so float rounding does not cause false failures.

## When thresholds are checked

1. **Before the run**, every threshold is parsed. A syntax error stops Crankfire before any traffic is sent.
2. **After the run**, each threshold is evaluated against the final statistics, the same numbers shown in the report.

Requests retried with `--retries` are recorded once per attempt, so retries count toward `http_requests:count` and `http_req_failed:*`.

## Output

The text report ends with a threshold section:

```
Thresholds:
  ✓ http_req_duration:p95 < 500: 212.40 < 500.00
  ✗ http_req_failed:rate < 0.01: 0.03 < 0.01

Threshold Summary: 1/2 passed
```

With `--json-output`, results appear under a `thresholds` key:

```json
"thresholds": {
  "total": 2,
  "passed": 1,
  "failed": 1,
  "results": [
    {
      "threshold": "http_req_duration:p95 < 500",
      "metric": "http_req_duration",
      "aggregate": "p95",
      "operator": "<",
      "expected": 500,
      "actual": 212.4,
      "pass": true
    }
  ]
}
```

The JSON encoder escapes `<`, `>` and `&`, so the raw file contains `"threshold": "http_req_duration:p95 \u003c 500"`. `jq` decodes this for you, but a plain `grep` for `<` will not match.

The [HTML report](dashboard-reporting.md#html-report) also includes a pass/fail table.

## Exit codes

| Outcome | Exit code |
|---|---|
| Every threshold passed **and** no request failed | `0` |
| Any threshold failed | `1` (`one or more thresholds failed`) |
| No threshold failed, but at least one request failed | `1` (`N requests failed`) |
| Invalid threshold syntax, or a config or setup error | `1` |

> **Note:** A failed request fails the run even when a threshold like `http_req_failed:rate < 0.01` allows some errors. If you need to tolerate a small error rate in CI, check the JSON output instead of relying on the exit code alone (see below).

## CI examples

### GitHub Actions

```yaml
name: Performance gate
on: [pull_request]

jobs:
  load-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-go@v6
        with:
          go-version: stable
      - run: go install github.com/torosent/crankfire/cmd/crankfire@latest
      - name: Run load test
        run: |
          crankfire --config perf/loadtest.yaml --json-output > results.json
      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: load-test-results
          path: results.json
```

### Tolerating some errors

Let the run finish, then gate on the threshold verdict only:

```bash
crankfire --config loadtest.yaml --json-output > results.json || true
jq -e '.thresholds.failed == 0' results.json
```

### Downloading a release binary

Release assets are archives named `crankfire_<os>_<arch>.tar.gz` (`.zip` on Windows):

```bash
curl -fsSL -o crankfire.tar.gz \
  https://github.com/torosent/crankfire/releases/latest/download/crankfire_linux_amd64.tar.gz
tar -xzf crankfire.tar.gz
```

## Recipes

```yaml
# API latency SLO
thresholds:
  - "http_req_duration:p95 < 300"
  - "http_req_duration:p99 < 800"
  - "http_req_failed:rate < 0.001"

# Throughput floor
thresholds:
  - "http_requests:rate >= 500"

# Hard ceiling on any single request
thresholds:
  - "http_req_duration:max < 5000"
```

## See also

- [Dashboard & Reporting](dashboard-reporting.md): where threshold results show up.
- [Sets](sets.md): structured thresholds across many sessions (`p95`, `error_rate`, `rps`, scopes).
- [Configuration](configuration.md): the full flag and key reference.
{% endraw %}
