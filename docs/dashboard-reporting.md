---
layout: default
title: Dashboard & Reporting
---

# Dashboard & Reporting

Crankfire is designed to be both human and machine friendly: you can watch tests live in your terminal and also capture detailed JSON reports for automation.

## Live Dashboard

Enable the dashboard with `--dashboard`:

```bash
crankfire --target https://api.example.com \
  --concurrency 20 \
  --rate 100 \
  --duration 60s \
  --dashboard
```

The dashboard shows:

- Overall summary (elapsed, total, success rate).
- Request rate gauge.
- Latency percentiles (min/mean/p50/p90/p99).
- Status buckets by protocol.
- Endpoint breakdown with RPS and tail latency.

Controls:

- `q` or `Ctrl+C` – exit.
- Terminal resizing is handled automatically.

## Progress Ticker

Even without the dashboard, Crankfire prints lightweight progress updates to stderr every second, including totals, success/failure counts, and RPS.

## JSON Output

Use `--json-output` to emit a structured report instead of human‑readable text:

```bash
crankfire --target https://api.example.com \
  --total 1000 \
  --json-output > results.json
```

The JSON includes:

- Global totals and success/failure counts.
- Latency statistics (min/max/mean/p50/p90/p99).
- Duration and achieved RPS.
- Status buckets (HTTP, gRPC, protocol‑specific codes).
- Per‑endpoint metrics.
- Protocol‑specific metrics for WebSocket, SSE, and gRPC.

## HTML Report

Generate a standalone HTML report with interactive charts and detailed statistics using `--html-output`:

```bash
crankfire --target https://api.example.com \
  --total 1000 \
  --html-output report.html
```

The report includes:

- **Summary Cards**: Key metrics at a glance.
- **Interactive Charts**: RPS and latency percentiles over time.
- **Latency Statistics**: Detailed percentile breakdown.
- **Threshold Results**: Pass/fail status for configured thresholds.
- **Endpoint Breakdown**: Per-endpoint performance metrics.

## CI/CD Integration

Combine JSON output with tools like `jq` to enforce performance budgets:

```bash
crankfire --target https://api.example.com \
  --total 1000 \
  --json-output > results.json

p99=$(jq -r '.p99_latency_ms' results.json)
if (( $(echo "$p99 > 200" | bc -l) )); then
  echo "P99 latency too high: ${p99}ms"
  exit 1
fi
```

For a complete GitHub Actions example, see [Usage Examples](usage.md).
