# Protocols

Crankfire speaks four protocols out of the box:

- HTTP
- WebSocket
- Server‑Sent Events (SSE)
- gRPC

All protocols share the same scheduling and reporting engine, so you can compare behavior under identical workloads. Select the protocol per run with `--protocol` (or `protocol:` in config); multi-protocol mixes in a single run are not supported yet.

## HTTP

### Basic

```bash
crankfire --target https://api.example.com --total 1000
```

### POST with Body

```bash
crankfire --target https://api.example.com/data \
  --method POST \
  --header "Content-Type=application/json" \
  --body '{"user":"test"}' \
  --concurrency 10 \
  --total 500
```

## WebSocket

Enable WebSocket mode with `--protocol websocket` or `protocol: websocket` in config.

```bash
crankfire --protocol websocket \
  --target ws://localhost:8080/ws \
  --ws-messages '{"type":"ping"}' \
  --ws-message-interval 1s \
  --concurrency 10 \
  --duration 30s
```

WebSocket runs reuse the global headers section, so OAuth tokens (from the `auth` block) and feeder placeholders flow into the handshake plus each message you send.

See [Usage Examples](usage.md) for more recipes.

## Server-Sent Events (SSE)

```bash
crankfire --protocol sse \
  --target http://localhost:8080/events \
  --sse-read-timeout 30s \
  --sse-max-events 100 \
  --concurrency 20 \
  --duration 2m
```

Use feeders to parameterize query strings or headers (e.g., `https://events.example.com/stream?topic={{topic}}`). OAuth headers are injected automatically when configured.

## gRPC

gRPC mode uses a `.proto` file and JSON messages.

```bash
crankfire --protocol grpc \
  --target localhost:50051 \
  --grpc-proto-file ./hello.proto \
  --grpc-service helloworld.Greeter \
  --grpc-method SayHello \
  --grpc-message '{"name":"LoadTest"}' \
  --concurrency 10 \
  --duration 30s
```

Key points:

- `--grpc-proto-file` (or `grpc.proto_file`) should point to the service definition. Crankfire parses descriptors at runtime—no generated Go stubs required.
- The JSON passed to `--grpc-message` must match the request type in the proto. Feeders are resolved before encoding so you can reference fields like `"order_id":"{{order_id}}"`.
- Metadata supplied via `--grpc-metadata key=value` (or `grpc.metadata`) and OAuth tokens both appear as lowercase gRPC metadata headers.
- TLS, insecure, and timeout settings mirror the CLI flags.

See [Usage Examples](usage.md) for TLS, metadata, and feeder integration.
