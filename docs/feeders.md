---
layout: default
title: Data Feeders
---
{% raw %}

# Data Feeders

A feeder reads records from a CSV or JSON file and hands one record to each request. The record's fields fill `{{placeholders}}` in the URL, headers, body, WebSocket messages, and the gRPC target, message and metadata.

```bash
crankfire --target "https://api.example.com/users/{{user_id}}" \
  --feeder-path users.csv --feeder-type csv \
  --concurrency 10 --duration 30s
```

```yaml
feeder:
  path: ./users.csv   # relative to the working directory
  type: csv           # csv | json (required when path is set)
```

## CSV

The first row holds the field names. Every data row must have the same number of columns, and the file needs at least one data row. Leading spaces in fields are trimmed.

```csv
user_id,email,role
101,alice@example.com,admin
102,bob@example.com,user
103,charlie@example.com,user
```

```yaml
target: https://api.example.com/users/{{user_id}}
feeder: { path: ./users.csv, type: csv }
headers:
  X-User-Email: "{{email}}"
  X-User-Role: "{{role}}"
```

## JSON

The file must be a non-empty array of objects:

```json
[
  {"product_id": "p1001", "name": "Widget", "price": 49.99},
  {"product_id": "p1002", "name": "Gadget", "price": 19.5}
]
```

```yaml
target: https://api.example.com/cart
method: POST
feeder: { path: ./products.json, type: json }
headers:
  Content-Type: application/json
body: '{"product_id":"{{product_id}}","price":{{price}}}'
```

Every value becomes a string. Numbers keep their original digits, `null` becomes `<nil>`, and nested objects or arrays become Go's `map[...]` / `[...]` text, so keep records flat.

## How records are handed out

- **One record per request**, for every protocol. A request is one HTTP exchange, one WebSocket send-and-receive cycle, one SSE read, or one gRPC call.
- **Round-robin, shared by all workers.** The file is read in order and each worker gets the next record. Which record a given worker gets depends on timing.
- **Feeders wrap around and never run out.** After the last record, reading starts again from the first. Use `total` or `duration` to bound the run.
- **One record covers the whole request.** Every placeholder in that request (URL, headers, body) reads from the same record, so fields from one row stay together.
- **Retries take a new record.** With `retries`, every retry attempt takes the next record, so a retried request is not resent with the same data.

## Placeholders by protocol

| Protocol | Where placeholders apply | Default values |
|----------|--------------------------|:------------------:|
| HTTP | URL, endpoint paths, header values, body | ✅ |
| WebSocket | URL, header values, each message | — |
| SSE | URL, header values | — |
| gRPC | target, `grpc.message`, `grpc.metadata` values | — |

In HTTP mode, placeholders also look up variables extracted from earlier responses. They are checked before feeder fields. See [Request Chaining](request-chaining.md) for the lookup order and default syntax.

> **Note:** A placeholder that matches nothing is replaced with an **empty string** in HTTP mode. WebSocket, SSE and gRPC **leave it unchanged** (a literal `{{missing}}`). Check your column names: matching is exact and case-sensitive, and spaces are **not** trimmed, so `{{ user_id }}` looks up a field named `" user_id "`.

> **Note:** Values are inserted **as-is**, with no escaping. URL-encode values that go into URLs (`red%20shoes`, not `red shoes`), and JSON-escape values that go into JSON bodies.

## Sample files

Runnable examples live in [`scripts/doc-samples`](https://github.com/torosent/crankfire/tree/main/scripts/doc-samples), including `users.csv`, `products.json` and feeder configs for each protocol. `scripts/verify-doc-samples.sh` checks that they still parse.
{% endraw %}
