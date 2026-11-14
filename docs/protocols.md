---
layout: default
title: Protocols
---
{% raw %}

# Protocols

Crankfire load-tests HTTP, WebSocket, Server-Sent Events and gRPC. One run uses one protocol, set with `--protocol` or `protocol:`. All four share the same scheduler, rate limiting, arrival models, load patterns, feeders, auth, thresholds and reports. Each one defines a single "request" differently, as described below.

## Feature support

| Feature | HTTP | WebSocket | SSE | gRPC |
|---------|:----:|:---------:|:---:|:----:|
| Rate, arrival models, load patterns | ✅ | ✅ | ✅ | ✅ |
| Feeders (`{{key}}`) | ✅ | ✅ | ✅ | ✅ |
| Placeholder default values | ✅ | — | — | — |
| Auth (Bearer token) | header | handshake header | header | `authorization` metadata |
| Weighted `endpoints` | ✅ | — | — | — |
| Request chaining (extractors) | ✅ | — | — | — |
| Retries, `--log-errors` | ✅ | — | — | — |
| Trace propagation | ✅ | ✅ | ✅ | ✅ |
| Thresholds, dashboard, JSON and HTML reports | ✅ | ✅ | ✅ | ✅ |

## HTTP

```bash
crankfire --target https://api.example.com/data \
  --method POST \
  --header "Content-Type=application/json" \
  --body '{"user":"test"}' \
  --concurrency 10 --total 500
```

**One request** is one HTTP exchange, and the response body is always read to the end. Latency covers feeder lookup, any token fetch, request building, the round trip and the full body download.

- **Transport.** One shared client per run. HTTP/2 is used when the server supports it. Up to 256 idle connections are kept in total and 32 per host, and they are dropped after 90s idle. The TLS handshake times out after 10s. Proxy settings come from the environment (`HTTPS_PROXY` and similar).
- **Timeout.** `timeout` (default 30s) bounds the whole exchange. `0` means no timeout. Use `500ms`, not `0.5` (a bare number is whole seconds).
- **Redirects** are followed, up to 10 by Go's default policy. Only the final status counts.
- **Failure** means a transport error or a final status of `400` or higher. `1xx`, `2xx` and `3xx` count as successes.
- **Body handling.** The body is buffered (up to 1 MB) only when the status is at least 400 or the endpoint has extractors. Otherwise it is discarded as it streams in.

## WebSocket

```bash
crankfire --protocol websocket \
  --target ws://localhost:8080/ws \
  --ws-messages ping \
  --ws-message-interval 500ms \
  --ws-receive-timeout 2s \
  --concurrency 10 --duration 30s
```

```yaml
protocol: websocket
target: wss://chat.example.com/ws?room={{room}}
websocket:
  messages:
    - '{"type":"join","room":"{{room}}"}'
    - '{"type":"message","text":"hello"}'
  message_interval: 200ms
  receive_timeout: 2s
  handshake_timeout: 10s
```

**One request** goes through these steps:

1. Take a pooled connection, or dial a new one. Dialing uses `handshake_timeout`, default 30s.
2. Send each entry in `messages` as a **text frame**. If you set `message_interval` and there is more than one message, Crankfire pauses that long after **every** message, including the last.
3. If `receive_timeout > 0`, read incoming messages until the window ends. The request always lasts the full window; it does not stop after a reply.

Behaviour to know about:

