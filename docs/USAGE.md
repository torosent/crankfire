# Usage Examples

This document provides practical examples for using Crankfire in various scenarios.

## Basic Examples

### Simple GET Request

Test a simple endpoint with 100 requests:

```bash
crankfire --target https://httpbin.org/get --total 100
```

### Concurrent Load Test

Run 1000 requests with 50 concurrent workers:

```bash
crankfire --target https://httpbin.org/get \
  --concurrency 50 \
  --total 1000
```

### Duration-Based Test

Run for 30 seconds with 10 workers:

```bash
crankfire --target https://httpbin.org/get \
  --concurrency 10 \
  --duration 30s
```

## POST Requests

### POST with Inline Body

```bash
crankfire --target https://httpbin.org/post \
  --method POST \
  --header "Content-Type=application/json" \
  --body '{"user":"test","action":"create"}' \
  --total 50
```

### POST with Body from File

Create `payload.json`:
```json
{
  "user": "test",
  "email": "test@example.com"
}
```

Run test:
```bash
crankfire --target https://httpbin.org/post \
  --method POST \
  --header "Content-Type=application/json" \
  --body-file payload.json \
  --concurrency 10 \
  --total 100
```

## Rate Limiting

### Fixed Rate Test

100 requests per second for 1 minute:

```bash
crankfire --target https://api.example.com \
  --rate 100 \
  --duration 1m \
  --concurrency 20
```

### Gradually Increasing Load

Start with low rate, then increase:

```bash
# Phase 1: 10 req/s
crankfire --target https://api.example.com --rate 10 --duration 30s

# Phase 2: 50 req/s
crankfire --target https://api.example.com --rate 50 --duration 30s

# Phase 3: 100 req/s
crankfire --target https://api.example.com --rate 100 --duration 30s
```

## Advanced Workload Modeling

Describe coordinated phases, arrival distributions, and weighted endpoints in a single config file instead of scripting separate runs.

```yaml
target: https://api.example.com
concurrency: 40
load_patterns:
  - name: warmup
    type: ramp
    from_rps: 20
    to_rps: 400
    duration: 5m
  - name: soak
    type: step
    steps:
      - rps: 400
        duration: 8m
      - rps: 600
        duration: 4m
  - name: spike
    type: spike
    rps: 1200
    duration: 20s
arrival:
  model: poisson
endpoints:
  - name: list-users
    weight: 60
    path: /users
  - name: user-detail
    weight: 30
    path: /users/{id}
  - name: create-order
    weight: 10
    path: /orders
    method: POST
    body: '{"status":"new"}'
```

- **Load patterns**: `ramp`, `step`, and `spike` share a single timeline. Each step specifies its own duration; ramps interpolate between `from_rps` and `to_rps` across the provided duration.
- **Arrival models**: Defaults to uniform pacing. Switch to Poisson globally with `arrival:
  model: poisson` or `--arrival-model poisson` for more realistic inter-arrival gaps.
- **Endpoints**: Define multiple endpoints with relative `weight` values. Each endpoint inherits the global target URL, method, headers, and body unless explicitly overridden.

Endpoint weighting automatically feeds the collector. The JSON report, progress ticker, and dashboard expose per-endpoint totals, RPS, and latency percentiles so you can compare how each route behaves during the same run.

## Authentication

Crankfire automates token acquisition, refresh, and header injection for common authentication flows.

### OAuth2 Client Credentials Flow

Automatically fetch and refresh access tokens:

```yaml
target: https://api.example.com
auth:
  type: oauth2_client_credentials
  token_url: https://idp.example.com/oauth/token
  client_id: my-client-id
  client_secret: my-client-secret
  scopes:
    - api:read
    - api:write
  refresh_before_expiry: 30s  # refresh 30s before expiry
concurrency: 20
rate: 100
duration: 10m
```

### OAuth2 Resource Owner Password Flow

Use username and password for token acquisition:

