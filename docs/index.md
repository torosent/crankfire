---
layout: default
title: Crankfire
---
{% raw %}

# Crankfire

Crankfire is a single-binary load-testing CLI for **HTTP, WebSocket, SSE and gRPC**. You describe a workload with flags or a YAML/JSON file and watch it live in the terminal. The results come back as a text, JSON or HTML report, and you can gate CI on thresholds. Saved tests can be grouped into suites, compared and diffed, and run on a schedule.

```bash
crankfire --target https://api.example.com --concurrency 20 --rate 100 --duration 30s --dashboard
```

> Only load-test systems you own or have explicit permission to test.

## What you can do

| Capability | Where to read |
|---|---|
| Run a test from flags or a config file | [Getting Started](getting-started.md), [Configuration](configuration.md) |
| Shape traffic: rate limits, Poisson arrivals, ramp/step/spike patterns, weighted endpoints, retries | [Configuration → Workload modeling](configuration.md) |
| Test WebSocket, SSE and gRPC services | [Protocols](protocols.md) |
| Authenticate with OAuth2 client credentials, password grant or static tokens | [Authentication](authentication.md) |
| Inject per-request data from CSV/JSON files | [Data Feeders](feeders.md) |
| Extract values from responses and reuse them | [Request Chaining](request-chaining.md) |
| Replay a browser session from a HAR file | [HAR Import](har-import.md) |
| Fail a run on latency, error-rate or throughput limits | [Thresholds](thresholds.md) |
| Watch a live dashboard; export JSON and HTML reports | [Dashboard & Reporting](dashboard-reporting.md) |
| Export OpenTelemetry spans and propagate trace context | [Tracing](tracing.md) |
| Keep a library of tests in a terminal UI | [Terminal UI](tui.md) |
| Run suites of tests with stages, overrides, thresholds, diffs and cron schedules | [Sets, Scheduling & Session CLI](sets.md) |

## Commands at a glance

| Command | Purpose |
|---|---|
| `crankfire [flags]` | Run one load test (see [Configuration](configuration.md)) |
| `crankfire tui [--data-dir DIR]` | Open the [terminal UI](tui.md) |
| `crankfire session list`, `crankfire session edit` | Manage saved sessions and their tags ([Session CLI](sets.md#session-cli)) |
| `crankfire set list`, `show`, `run`, `new`, `diff` | Run and compare [sets](sets.md) |
| `crankfire daemon [--data-dir DIR]` | Run scheduled sets ([Scheduling](sets.md#scheduling-with-crankfire-daemon)) |

## Feature support by protocol

| Feature | HTTP | WebSocket | SSE | gRPC |
|---|:-:|:-:|:-:|:-:|
| Rate limiting, arrival models, load patterns | ✅ | ✅ | ✅ | ✅ |
| Authentication | ✅ | ✅ | ✅ | ✅ |
| Data feeders (`{{key}}` placeholders) | ✅ | ✅ | ✅ | ✅ |
| Placeholder default values | ✅ | — | — | — |
| Request chaining (extractors) | ✅ | — | — | — |
| Weighted multi-endpoint tests, HAR import | ✅ | — | — | — |
| Retries, `--log-errors` | ✅ | — | — | — |
| Thresholds, dashboard, JSON/HTML reports | ✅ | ✅ | ✅ | ✅ |
| Tracing and trace-context propagation | ✅ | ✅ | ✅ | ✅ |

## For contributors

- [Developer Guide](developer-guide.md): building, testing, repository layout, releasing.
- Architecture reference:
  - [Project Overview](architecture/1-project-overview.md)
  - [Architecture Overview](architecture/2-architecture-overview.md) (C4 context, containers, components)
  - [Workflow Overview](architecture/3-workflow-overview.md) (run, request, set, daemon and TUI sequences)
  - Deep dives: [Runner Engine](architecture/deep-dive/runner-engine.md) · [Metrics & Reporting](architecture/deep-dive/metrics-and-reporting.md) · [Configuration & Request Building](architecture/deep-dive/configuration-and-request-building.md) · [Protocol Clients](architecture/deep-dive/protocol-clients.md) · [Persistence & Sets](architecture/deep-dive/persistence-and-sets.md) · [Terminal UI](architecture/deep-dive/terminal-ui.md)
{% endraw %}
