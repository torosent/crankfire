---
layout: default
title: Configuration & CLI Reference
---
{% raw %}

# Configuration & CLI Reference

This page lists every setting Crankfire reads, how settings are combined, and how the main load-test command exits. Protocol-specific behaviour is in [Protocols](protocols.md). Auth, feeders and chaining each have their own pages.

- [How configuration is resolved](#how-configuration-is-resolved)
- [CLI flags](#cli-flags)
- [Config file keys](#config-file-keys)
- [Modeling the workload](#modeling-the-workload)
- [Environment variables](#environment-variables)
- [Validation](#validation)
- [Exit codes](#exit-codes)
- [Full example](#full-example)

## How configuration is resolved

Each setting is applied in this order. Later sources win.

1. **Built-in defaults**: `method: GET`, `concurrency: 1`, `timeout: 30s`, `graceful_shutdown: 5s`, `arrival: uniform`. Everything else starts empty or zero.
2. **Config file**, passed with `--config path`.
3. **CLI flags you actually pass.** A flag you leave out never overrides the config file, even if `--help` shows a default for it. See [Effective defaults](#effective-defaults).

Details:

- **File format** comes from the file extension: `.yaml`/`.yml` or `.json`. Viper's other formats (`.toml` and so on) also load, but only YAML and JSON are tested.
- **Keys are case-insensitive.** Multi-word keys accept snake, kebab or concatenated spelling: `body_file`, `body-file` and `bodyfile` are the same key. This page uses snake_case.
- **Durations** take Go duration strings (`500ms`, `30s`, `1m30s`). A bare number counts as **seconds**, and fractional seconds are truncated (`1.5` becomes `1s`, and `timeout: 0.5` becomes `0`, which means no timeout). Write `500ms` instead of `0.5`.
- **Relative paths** (`body_file`, `feeder.path`, `grpc.proto_file`, `har_file`) resolve against the **current working directory**, not the directory of the config file.
- `--body` and `--body-file` cancel each other: passing one clears the other value from the config file.
- `--header` entries are **merged** into the config file's `headers`. `--threshold`, `--ws-messages` and `--grpc-metadata` **replace** the config value.
- Running `crankfire` with no arguments prints usage.

> **Note:** `--header`, `--threshold` and `--ws-messages` are string-slice flags. Each value is parsed as CSV: it is **split on commas**, and a value that contains a double quote is rejected (`bare " in non-quoted-field`). `--header "Accept=a,b"` fails with `header must be in key=value format: b`, and a JSON message such as `--ws-messages '{"type":"ping"}'` fails to parse. Put JSON or values with commas in the config file instead.

## CLI flags

`crankfire [flags]` runs one load test. Subcommands (`tui`, `session`, `set`, `daemon`) are described in [Terminal UI](tui.md) and [Sets, Scheduling & Session CLI](sets.md).

### General and HTTP

| Flag | Config key | Default | Notes |
|------|-----------|---------|-------|
| `--config` | — | — | Path to a YAML or JSON config file. |
| `--target` | `target` | — | Target URL (`host:port` for gRPC). Required unless `har_file` is set or every endpoint has a `url`. |
| `--method` | `method` | `GET` | Uppercased. HTTP only. |
| `--header` | `headers` | — | `Key=Value`, repeatable. Keys are canonicalised (`content-type` becomes `Content-Type`). |
| `--body` | `body` | — | Inline request body. |
| `--body-file` | `body_file` | — | Body read from a file. Streamed unless it contains `{{placeholders}}`. |
| `-c`, `--concurrency` | `concurrency` | `1` | Number of worker goroutines. |
| `-r`, `--rate` | `rate` | `0` | Requests per second. `0` means no limit. |
| `-d`, `--duration` | `duration` | `0` | Stop scheduling new requests after this long. `0` means no time limit. |
| `-t`, `--total` | `total` | `0` | Stop after this many requests. `0` means no count limit. |
| `--timeout` | `timeout` | `30s` | HTTP client timeout per request (connect, headers and body). `0` means no timeout. Not used by WebSocket, SSE or gRPC. |
| `--graceful-shutdown` | `graceful_shutdown` | `5s` | Time in-flight requests get to finish when the run ends on its own (total, duration or pattern end). `0` means 5s. A negative value cancels them immediately. Not used on Ctrl+C. |
| `--retries` | `retries` | `0` | Retries per request. HTTP only. See [Retries](#retries). |
| `--arrival-model` | `arrival` / `arrival_model` | `uniform` | `uniform` or `poisson`. |
| `--log-errors` | `log_errors` | `false` | Print each failed request to stderr. HTTP only. With `--retries`, every failed attempt is printed. |
| `--protocol` | `protocol` | `http` | `http`, `websocket`, `sse` or `grpc`. |

### Output

| Flag | Config key | Default | Notes |
|------|-----------|---------|-------|
| `--dashboard` | `dashboard` | `false` | Live terminal dashboard. Cannot be combined with `--json-output`. |
| `--json-output` | `json_output` | `false` | Print the final report as JSON on stdout. |
| `--html-output` | `html_output` | — | Also write an HTML report to this path. |
| `--threshold` | `thresholds` | — | Pass/fail rule, repeatable. See [Thresholds](thresholds.md). |

### Data and HAR

| Flag | Config key | Default | Notes |
|------|-----------|---------|-------|
| `--feeder-path` | `feeder.path` | — | CSV or JSON file. See [Data Feeders](feeders.md). |
| `--feeder-type` | `feeder.type` | — | `csv` or `json`. Required when a path is set. |
| `--har` | `har_file` | — | HAR file to import as endpoints. See [HAR Import](har-import.md). |
| `--har-filter` | `har_filter` | — | For example `host:api.example.com;method:GET,POST`. |

### WebSocket, SSE, gRPC

| Flag | Config key | Effective default | Notes |
|------|-----------|-------------------|-------|
| `--ws-messages` | `websocket.messages` | — | Text frames sent on each request. |
| `--ws-message-interval` | `websocket.message_interval` | `0` | Pause after each message. |
| `--ws-receive-timeout` | `websocket.receive_timeout` | **`0`** (no receive phase) | `--help` shows `10s`, but that value applies only when you pass the flag. |
| `--ws-handshake-timeout` | `websocket.handshake_timeout` | `30s` | `0` falls back to the client's 30s default. |
| `--sse-read-timeout` | `sse.read_timeout` | `30s` | `0` falls back to the SSE client's 30s default. |
| `--sse-max-events` | `sse.max_events` | **`100`** | Help says `0=unlimited`, but `0` is treated as 100. |
| `--grpc-proto-file` | `grpc.proto_file` | — | Required at run time for gRPC. |
| `--grpc-service` | `grpc.service` | — | Fully qualified, for example `helloworld.Greeter`. |
| `--grpc-method` | `grpc.method` | — | For example `SayHello`. |
| `--grpc-message` | `grpc.message` | `{}` | JSON request body. Supports placeholders. |
| `--grpc-metadata` | `grpc.metadata` | — | `k=v,k2=v2`. Keys are lowercased. |
| `--grpc-timeout` | `grpc.timeout` | **`0`** (no deadline) | `--help` shows `30s`, but that value applies only when you pass the flag. |
| `--grpc-tls` | `grpc.tls` | `false` | Use TLS with system roots. |
| `--grpc-insecure` | `grpc.insecure` | `false` | Together with `--grpc-tls`, skips certificate verification. |

### Tracing

| Flag | Config key | Effective default | Notes |
|------|-----------|-------------------|-------|
| `--tracing-endpoint` | `tracing.endpoint` | — | OTLP `host:port`. Tracing is off unless this is set. |
| `--tracing-protocol` | `tracing.protocol` | `grpc` | `grpc` or `http`. |
| `--tracing-service-name` | `tracing.service_name` | `crankfire` | Falls back to `OTEL_SERVICE_NAME`, then `crankfire`. |
| `--tracing-sample-rate` | `tracing.sample_rate` | **`0`** (nothing sampled) | `--help` shows `1`, but that value applies only when you pass the flag. **Always set this explicitly.** |
| `--tracing-insecure` | `tracing.insecure` | `false` | Plaintext connection to the collector. |
| — | `tracing.propagate` | `true` when an endpoint is set | Config file only. Injects W3C `traceparent` headers. |

See [Tracing](tracing.md).

### Effective defaults

`--help` prints defaults for every flag, but a flag only takes effect when you pass it. When a setting is absent from both the config file and the command line, Crankfire uses its internal value, which can differ from the help text. The bold entries in the tables above show where the two disagree: `ws-receive-timeout`, `sse-max-events`, `grpc-timeout` and `tracing-sample-rate`. If you rely on one of these, set it explicitly.

## Config file keys

### Top level

| Key | Type | Description |
|-----|------|-------------|
| `target` | string | Base URL, or gRPC `host:port`. |
| `method` | string | HTTP method. Default `GET`. |
| `headers` | map | Request headers. Values support placeholders. |
| `body` / `body_file` | string | Request body (mutually exclusive). |
| `concurrency`, `rate`, `total` | int | See [CLI flags](#cli-flags). |
| `duration`, `timeout`, `graceful_shutdown` | duration | See [CLI flags](#cli-flags). |
| `retries` | int | HTTP retries per request. |
| `arrival` | string or `{model: …}` | `uniform` or `poisson`. `arrival_model: poisson` is also accepted. If both keys are present, `arrival` wins. |
| `load_patterns` | list | See [Load patterns](#load-patterns). Config file only. |
| `endpoints` | list | See [Weighted endpoints](#weighted-endpoints). Config file only. |
| `protocol` | string | `http` (default), `websocket`, `sse`, `grpc`. |
| `websocket`, `sse`, `grpc` | object | Protocol blocks; see the tables above and [Protocols](protocols.md). |
| `auth` | object | Config file only. See [Authentication](authentication.md). |
| `feeder` | `{path, type}` | See [Data Feeders](feeders.md). |
| `thresholds` | list of strings | A single string is accepted as a one-item list. |
| `har_file`, `har_filter` | string | See [HAR Import](har-import.md). |
| `tracing` | object | `endpoint`, `protocol`, `service_name`, `sample_rate`, `insecure`, `propagate`. |
| `dashboard`, `json_output`, `log_errors` | bool | Output switches. |
| `html_output` | string | HTML report path. |

### Endpoint

| Key | Description |
|-----|-------------|
| `name` | Label used in reports and metrics. Must be unique (case-insensitive). Defaults to the resolved URL. |
| `weight` | Relative selection weight, `>= 1`. Default `1`. |
| `method` | Overrides the global method. |
| `url` (alias `target`) | Absolute URL. Takes precedence over `path`. |
| `path` | Resolved against the global `target`. |
| `headers` | Merged over the global headers. Endpoint values win. |
| `body` / `body_file` | Replaces the global body. |
| `extractors` | A list of extractors. Each has `jsonpath` or `regex`, a `var`, and optionally `on_error`. See [Request Chaining](request-chaining.md). |

### Load pattern

| Key | Used by | Description |
|-----|---------|-------------|
| `name` | all | Optional label. It is currently not displayed; the dashboard shows the pattern type and rates. |
| `type` | all | `constant`, `ramp`, `step` or `spike`. |
| `rps`, `duration` | constant, spike | Target rate and how long to hold it. |
| `from_rps`, `to_rps`, `duration` | ramp | Linear ramp between the two rates. |
| `steps` | step | List of `{rps, duration}`. |

## Modeling the workload

### Stop conditions

A single scheduler hands out request permits to `concurrency` workers. Scheduling stops at the **first** of these:

- `total` requests have been scheduled,
- `duration` has elapsed,
- the last load pattern has finished,
- you press Ctrl+C (or `q` in the dashboard).

If you set none of `total`, `duration` or `load_patterns`, the test runs until you interrupt it. When the run ends on its own, in-flight requests have `graceful_shutdown` (default 5s) to finish before they are cancelled.

> **Note:** On Ctrl+C or `q` in the dashboard, in-flight requests are cancelled at once and counted as failures. An interrupted run therefore normally exits with `1`.

### Rate and arrival models

`rate` caps how often permits are issued. `rate: 0` means as fast as the workers can go. `concurrency` caps how many requests are in flight at once.

- `uniform` (default) uses a token-bucket limiter with a burst of about 10 ms of traffic (`max(1, rps/100)`), which gives evenly spaced requests.
- `poisson` uses exponentially distributed gaps with mean `1/rps`, which models independent users arriving at random.

### Load patterns

When `load_patterns` is set, it controls the rate for the whole run, and the run ends when the last pattern ends. The rate is updated every 100 ms.

```yaml
load_patterns:
  - name: warmup
    type: ramp
    from_rps: 10
    to_rps: 200
    duration: 1m
  - name: plateau
    type: constant
    rps: 200
    duration: 5m
  - name: stairs
    type: step
    steps:
      - { rps: 250, duration: 1m }
      - { rps: 300, duration: 1m }
  - name: burst
    type: spike
    rps: 800
    duration: 10s
```

> **Note:** A pattern rate never drops below **1 rps**, so `from_rps: 0` effectively starts at 1. Make sure `concurrency` is high enough to sustain your peak rate.

### Weighted endpoints

With `endpoints` set, each HTTP request picks one endpoint at random, in proportion to `weight`. Each endpoint inherits the global `method`, `headers` and `body` and can override them:

```yaml
target: https://api.example.com
headers:
  Accept: application/json
endpoints:
  - name: list-products
    weight: 8
    path: /products
  - name: create-order
    weight: 2
    method: POST
    path: /orders
    headers:
      Content-Type: application/json
    body: '{"product_id":"{{product_id}}","quantity":1}'
  - name: partner-api
    url: https://partner.example.com/v1/ping
```

- `path` is resolved against `target` using standard URL rules. A leading `/` replaces the target's path.
- Placeholders such as `/users/{{user_id}}` work in paths.
- Endpoint `name` is the key for per-endpoint metrics. Two endpoints with the same name would merge into one bucket, so validation rejects duplicates. HAR-imported endpoints are not validated: entries with the same path share one row.
- Endpoints are HTTP only.

### Retries

`retries: N` makes up to `N+1` attempts per HTTP request. It retries status `429`, status `5xx`, and network or build errors. Other `4xx` responses, per-request timeouts and cancellation are not retried. Backoff is `100ms × 2^(attempt-1)`, capped at 5s, plus up to 50% random jitter. Every retry goes to the same endpoint.

> **Note:** Each attempt is recorded in the metrics, so with retries enabled the report's request count can be higher than the number of scheduled requests.

## Environment variables

Crankfire itself reads these variables. There is no general `CRANKFIRE_<KEY>` mapping. HTTP and WebSocket also honour the standard proxy variables (`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`), and the tracing exporter reads the standard `OTEL_EXPORTER_OTLP_*` variables (see [Tracing](tracing.md)).

| Variable | Effect |
|----------|--------|
| `CRANKFIRE_AUTH_CLIENT_SECRET` | Fills `auth.client_secret` when an `auth` block exists and the field is empty. |
| `CRANKFIRE_AUTH_PASSWORD` | Same, for `auth.password`. |
| `CRANKFIRE_AUTH_STATIC_TOKEN` | Same, for `auth.static_token`. |
| `OTEL_SERVICE_NAME` | Service name when `tracing.service_name` is empty. |
| `CRANKFIRE_DATA_DIR` | Data directory for `tui`, `session`, `set` and `daemon` (default `~/.crankfire`). |

> **Note:** Config files are **not** environment-expanded. `client_secret: ${MY_SECRET}` sends the literal text `${MY_SECRET}`. Leave the field out and use the matching `CRANKFIRE_AUTH_*` variable instead.

## Validation

Crankfire validates the whole configuration before sending any traffic. It reports every problem at once:

```
validation failed: concurrency must be >= 1; endpoints[1]: weight must be >= 1
```

Common errors:

- `target is required`, unless `har_file` is set or every endpoint has a `url`
- `concurrency must be >= 1`; `rate`, `total`, `timeout`, `retries`, `duration` `must be >= 0`
- `body and bodyFile are mutually exclusive` (globally and per endpoint)
- `dashboard and json-output are mutually exclusive`
- `arrival model "x" is not supported`
- load patterns: missing `type`, `rps must be > 0` for constant/spike, `duration must be > 0`, `steps are required for step pattern`
- endpoints: `weight must be >= 1`, duplicate `name`
- extractors: exactly one of `jsonpath`/`regex`; `var` must be a valid identifier
- auth: the fields each type requires (see [Authentication](authentication.md))
- feeder: `type is required when path is specified`; type must be `csv` or `json`
- protocol: one of the four modes; gRPC requires `service` and `method`; timeouts and `max_events` must not be negative

Thresholds are parsed before the run starts, so a typo fails fast.

Warnings, printed to stderr without stopping the run:

- `rate > 1000` or `concurrency > 500`
- `oauth2_resource_owner` (legacy grant)
- any `oidc_*` auth type (always, because these types use a static token)
- `grpc.insecure: true` with `protocol: grpc`, even when `tls` is off
- HAR files may contain sensitive data

## Exit codes

For a plain `crankfire [flags]` run:

| Code | When |
|------|------|
| `0` | Every threshold passed **and** no request failed. Also returned for `--help`. |
| `1` | Configuration or setup error, a failed threshold (`one or more thresholds failed`), or **at least one failed request** (`N requests failed`), even when all thresholds passed. |

The `set`, `session` and `daemon` subcommands use a richer scheme (0/1/2/3). See [Sets, Scheduling & Session CLI](sets.md#set-run).

## Full example

```yaml
# loadtest.yaml — run with: crankfire --config loadtest.yaml
target: https://api.example.com
protocol: http
method: GET
headers:
  Accept: application/json
  User-Agent: crankfire

concurrency: 50          # max in-flight requests
timeout: 5s              # per HTTP request
retries: 2               # 429, 5xx and network errors; not timeouts
graceful_shutdown: 10s
arrival: poisson

load_patterns:           # drives the rate; run ends after the last one
  - { name: warmup, type: ramp, from_rps: 10, to_rps: 100, duration: 30s }
  - { name: steady, type: constant, rps: 100, duration: 2m }

feeder:
  type: csv
  path: ./users.csv      # relative to the working directory

auth:
  type: oauth2_client_credentials
  token_url: https://idp.example.com/oauth/token
  client_id: load-tester
  # client_secret comes from CRANKFIRE_AUTH_CLIENT_SECRET
  scopes: [api.read]

endpoints:
  - name: profile
    weight: 3
    path: /users/{{user_id}}
  - name: search
    weight: 1
    path: /search?q={{query|shoes}}   # values are inserted as-is: URL-encode them in the CSV

thresholds:
  - "http_req_duration:p95 < 300"
  - "http_req_failed:rate < 0.01"

html_output: report.html
```
{% endraw %}
