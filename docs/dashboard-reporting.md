---
layout: default
title: Dashboard & Reporting
---
{% raw %}

# Dashboard & Reporting

Every run produces a final report. While the test runs, you choose how to watch it:

| Mode | Flag | While running | At the end (stdout) |
|---|---|---|---|
| Progress line (default) | none | One-line ticker on stdout, updated every second | Text report |
| Live dashboard | `--dashboard` | Full-screen terminal UI | Text report |
| JSON | `--json-output` | Nothing | JSON document |

`--html-output <file>` can be added to any of these modes. `--dashboard` and `--json-output` cannot be combined.

## Progress line

Without `--dashboard` or `--json-output`, Crankfire rewrites a single line on **stdout** once per second:

```
Requests: 1520 | Successes: 1518 | Failures: 2 | RPS: 101.3 | Top Endpoint: list-products (80%, P99 41.2ms)
```

The `Top Endpoint` part appears only when the run uses endpoints. WebSocket, SSE and gRPC runs also add the protocol name at the end (for example `| websocket`).

## Live dashboard

```bash
crankfire --target https://api.example.com \
  --concurrency 20 --rate 100 --duration 60s \
  --dashboard
```

The dashboard needs an interactive terminal (TTY). It runs in the terminal's alternate screen, refreshes every 500 ms, and shows these panels:

| Panel | Contents |
|---|---|
| Run Summary | Elapsed time, total requests, success rate, progress toward `--total`, load-pattern progress |
| RPS strip | Current requests/second, with a target bar when `--rate` is set |
| Request Context | Target URL, method/protocol, workers, rate, duration, timeout, retries |
| Latency Trend | Chart of the run's p95 (cumulative since the start), sampled each refresh, with P50/P95 marker lines and an SLO line taken from your first `http_req_duration:p95 <` (or `<=`) threshold |
| Latency Stats / Health | Min, mean, P50, P90, P95, P99 |
| Protocol Metrics / Load Pattern | Protocol counters, or the current step and progress of your `load_patterns` |
| Endpoints | Per-endpoint share, RPS, p95, p99, error % |
| Status Buckets | Failure counts grouped by protocol and status code |

On wide terminals (≥ 80 columns) panels are laid out side by side; on narrow ones they stack.

**Controls:** `q`, `Esc` or `Ctrl+C` stops the test early. The dashboard closes and the final report prints as usual. Requests that are still in flight are cancelled at once (the graceful-shutdown window applies only when a run ends on its own) and are counted as failures, so a stopped run usually exits with code `1`.

The [TUI](tui.md) shows a reduced version of this view while it runs a session.

## Text report

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
  HTTP 503: 2

Endpoint Breakdown:
  - list-products: total=800 (80.0%), successes=800, failures=0, rps=78.43, p99=95ms
  - create-order: total=200 (20.0%), successes=198, failures=2, rps=19.61, p99=180ms
    Status Buckets:
      HTTP 503: 2
```

The report contains these sections:

- **Status Buckets** lists **failures only**, grouped by protocol and code: HTTP status, WebSocket close code, gRPC status name, or an error type such as `ERROR`. See [Protocols → Status buckets](protocols.md#status-buckets).
- **Thresholds** appears when you configured [thresholds](thresholds.md).
- **Endpoint Breakdown** appears when the run uses `endpoints` (or a HAR import), sorted by request count.
- **Protocol Metrics** lists counters for WebSocket, SSE and gRPC runs, such as `messages_sent` and `bytes_received`.

## JSON output

```bash
crankfire --config loadtest.yaml --json-output > results.json
```

The document is a single object with every field at the top level:

| Key | Type | Notes |
|---|---|---|
| `total`, `successes`, `failures` | int | Counts of recorded requests (retries count once per attempt) |
| `requests_per_sec` | float | |
| `min_latency_ms`, `max_latency_ms`, `mean_latency_ms` | float | |
| `p50_latency_ms`, `p90_latency_ms`, `p95_latency_ms`, `p99_latency_ms` | float | |
| `duration_ms` | float | Wall-clock duration of the run |
| `status_buckets` | object | `{ "<protocol>": { "<code>": count } }`, failures only; omitted when empty |
| `endpoints` | object | `{ "<name>": { ...same latency/count fields... } }`; omitted when empty |
| `protocol_metrics` | object | `{ "<protocol>": { "<metric>": value } }`; omitted when empty |
| `thresholds` | object | `{ total, passed, failed, results[] }`; omitted when there are no thresholds |

Example:

```json
{
  "total": 1000,
  "successes": 998,
  "failures": 2,
  "requests_per_sec": 98.04,
  "min_latency_ms": 12.4,
  "max_latency_ms": 156.8,
  "mean_latency_ms": 45.2,
  "p50_latency_ms": 42.1,
  "p90_latency_ms": 68.3,
  "p95_latency_ms": 81.0,
  "p99_latency_ms": 112.7,
  "status_buckets": { "http": { "503": 2 } },
  "duration_ms": 10245.3,
  "endpoints": {
    "create-order": {
      "total": 200,
      "successes": 198,
      "failures": 2,
      "requests_per_sec": 19.61,
      "p99_latency_ms": 180.4,
      "status_buckets": { "http": { "503": 2 } }
    }
  }
}
```

(The endpoint entry above is trimmed. Every endpoint carries the full set of latency fields.)

Handy `jq` queries:

```bash
jq '.p95_latency_ms' results.json
jq '.failures / .total' results.json
jq '.endpoints | to_entries[] | {name: .key, p99: .value.p99_latency_ms}' results.json
jq '.protocol_metrics.websocket.messages_received' results.json
```

## HTML report

```bash
crankfire --target https://example.com --total 1000 --html-output report.html
```

This writes a single HTML file. Its path is printed to stderr when the run ends. The file contains:

- Summary cards: total requests, success and failure percentages, RPS
- "Performance Over Time" charts: RPS, plus P50/P95/P99 latency, sampled about every second during the run. With `--dashboard`, the dashboard adds its own samples every 500 ms, so points are unevenly spaced and some RPS points can show false dips to 0
- A latency statistics grid (min through p99)
- Threshold results
- Per-endpoint breakdown
- The list of tested endpoints (name, method, URL)

> **Note:** The charts load the [uPlot](https://github.com/leeoniya/uPlot) library from the jsDelivr CDN, so opening the report offline shows everything except the charts. The HTML report does not include status buckets or protocol metrics; use the text or JSON output for those.

The latency percentiles in the time series are cumulative from the start of the run. They are not per-interval values.

## Logging failed requests

`--log-errors` prints each failed HTTP request to stderr as it happens, as `[crankfire] request failed: <error>`. It applies to HTTP only.

## Reports for TUI runs and sets

- Single-session runs started from the [TUI](tui.md) save `result.json` (the JSON report above) and `report.html` into the run's directory under the data dir. Press `o` in the History screen to open the report.
- Set runs produce `set-run.json`. `crankfire set run --html <file>` renders a self-contained set report (no CDN), and `crankfire set diff` compares two runs. See [Sets](sets.md).
{% endraw %}
