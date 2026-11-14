---
layout: default
title: "Architecture: Project Overview"
---
{% raw %}

[Docs home](../index.md) · **Overview** · [Architecture](2-architecture-overview.md) · [Workflows](3-workflow-overview.md)

# Project Overview

## What is Crankfire?

Crankfire is a single-binary load-testing tool written in Go. It drives HTTP, WebSocket, Server-Sent Events (SSE) or gRPC traffic against a target, one protocol per run, and reports latency percentiles, throughput, failure buckets and protocol-specific counters.

It has four ways to run:

| Mode | Invocation | Persists results? |
|------|------------|-------------------|
| One-shot CLI run | `crankfire [flags]` / `crankfire --config file.yaml` | No (text or JSON on stdout, optional HTML file) |
| Terminal UI | `crankfire tui [--data-dir DIR]` | Yes, under the data dir |
| Set CLI | `crankfire set` (`list`, `show`, `run`, `new`, `diff`) | Yes (`set run`) |
| Scheduler daemon | `crankfire daemon [--data-dir DIR]` | Yes (scheduled set runs) |

`crankfire session list|edit` handles session metadata. It does not run tests.

## Core Purpose

- Model real traffic: weighted endpoints, ramp/step/spike/constant load patterns, and uniform or Poisson arrivals.
- Give accurate tail latency: per-endpoint HDR histograms at microsecond resolution.
- Fit into CI: JSON output, thresholds, non-zero exit codes and a standalone HTML report.
- Keep runs repeatable: saved sessions, multi-stage *sets* with thresholds, run-to-run diffs and cron scheduling.

## Technology Stack

| Concern | Library |
|---------|---------|
| Language | Go 1.26 (`go.mod`) |
| Flags / config | `spf13/cobra` (one command used as the flag container and help printer, no subcommand tree), `spf13/pflag`, `spf13/viper` for file reading, `gopkg.in/yaml.v3` for session/set files |
| Rate limiting | `golang.org/x/time/rate` |
| Latency histograms | `HdrHistogram/hdrhistogram-go` |
| WebSocket | `gorilla/websocket` |
| gRPC | `google.golang.org/grpc`, `jhump/protoreflect` (runtime `.proto` parsing, dynamic messages) |
| JSON extraction | `tidwall/gjson` |
| Terminal UI | `charmbracelet/bubbletea`, `bubbles`, `lipgloss` |
| Tracing | OpenTelemetry SDK + OTLP gRPC/HTTP exporters |
| Persistence | `gofrs/flock` (file locks), `oklog/ulid/v2` (IDs) |
| Scheduling | `robfig/cron/v3` |
| HTML charts | uPlot 1.6.24, loaded from the jsDelivr CDN by the generated report |

## Key Features

- Four protocols behind one `runner.Requester` interface.
- Weighted multi-endpoint HTTP tests, also generated from HAR recordings.
- CSV/JSON feeders and `{{placeholder}}` substitution.
- HTTP request chaining through JSONPath (gjson) or regex extractors into a per-worker variable store.
- OAuth2 client-credentials and resource-owner flows, plus static OIDC tokens.
- HTTP retries with exponential backoff and jitter.
- Thresholds (`http_req_duration:p95 < 500`) that fail the process.
- Live Bubble Tea dashboard (`--dashboard`), a 1-second progress line, text/JSON summaries and an HTML report.
- OpenTelemetry client spans with W3C trace-context propagation.
- Saved sessions, sets (sequential stages of parallel items), set thresholds, compare/diff, templates and a cron daemon.

## Entry Points

`cmd/crankfire/main.go` dispatches on `os.Args[1]` by hand:

```text
tui      -> tui.Run(tui.Options{DataDir})
daemon   -> store.NewFS(dir) -> cli.RunDaemon(...)
session  -> store.NewFS(dir) -> cli.RunSession(...)   (no --data-dir; uses CRANKFIRE_DATA_DIR or ~/.crankfire)
set      -> store.NewFS(dir) -> cli.RunSet(...)       (same)
*        -> cli.Run(args)                             (one-shot load test)
```

## Project Structure

```text
cmd/crankfire/          main.go dispatcher + integration tests (build tag "integration")
internal/
  cli/                  Orchestration: Run, BuildRunner, per-protocol requesters, endpoint
                        selection, retry policy, set/session/daemon subcommands
  cli/livedash/         Bubble Tea driver for --dashboard (hosts tui/runview)
  config/               Config struct, flag set, file loader, validation
  runner/               Scheduler + worker pool, arrival models, load patterns, retry/logging decorators
  metrics/              Sharded HDR-histogram collector, stats, time-series history, status buckets
  clientmetrics/        Mutex-guarded connection/message/byte counters for WS, SSE, gRPC clients
  pool/                 Keyed connection pool for WebSocket and SSE clients
  httpclient/           Tuned http.Client and RequestBuilder (feeder, placeholders, auth)
  websocket/            gorilla/websocket client wrapper
  sse/                  Hand-written SSE client on net/http
  grpcclient/           gRPC dial (TLS/insecure) and unary Invoke wrapper
  auth/                 OAuth2 (client credentials, resource owner) and static token providers
  feeder/               CSV and JSON record feeders (round robin, wrap-around)
  placeholders/         {{key}} / {{key|default}} substitution
  variables/            Per-worker variable store carried in context
  extractor/            JSONPath (gjson) and regex extractors
  har/                  HAR 1.2 parser and converter to endpoints
  threshold/            Threshold parser and evaluator
  tracing/              OTel provider init, request spans, header/metadata propagation
  output/               Text report, JSON report, progress ticker, HTML report
  output/setreport/     Offline HTML report for a set run
  store/                File-system store for sessions, runs, sets, set runs, templates
  setrunner/            Set execution, overrides, set thresholds, compare, diff
  scheduler/            Cron manager for scheduled sets
  tagfilter/            Tag filter expressions (space = AND, comma = OR)
  template/             text/template rendering for set templates
  tui/                  Bubble Tea root model (screen stack) and entry point
  tui/screens/          Session/set screens: list, edit, import, run, history, sets, compare, diff
  tui/runview/          Shared live run dashboard view
  tui/widgets/          Sparklines, trend chart, tables, progress bars
scripts/                Sample configs, test servers, doc-sample verification, mermaid validator
docs/                   This site (Jekyll, GitHub Pages)
```

Roughly 17k lines of non-test Go and 25k lines of tests.

## Getting Started (developers)

```bash
go build -o build/crankfire ./cmd/crankfire
go test -race ./...                                       # unit tests
go test -v -tags=integration -race ./cmd/crankfire/       # integration tests (as in CI)
scripts/verify-doc-samples.sh                             # parse-check sample configs
```

User-facing setup is covered in [Getting Started](../getting-started.md). Contribution workflow is in the [Developer Guide](../developer-guide.md).

## Architecture Summary

Configuration (flags plus an optional JSON/YAML file) becomes a `config.Config`. `cli.BuildRunner` wires that config into three things: a protocol-specific `runner.Requester` (decorated with endpoint selection, retry and logging for HTTP), a `metrics.Collector`, and a `runner.Runner`. The runner paces permits through an arrival controller and hands them to a fixed pool of workers. Each worker calls `Requester.Do`, which records latency and outcome in the collector. After the run, `internal/output` renders the collector's `Stats` and history, and `internal/threshold` decides pass or fail.

The TUI, the set runner and the daemon reuse `cli.BuildRunner` in-process. They add persistence (`internal/store`) and orchestration (`internal/setrunner`, `internal/scheduler`) on top.

Next: [Architecture Overview](2-architecture-overview.md).
{% endraw %}