```json
{
  "target": "https://api.example.com",
  "auth": {
    "type": "oauth2_resource_owner",
    "token_url": "https://idp.example.com/oauth/token",
    "client_id": "client-id",
    "client_secret": "client-secret",
    "username": "testuser@example.com",
    "password": "userpassword",
    "scopes": ["openid", "profile"]
  },
  "concurrency": 15,
  "total": 5000
}
```

### OIDC with Static Token

For implicit or auth code flows with pre-configured tokens:

```yaml
target: https://api.example.com
auth:
  type: oidc_implicit
  static_token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0...
concurrency: 10
duration: 5m
```

**Note**: Authentication must be configured via config files. Auth settings are not available as CLI flags to prevent exposing secrets in shell history.

### Manual Authentication (Without Config File)

For APIs not using OAuth2/OIDC, continue using manual header injection:

```bash
crankfire --target https://api.example.com \
  --header "Authorization=Bearer manual-token" \
  --header "X-API-Key=your-api-key" \
  --total 100
```

Or via config:

```json
{
  "target": "https://api.example.com",
  "headers": {
    "Authorization": "Bearer manual-token",
    "X-API-Key": "your-api-key"
  },
  "concurrency": 10,
  "total": 1000
}
```

**Note**: Auth credentials are never cached between runs. Each execution obtains fresh tokens.

## Configuration Files

### JSON Configuration

Create `loadtest.json`:
```json
{
  "target": "https://api.example.com/v1/users",
  "method": "GET",
  "headers": {
    "Authorization": "Bearer your-token-here",
    "Accept": "application/json"
  },
  "concurrency": 25,
  "rate": 50,
  "duration": "2m",
  "timeout": "10s",
  "retries": 2
}
```

Run:
```bash
crankfire --config loadtest.json
```

### YAML Configuration

Create `loadtest.yaml`:
```yaml
target: https://api.example.com/v1/users
method: POST
headers:
  Content-Type: application/json
  Authorization: Bearer your-token-here
body: '{"name":"John Doe","email":"john@example.com"}'
concurrency: 30
rate: 75
duration: 5m
timeout: 15s
retries: 3
```

Run:
```bash
crankfire --config loadtest.yaml
```

### Override Config with Flags

Start with config but override specific values:

```bash
crankfire --config loadtest.json \
  --concurrency 100 \
  --duration 10m
```

## Authentication

### Bearer Token

```bash
crankfire --target https://api.example.com/protected \
  --header "Authorization=Bearer eyJhbGc..." \
  --total 100
```

### API Key

```bash
crankfire --target https://api.example.com/data \
  --header "X-API-Key=your-api-key" \
  --total 100
```

### Basic Auth

```bash
# Use base64 encoded credentials
crankfire --target https://api.example.com/admin \
  --header "Authorization=Basic dXNlcjpwYXNz" \
  --total 50
```

## Data Feeders

### CSV Feeder for User Testing

Create `users.csv`:

```csv
user_id,email,name
101,alice@example.com,Alice
102,bob@example.com,Bob
103,charlie@example.com,Charlie
```

Test with user data:

```bash
crankfire --target "https://api.example.com/users/{{user_id}}" \
  --header "X-User-Email={{email}}" \
  --feeder-path ./users.csv \
  --feeder-type csv \
  --concurrency 10 \
  --rate 50
```

### JSON Feeder for Product API

Create `products.json`:

```json
[
  {"product_id": "p1", "name": "Widget", "price": "19.99"},
  {"product_id": "p2", "name": "Gadget", "price": "29.99"}
]
```

POST with dynamic product data:

```bash
crankfire --target https://api.example.com/products \
  --method POST \
  --header "Content-Type=application/json" \
  --body '{"id":"{{product_id}}","name":"{{name}}","price":{{price}}}' \
  --feeder-path ./products.json \
  --feeder-type json \
  --concurrency 5
```

### Feeder with Config File

`feeder-test.yml`:

