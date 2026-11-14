---
layout: default
title: Getting Started
---
{% raw %}

# Getting Started

This guide covers installing Crankfire and running your first tests.

> Only load-test systems you own or have explicit permission to test.

## Install

**Homebrew** (macOS and Linux):

```bash
brew tap torosent/crankfire
brew install crankfire
```

**Docker**:

```bash
docker run --rm ghcr.io/torosent/crankfire --target https://example.com --total 100
```

**Go**:

```bash
go install github.com/torosent/crankfire/cmd/crankfire@latest
```

**Prebuilt binaries**: download `crankfire_<os>_<arch>.tar.gz` (Linux, macOS) or the `.zip` (Windows) from the [releases page](https://github.com/torosent/crankfire/releases).

**From source**:

```bash
git clone https://github.com/torosent/crankfire.git
cd crankfire
go build -o build/crankfire ./cmd/crankfire
```

Check that it works:

```bash
crankfire --help
```

## Your first test

Send 100 requests using 10 workers:

```bash
crankfire --target https://httpbin.org/get --concurrency 10 --total 100
```

While the test runs, a progress line updates in place on stdout. When it finishes, Crankfire prints a report:

```
--- Load Test Results ---
Total Requests:    100
Successful:        98
Failed:            2
Duration:          2.41s
Requests/sec:      41.49

Latency:
  Min:             38.2ms
  Max:             412.7ms
  Mean:            92.5ms
  P50:             71.3ms
  P90:             160.1ms
  P95:             221.8ms
  P99:             398.0ms

Status Buckets:
  HTTP 503: 2
```

Reading the report:

- **Latency** covers the whole request, from building it until the full response body has been read. Percentiles come from an HDR histogram.
- **Status Buckets** list only **failures**, grouped by protocol and code. For HTTP, a failure is a status of `400` or higher, or a transport error. See [Protocols](protocols.md#status-buckets).
- The report can also include sections for thresholds, per-endpoint results and protocol metrics. Each one appears only when it applies.

## Controlling load

```bash
# 20 workers, at most 100 req/s, for 30 seconds
crankfire --target https://httpbin.org/get -c 20 -r 100 -d 30s
```

- `--concurrency` / `-c`: the maximum number of requests in flight.
- `--rate` / `-r`: requests per second. `0` means unlimited.
- `--duration` / `-d` or `--total` / `-t`: when to stop. The test ends at whichever limit comes first. With neither set, it runs until you press Ctrl+C.

For ramps, steps, spikes and Poisson arrivals, see [Modeling the workload](configuration.md#modeling-the-workload).

## POST with headers

```bash
crankfire --target https://httpbin.org/post \
  --method POST \
  --header "Content-Type=application/json" \
  --body '{"name":"crankfire"}' \
  --total 50
```

Headers are written as `Key=Value`, and `--header` can be repeated.

## Using a config file

`loadtest.yaml`:

```yaml
target: https://api.example.com/users
concurrency: 25
rate: 50
duration: 2m
timeout: 10s
retries: 2
headers:
  Accept: application/json
thresholds:
  - "http_req_duration:p95 < 500"
  - "http_req_failed:rate < 0.01"
```

```bash
crankfire --config loadtest.yaml
crankfire --config loadtest.yaml --concurrency 50   # flags you pass override the file
```

The [Configuration reference](configuration.md) lists every key.

## Live dashboard

```bash
crankfire --target https://httpbin.org/get -c 20 -r 100 -d 1m --dashboard
```

The dashboard is a full-screen terminal view. It shows throughput, latency trends, percentiles, endpoints, status buckets and load-pattern progress. Press `q`, `Esc` or `Ctrl+C` to **stop the test**; the final report then prints. `--dashboard` can't be combined with `--json-output`. See [Dashboard & Reporting](dashboard-reporting.md).

## JSON and HTML reports

```bash
crankfire --target https://httpbin.org/get --total 500 --json-output > results.json
crankfire --target https://httpbin.org/get --total 500 --html-output report.html
```

`report.html` is a single file with charts of throughput and latency over time. The charts load from a CDN, so viewing it needs a network connection.

## Replaying a browser session

Save a HAR from your browser's DevTools Network tab, then run:

```bash
crankfire --har recording.har --har-filter "host:api.example.com" --total 100
```

See [HAR Import](har-import.md).

## Exit codes

The command exits with `0` only if **every threshold passed and no request failed**. Anything else exits with `1`, including a single failed request when no thresholds are set. Stopping a test early with Ctrl+C or `q` cancels in-flight requests, which count as failures, so an interrupted run usually exits with `1`. This makes it usable as a CI gate without extra scripting. See [Exit codes](configuration.md#exit-codes) and [Thresholds](thresholds.md).

## Next steps

- [Configuration & CLI Reference](configuration.md): every flag and key, plus workload modeling
- [Protocols](protocols.md): WebSocket, SSE and gRPC
- [Authentication](authentication.md), [Data Feeders](feeders.md), [Request Chaining](request-chaining.md)
- [Terminal UI](tui.md) and [Sets, Scheduling & Session CLI](sets.md): saved tests, suites, and runs on a schedule
{% endraw %}