- **Receiving is off by default.** `receive_timeout` defaults to `0` unless you set it (the `--help` default of 10s is not applied). With it off, server replies are never read.
- **Connection reuse.** Connections are pooled by target URL plus headers. A connection goes back to the pool only when the request ended without an error **and** without a receive timeout, so in practice reuse happens only when receiving is off. If the first send on a reused connection fails, Crankfire reconnects once and retries.
- **Latency** includes connecting when needed, every send, every interval pause and the full receive window.
- **Placeholders** from feeders are applied to the URL, the headers and every message.
- **Server closes fail the request.** During the receive window, any read error other than the timeout is a failure, including a normal close by the server. The status bucket is the close code, such as `1000`.
- **JSON messages belong in the config file.** `--ws-messages` splits on commas and rejects double quotes, so use plain values on the command line (see [Configuration](configuration.md#how-configuration-is-resolved)).

## Server-Sent Events (SSE)

```bash
crankfire --protocol sse \
  --target "https://events.example.com/stream?topic={{topic}}" \
  --feeder-path topics.csv --feeder-type csv \
  --sse-read-timeout 10s \
  --sse-max-events 50 \
  --concurrency 20 --duration 2m
```

**One request** goes through these steps:

1. Take a pooled stream, or open a new one. Crankfire sends a `GET` with `Accept: text/event-stream`, and a response other than `200` is a failure.
2. Read events until `max_events` have arrived or `read_timeout` has passed, whichever comes first.

Behaviour to know about:

- **Ending at the timeout is normal**, not a failure. A timed-out stream is closed. A stream that reached `max_events` goes back to the pool.
- **`max_events: 0` means 100**, not "unlimited".
- **An early close fails the request.** If the server closes the stream before `max_events` or the timeout, the request fails (`read event: connection closed`, bucket `WRAPERROR`). The only exception is the first read on a pooled stream, which reconnects once.
- **The stream's lifetime is capped.** The SSE HTTP client's timeout equals `read_timeout` (30s when unset). It covers the stream's whole life, so even a pooled stream is cut off at most that long after it connected.
- **Events.** The parser handles `data:`, `event:` and `id:` fields, ignores `:` comments, and joins multi-line `data` with newlines. `retry:` is ignored.

## gRPC

```bash
crankfire --protocol grpc \
  --target localhost:50051 \
  --grpc-proto-file ./protos/hello.proto \
  --grpc-service helloworld.Greeter \
  --grpc-method SayHello \
  --grpc-message '{"name":"{{name}}"}' \
  --grpc-metadata "x-tenant=acme" \
  --grpc-timeout 2s \
  --concurrency 10 --duration 30s
```

```yaml
protocol: grpc
target: orders.example.com:443
grpc:
  proto_file: ./protos/orders.proto
  service: shop.v1.OrderService
  method: CreateOrder
  message: '{"user_id":"{{user_id}}","sku":"{{sku}}"}'
  metadata:
    x-request-source: crankfire
  timeout: 2s
  tls: true
```

**One request** is one **unary** call. Streaming RPCs are not supported.

- **Proto loading.** Crankfire parses `proto_file` once, at the first request; no generated stubs are needed. Imports resolve relative to the proto file's own directory. If parsing fails, every request fails with `load proto descriptor`. `proto_file` is not checked during validation, so a missing file only shows up when the run starts.
- **Service name.** Use the **fully qualified** name (`package.Service`). The name is used as written in the wire path `/<service>/<method>`, so a short name such as `Greeter` can find the descriptor but still get `Unimplemented` from the server.
- **Message.** The JSON in `message` must match the request type. It defaults to `{}`.
- **Metadata.** Keys are trimmed and lowercased, and values support placeholders. When auth is configured, Crankfire adds `authorization: Bearer <token>`.
- **Timeout.** Only `grpc.timeout` / `--grpc-timeout` applies. The global `timeout` and `headers` are ignored. With no timeout set, calls have **no deadline**.
- **Connections.** Crankfire keeps one connection per target, shared by all workers over HTTP/2.

TLS options:

| `tls` | `insecure` | Result |
|:-----:|:----------:|--------|
| `false` | any | Plaintext (h2c) |
| `true` | `false` | TLS verified against the system roots |
| `true` | `true` | TLS with **no** certificate verification |

A warning is printed whenever `insecure: true` is set with `protocol: grpc`, even when `tls` is off.

## Protocol metrics

WebSocket, SSE and gRPC report extra counters. They appear in the dashboard, in the text report under `Protocol Metrics:`, and in JSON under `protocol_metrics.<protocol>`. Numeric values are summed over all requests.

| Protocol | Keys |
|----------|------|
| `websocket` | `messages_sent`, `messages_received`, `bytes_sent`, `bytes_received`, `connection_duration_ms` |
| `sse` | `events_received`, `bytes_received`, `connection_duration_ms` |
| `grpc` | `messages_sent`, `messages_received`, `bytes_sent`, `bytes_received`, `status_code` (always `OK`) |

WebSocket and SSE record these counters for failed requests too. gRPC records them only for successful calls; failed calls appear under [Status buckets](#status-buckets).

```bash
crankfire --config sse.yml --json-output | jq '.protocol_metrics.sse.events_received'
```

> **Note:** `connection_duration_ms` is summed over all requests, so it is not the duration of any one connection.

## Status buckets

Failed requests are grouped by protocol and code. The groups appear in the text report, the dashboard and JSON (`status_buckets`). Successful requests never appear here.

| Protocol | Code | Example |
|----------|------|---------|
| `http` | Numeric status for responses `>= 400` | `503` |
| `websocket` | WebSocket close code | `1006` |
| `grpc` | gRPC status name, with underscores removed | `UNAVAILABLE`, `DEADLINEEXCEEDED` |
| any | Go error type name for transport and setup errors | see below |

Error-type codes:

| Code | Typical cause |
|------|---------------|
| `ERROR` | HTTP transport error, including client timeouts and connection refused |
| `WRAPERROR` | A wrapped setup error: feeder, request build, WebSocket handshake failure, most SSE read errors |
| `STATUSERROR` | SSE connect returned a status other than 200. The status number is not kept. |
| `ERRORSTRING` | Plain errors, including cancellation |

> **Note:** Error-type codes say which kind of failure happened, not why. Use `--log-errors` (HTTP only) to see each failure, or check the server's logs.
{% endraw %}