```yaml
target: https://api.example.com/search
method: GET
concurrency: 20
rate: 100
duration: 1m

feeder:
  path: ./search-queries.csv
  type: csv

# Queries CSV has columns: query, category, limit
# Target becomes: /search?q={{query}}&category={{category}}&limit={{limit}}
```

Run:

```bash
crankfire --config feeder-test.yml
```

### Complex Placeholder Substitution

Config with multiple placeholders:

```yaml
target: https://api.example.com/v1/{{resource}}/{{id}}
method: PUT
concurrency: 10
rate: 30

feeder:
  path: ./updates.csv
  type: csv

headers:
  X-Resource-ID: "{{id}}"
  X-Resource-Type: "{{resource}}"
  Content-Type: application/json

body: |
  {
    "id": "{{id}}",
    "name": "{{name}}",
    "status": "{{status}}",
    "updated_by": "{{user}}"
  }
```

CSV file (`updates.csv`):

```csv
resource,id,name,status,user
products,p123,Widget Pro,active,admin
orders,o456,Order 456,pending,system
users,u789,Test User,inactive,admin
```

## Reliability Testing

### With Retries

Test flaky endpoints with automatic retries:

```bash
crankfire --target https://flaky-api.example.com \
  --retries 5 \
  --timeout 5s \
  --total 200
```

### Long Timeout for Slow APIs

```bash
crankfire --target https://slow-api.example.com \
  --timeout 60s \
  --concurrency 5 \
  --total 50
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Performance Test
on: [push]
jobs:
  load-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v3
        with:
          go-version: '1.21'
      - name: Install crankfire
        run: go install github.com/torosent/crankfire/cmd/crankfire@latest
      - name: Run load test
        run: |
          crankfire --target ${{ secrets.API_URL }} \
            --header "Authorization: Bearer ${{ secrets.API_TOKEN }}" \
            --total 1000 \
            --concurrency 50 \
            --json-output > results.json
      - name: Check performance
        run: |
          p99=$(jq -r '.p99_latency_ms' results.json)
          if (( $(echo "$p99 > 200" | bc -l) )); then
            echo "P99 latency too high: ${p99}ms"
            exit 1
          fi
```

### JSON Output Processing

Extract metrics with jq:

```bash
crankfire --target https://api.example.com \
  --total 1000 \
  --json-output | jq '{
    total: .total,
    success_rate: (.successes / .total * 100),
    avg_latency: .mean_latency_ms,
    p99_latency: .p99_latency_ms
  }'
```

## Monitoring and Debugging

### Verbose Failure Logging

Non-JSON mode logs errors to stderr:

```bash
crankfire --target https://api.example.com \
  --total 100 \
  2> errors.log
```

### Track Progress in Real-time

Progress updates appear on stderr every second:

```bash
crankfire --target https://api.example.com \
  --duration 5m \
  --concurrency 50
# Shows: Requests: 1234 | Successes: 1230 | Failures: 4 | RPS: 41.2
```

## Performance Benchmarking

### Compare Two Endpoints

```bash
# Test endpoint A
crankfire --target https://api-a.example.com/users \
  --total 1000 \
  --json-output > results-a.json

# Test endpoint B
crankfire --target https://api-b.example.com/users \
  --total 1000 \
  --json-output > results-b.json

# Compare
jq -s '{a: .[0].mean_latency_ms, b: .[1].mean_latency_ms}' \
  results-a.json results-b.json
```

### Stress Test to Find Limits

Gradually increase concurrency:

```bash
for c in 10 25 50 100 200 500; do
  echo "Testing with concurrency: $c"
  crankfire --target https://api.example.com \
    --concurrency $c \
    --total 1000 \
    --json-output | jq '{concurrency: '$c', rps: .requests_per_sec, p99: .p99_latency_ms}'
done
```

## Tips

