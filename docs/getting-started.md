---
layout: default
title: Getting Started
---

# Getting Started

This guide walks you from installation to your first meaningful load test.

## Install Crankfire

### Homebrew (macOS & Linux)

```bash
brew tap torosent/crankfire
brew install crankfire
```

### Go Install

```bash
go install github.com/torosent/crankfire/cmd/crankfire@latest
```

### Build From Source

```bash
git clone https://github.com/torosent/crankfire.git
cd crankfire
go build -o build/crankfire ./cmd/crankfire
```

### Docker

You can run Crankfire directly from the GitHub Container Registry without installing anything:

```bash
docker run ghcr.io/torosent/crankfire --target https://example.com --total 100
```

## Verify the Installation

```bash
crankfire --help
```

You should see the list of supported flags and a short description.

## Your First Load Test

### Minimal Example

Hit a public echo service with 100 requests:

```bash
crankfire --target https://httpbin.org/get --total 100
```

Key points:

- `--target` is required unless using `--har` or endpoints with full URLs.
- `--total` controls the total number of requests (instead of duration).

### Concurrency and Duration

Run a 30‑second test with 20 workers:

```bash
crankfire \
  --target https://httpbin.org/get \
  --concurrency 20 \
  --duration 30s
```

### Add the Dashboard

For a richer view, enable the terminal dashboard:

```bash
crankfire \
  --target https://httpbin.org/get \
  --concurrency 20 \
  --rate 100 \
  --duration 30s \
  --dashboard
```

Use `q` or `Ctrl+C` to exit the dashboard.

## Using a Config File

Instead of long command lines, define a test in JSON or YAML.

`loadtest.yaml`:

```yaml
target: https://api.example.com/users
method: GET
concurrency: 25
rate: 50
duration: 2m
timeout: 10s
retries: 2
headers:
  Authorization: Bearer your-token
  Accept: application/json
```

Run it with:

```bash
crankfire --config loadtest.yaml
```

You can still override values with flags:

```bash
crankfire --config loadtest.yaml --concurrency 50 --duration 5m
```

## Generating Reports

Crankfire can generate standalone HTML reports with interactive charts.

```bash
crankfire --target https://httpbin.org/get \
  --concurrency 10 \
  --total 1000 \
  --html-output report.html
```

Open `report.html` in your browser to view the results.

## Import from HAR File

You can record a browser session and replay it as a load test:

1. Open browser DevTools (F12) → Network tab
2. Perform the actions you want to test
3. Right-click → "Save all as HAR"
4. Run the load test:

```bash
crankfire --har recording.har --har-filter "host:api.example.com" --total 100
```

See [HAR Import](har-import.md) for filtering options and best practices.

## Next Steps

- Learn how to describe realistic workloads: [Configuration & CLI Reference](configuration.md).
- Explore authentication helpers: [Authentication](authentication.md).
- Introduce dynamic test data: [Data Feeders](feeders.md).
- Try out WebSocket, SSE, and gRPC: [Protocols](protocols.md).
