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

- `--target` is required.
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

## Next Steps

- Learn how to describe realistic workloads: [Configuration & CLI Reference](configuration.md).
- Explore authentication helpers: [Authentication](authentication.md).
- Introduce dynamic test data: [Data Feeders](feeders.md).
- Try out WebSocket, SSE, and gRPC: [Protocols](protocols.md).