1. **Start Small**: Begin with low concurrency and gradually increase
2. **Monitor Target**: Watch your target system's resources during tests
3. **Use Rate Limiting**: Prevent overwhelming your own or external APIs
4. **Enable Retries**: For flaky networks or services
5. **JSON Output**: Pipe to `jq` for automated analysis
6. **Config Files**: Reuse test configurations across environments

## Headers

Crankfire supports headers via repeatable `--header` flags or config file maps.

CLI format (use `Key=Value`):

```bash
crankfire --target https://api.example.com \
  --header "Authorization=Bearer token123" \
  --header "Content-Type=application/json" \
  --header "X-Trace-Id=req-99"
```

Config examples:

```json
"headers": {"Authorization": "Bearer token123", "X-Env": "prod"}
```

```yaml
headers:
  Authorization: Bearer token123
  X-Env: prod
```

Rules:
- Keys canonicalized (e.g. `x-custom-id` -> `X-Custom-Id`).
- Empty values allowed: `--header "X-Empty="`.
- Last duplicate wins.
- CLI flags override config values.
- Invalid newline characters in keys/values are rejected.

Override example:

```bash
crankfire --config base.yaml \
  --header "Authorization=Bearer override"
```

Sending empty value:

```bash
crankfire --header "X-Feature-Flag="
```

Avoid colon syntax (`Key: Value`). Always use `Key=Value`.

## WebSocket Testing

### Basic WebSocket Load Test

Test WebSocket endpoints with connection management and message exchange:

```bash
crankfire --protocol websocket \
  --target ws://localhost:8080/ws \
  --ws-messages '{"type":"ping"}' \
  --ws-messages '{"type":"subscribe","channel":"updates"}' \
  --ws-message-interval 1s \
  --concurrency 10 \
  --duration 30s
```

### WebSocket with Authentication

```bash
crankfire --protocol websocket \
  --target wss://api.example.com/ws \
  --header "Authorization=Bearer token123" \
  --ws-messages '{"action":"join","room":"lobby"}' \
  --ws-receive-timeout 15s \
  --concurrency 50 \
  --rate 100
```

### WebSocket Config File

Create `websocket-test.yml`:

```yaml
protocol: websocket
target: ws://localhost:8080/chat
concurrency: 20
rate: 50
duration: 2m

websocket:
  messages:
    - '{"type": "authenticate", "token": "test123"}'
    - '{"type": "join", "channel": "general"}'
    - '{"type": "ping"}'
  message_interval: 2s
  receive_timeout: 10s
  handshake_timeout: 30s

headers:
  X-Client-ID: "loadtest-{{worker_id}}"
  User-Agent: "Crankfire/1.0"
```

Run the test:

```bash
crankfire --config websocket-test.yml
```

### WebSocket Testing with Data Feeders

Combine WebSocket with CSV data for dynamic messages:

Create `users.csv`:
```csv
user_id,username,channel
101,alice,general
102,bob,random
103,charlie,support
```

Create `ws-feeder-test.yml`:
```yaml
protocol: websocket
target: ws://localhost:8080/chat
concurrency: 10
duration: 1m

feeder:
  path: ./users.csv
  type: csv

websocket:
  messages:
    - '{"action":"auth","user_id":"{{user_id}}"}'
    - '{"action":"join","channel":"{{channel}}","username":"{{username}}"}'
  message_interval: 1s
```

### WebSocket Metrics

WebSocket tests provide streaming-specific metrics:

```bash
crankfire --protocol websocket \
  --target ws://localhost:8080/ws \
  --ws-messages '{"ping":true}' \
  --concurrency 10 \
  --total 100 \
  --json-output
```

Output includes:
- `connection_duration_ms`: Time connected per worker
- `messages_sent`: Total messages sent
- `messages_received`: Total messages received
- `bytes_sent`: Total bytes transmitted
- `bytes_received`: Total bytes received
- `errors`: Connection and message errors

## Server-Sent Events (SSE) Testing

### Basic SSE Load Test

Test SSE endpoints with event streaming:

```bash
crankfire --protocol sse \
  --target http://localhost:8080/events \
  --sse-read-timeout 30s \
  --sse-max-events 100 \
  --concurrency 20 \
  --duration 2m
```

### SSE with Headers

```bash
crankfire --protocol sse \
  --target https://api.example.com/stream \
  --header "Authorization=Bearer token123" \
  --header "Accept=text/event-stream" \
  --sse-read-timeout 60s \
  --concurrency 50
```

### SSE Config File

Create `sse-test.json`:

```json
{
  "protocol": "sse",
  "target": "http://localhost:8080/events",
  "concurrency": 30,
  "rate": 100,
  "duration": "5m",
  "sse": {
    "read_timeout": "45s",
    "max_events": 500
  },
  "headers": {
    "Accept": "text/event-stream",
    "Authorization": "Bearer token123",
    "X-Client-Type": "loadtest"
  }
}
```

Run:

```bash
crankfire --config sse-test.json
```

### SSE with Query Parameters

Test SSE endpoints with dynamic query strings using feeders:

Create `topics.csv`:
```csv
topic,filter
sports,live
news,breaking
tech,updates
```

Create `sse-feeder-test.yml`:
```yaml
protocol: sse
target: http://localhost:8080/stream?topic={{topic}}&filter={{filter}}
concurrency: 15
duration: 3m

feeder:
  path: ./topics.csv
  type: csv

sse:
  read_timeout: 30s
  max_events: 200
```

### SSE Metrics

SSE tests provide event streaming metrics:

```bash
crankfire --protocol sse \
  --target http://localhost:8080/events \
  --concurrency 10 \
  --total 50 \
  --json-output
```

Output includes:
- `connection_duration_ms`: Streaming duration per connection
- `events_received`: Total SSE events received
- `bytes_received`: Total data streamed
- `errors`: Connection and read errors

### SSE Event Filtering

Monitor specific event types by parsing JSON output:

```bash
crankfire --protocol sse \
  --target http://localhost:8080/events \
  --concurrency 5 \
  --duration 1m \
  --json-output | jq '{
    total_events: .events_received,
    avg_event_rate: (.events_received / (.duration_ms / 1000)),
    bytes_per_second: (.bytes_received / (.duration_ms / 1000))
  }'
```

## gRPC Testing

### Basic gRPC Load Test

Test gRPC services with proto file compilation and message templates:

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

### gRPC with TLS

```bash
crankfire --protocol grpc \
  --target api.example.com:443 \
  --grpc-proto-file ./service.proto \
  --grpc-service api.Service \
  --grpc-method GetData \
  --grpc-message '{"id":"123"}' \
  --grpc-tls \
  --concurrency 20 \
  --rate 100
```

### gRPC Config File

Create `grpc-test.yml`:

```yaml
protocol: grpc
target: localhost:50051
concurrency: 20
rate: 100
duration: 2m

grpc:
  proto_file: ./api/service.proto
  service: myapp.UserService
  method: GetUser
  message: '{"user_id": "{{user_id}}"}'
  timeout: 5s
  tls: false
  insecure: false
  metadata:
    authorization: "Bearer token123"
    x-request-id: "loadtest-{{request_id}}"
```

Run:

```bash
crankfire --config grpc-test.yml
```

### gRPC with Data Feeders

Combine gRPC with CSV data for dynamic message payloads:

Create `users.csv`:
```csv
user_id,email,role
101,alice@example.com,admin
102,bob@example.com,user
103,charlie@example.com,user
```

Create `grpc-feeder-test.yml`:
```yaml
protocol: grpc
target: localhost:50051
concurrency: 15
rate: 50
duration: 1m

grpc:
  proto_file: ./users.proto
  service: users.UserService
  method: UpdateUser
  message: |
    {
      "user_id": "{{user_id}}",
      "email": "{{email}}",
      "role": "{{role}}",
      "updated_at": "2024-01-15T10:00:00Z"
    }
  timeout: 10s

feeder:
  path: ./users.csv
  type: csv
```

### gRPC with Authentication

Combine gRPC with OAuth2 authentication for secured services:

```yaml
protocol: grpc
target: api.example.com:443
concurrency: 20
rate: 100
duration: 5m

auth:
  type: oauth2_client_credentials
  token_url: https://idp.example.com/oauth/token
  client_id: grpc-client-id
  client_secret: grpc-client-secret
  scopes:
    - grpc.service.read
    - grpc.service.write

grpc:
  proto_file: ./orders.proto
  service: orders.OrderService
  method: CreateOrder
  message: '{"customer_id":"{{customer_id}}","amount":{{amount}}}'
  tls: true
  insecure: false
  timeout: 15s

feeder:
  path: ./orders.csv
  type: csv
```

The auth token is automatically injected into gRPC metadata as `authorization: Bearer <token>`.

### gRPC Metrics

gRPC tests track:
- **Calls**: Total gRPC calls made
- **Responses**: Successful responses received
- **Errors**: Call failures, timeouts, and status codes
- **Latency**: Per-call latency distribution

Example JSON output:

```bash
crankfire --protocol grpc \
  --target localhost:50051 \
  --grpc-proto-file ./service.proto \
  --grpc-service test.Service \
  --grpc-method Test \
  --grpc-message '{"test":true}' \
  --concurrency 10 \
  --total 100 \
  --json-output
```

Output includes protocol metrics:
```json
{
  "total": 100,
  "successes": 98,
  "failures": 2,
  "protocol_metrics": {
    "grpc": {
      "calls": 100,
      "responses": 98,
      "errors": 2
    }
  }
}
```

## Combined Scenarios

Crankfire supports combining multiple features for realistic, complex load testing scenarios.

### OAuth2 + CSV Feeder + HTTP

Test authenticated API with per-user data:

Create `users.csv`:
```csv
user_id,department,role
101,engineering,developer
102,sales,manager
103,support,agent
```

Create `auth-feeder-test.yml`:
```yaml
target: https://api.example.com/users/{{user_id}}/profile
method: GET
concurrency: 25
rate: 100
duration: 5m

auth:
  type: oauth2_client_credentials
  token_url: https://idp.example.com/oauth/token
  client_id: api-client
  client_secret: secret123
  scopes:
    - users:read

feeder:
  path: ./users.csv
  type: csv

headers:
  X-User-Department: "{{department}}"
  X-User-Role: "{{role}}"
```

Run:
```bash
crankfire --config auth-feeder-test.yml
```

### WebSocket + OAuth2 + Poisson Arrivals

Realistic WebSocket chat load with authenticated connections:

```yaml
protocol: websocket
target: wss://chat.example.com/ws
concurrency: 50
rate: 200
duration: 10m

auth:
  type: oauth2_client_credentials
  token_url: https://idp.example.com/oauth/token
  client_id: chat-client
  client_secret: secret456
  scopes:
    - chat:connect
    - chat:send

websocket:
  messages:
    - '{"action":"join","room":"general"}'
    - '{"action":"message","text":"Hello from loadtest"}'
  message_interval: 2s
  receive_timeout: 15s

arrival:
  model: poisson  # Natural clustering of connections
```

### SSE + JSON Feeder + Multi-Endpoint

Test event streaming across multiple topics with dynamic subscriptions:

Create `subscriptions.json`:
```json
[
  {"topic": "sports", "filter": "live", "priority": "high"},
  {"topic": "news", "filter": "breaking", "priority": "medium"},
  {"topic": "tech", "filter": "updates", "priority": "low"}
]
```

Create `sse-multi-endpoint.yml`:
```yaml
protocol: sse
concurrency: 30
rate: 150
duration: 3m

feeder:
  path: ./subscriptions.json
  type: json

sse:
  read_timeout: 45s
  max_events: 200

endpoints:
  - name: subscribe-stream
    weight: 80
    url: https://stream.example.com/events?topic={{topic}}&filter={{filter}}
    headers:
      X-Priority: "{{priority}}"
  - name: health-check
    weight: 20
    url: https://stream.example.com/health
    method: GET
```

### gRPC + OAuth2 + CSV Feeder + Load Patterns

Complex gRPC test with ramp-up, authentication, and data injection:

Create `orders.csv`:
```csv
order_id,customer_id,amount,items
ord-001,cust-101,49.99,3
ord-002,cust-102,79.99,5
ord-003,cust-103,29.99,1
```

Create `grpc-complete-test.yml`:
```yaml
protocol: grpc
target: orders.example.com:50051
concurrency: 40

load_patterns:
  - name: warmup
    type: ramp
    from_rps: 10
    to_rps: 100
    duration: 2m
  - name: steady
    type: step
    steps:
      - rps: 100
        duration: 5m
  - name: spike
    type: spike
    rps: 500
    duration: 30s

auth:
  type: oauth2_client_credentials
  token_url: https://idp.example.com/oauth/token
  client_id: orders-service
  client_secret: secret789
  scopes:
    - orders:create
  refresh_before_expiry: 60s

grpc:
  proto_file: ./api/orders.proto
  service: orders.OrderService
  method: CreateOrder
  message: |
    {
      "order_id": "{{order_id}}",
      "customer_id": "{{customer_id}}",
      "amount": {{amount}},
      "items_count": {{items}},
      "timestamp": "{{timestamp}}"
    }
  timeout: 10s
  tls: true
  insecure: false
  metadata:
    x-trace-id: "load-{{order_id}}"

feeder:
  path: ./orders.csv
  type: csv

arrival:
  model: poisson
```

Run with dashboard:
```bash
crankfire --config grpc-complete-test.yml --dashboard
```

### Multi-Protocol Report

Test multiple protocols in a single run with weighted endpoints:

```yaml
concurrency: 50
rate: 200
duration: 5m

arrival:
  model: poisson

endpoints:
  - name: http-api
    weight: 50
    protocol: http
    url: https://api.example.com/data
    method: GET
  
  - name: websocket-feed
    weight: 30
    protocol: websocket
    url: ws://stream.example.com/ws
    websocket:
      messages:
        - '{"subscribe":"updates"}'
      message_interval: 1s
  
  - name: sse-events
    weight: 15
    protocol: sse
    url: https://events.example.com/stream
    sse:
      read_timeout: 30s
      max_events: 100
  
  - name: grpc-service
    weight: 5
    protocol: grpc
    target: grpc.example.com:50051
    grpc:
      proto_file: ./service.proto
      service: api.Service
      method: Process
      message: '{"data":"test"}'
```

The JSON output will include protocol-specific metrics for each protocol:

```json
{
  "total": 10000,
  "protocol_metrics": {
    "http": {
      "requests": 5000,
      "successes": 4998
    },
    "websocket": {
      "connections": 3000,
      "messages_sent": 15000,
      "messages_received": 14850
    },
    "sse": {
      "connections": 1500,
      "events_received": 75000
    },
    "grpc": {
      "calls": 500,
      "responses": 498
    }
  }
}
```

### Feature Matrix Summary

| Scenario | Auth | Feeder | Protocol | Arrival | Load Patterns |
|----------|:----:|:------:|:--------:|:-------:|:-------------:|
| OAuth2 + CSV + HTTP | ✅ | ✅ | HTTP | ⚪ | ⚪ |
| WebSocket + OAuth2 + Poisson | ✅ | ⚪ | WebSocket | ✅ | ⚪ |
| SSE + JSON + Endpoints | ⚪ | ✅ | SSE | ⚪ | ⚪ |
| gRPC + OAuth2 + CSV + Patterns | ✅ | ✅ | gRPC | ✅ | ✅ |
| Multi-Protocol Report | ⚪ | ⚪ | All | ✅ | ⚪ |

All features can be combined in any configuration. These scenarios demonstrate common patterns.

````
