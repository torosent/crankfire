---
layout: default
title: Data Feeders
---

# Data Feeders

Data feeders allow you to drive tests with realistic, per-request data from CSV or JSON files. Records are pulled round-robin and exposed as `{{placeholders}}` inside URLs, headers, HTTP bodies, WebSocket/SSE payloads, and gRPC message JSON/metadata.

> **Doc Samples Ready To Run**
> Every `users.csv`, `products.json`, and config file referenced below already exists under `scripts/doc-samples`. You can reuse those files directly or run `scripts/verify-doc-samples.sh` to make sure they keep parsing.

## When to Use Feeders

- Testing APIs that depend on user IDs, product IDs, or other identifiers.
- Sending varied request bodies instead of a single static payload.
- Simulating traffic across many tenants, accounts, or partitions.

## CSV Feeders

`users.csv`:

```csv
user_id,email,name,role
101,alice@example.com,Alice Smith,admin
102,bob@example.com,Bob Johnson,user
103,charlie@example.com,Charlie Brown,user
```

`loadtest.yml`:

```yaml
target: https://api.example.com/users/{{user_id}}
method: GET
concurrency: 10
rate: 50
duration: 30s

feeder:
  path: ./users.csv
  type: csv

headers:
  X-User-Email: "{{email}}"
  X-User-Role: "{{role}}"
```

Run:

```bash
crankfire --config loadtest.yml
```

## JSON Feeders

`products.json`:

```json
[
  {"product_id": "p1001", "name": "Widget", "price": "49.99"},
  {"product_id": "p1002", "name": "Gadget", "price": "79.99"}
]
```

Config:

```json
{
  "target": "https://api.example.com/products",
  "method": "POST",
  "concurrency": 5,
  "rate": 20,
  "duration": "1m",
  "feeder": {
    "path": "./products.json",
    "type": "json"
  },
  "headers": {
    "Content-Type": "application/json"
  },
  "body": "{\"id\": \"{{product_id}}\", \"name\": \"{{name}}\", \"price\": {{price}}}"
}
```

## Placeholder Syntax

- `{{field}}` can be used in URLs, headers, and bodies.
- Missing fields are left as-is.
- Feeders step through rows/records in deterministic round‑robin order across workers.

## Exhaustion Behavior

When the feeder runs out of data, the run fails fast instead of silently reusing or skipping records. This helps catch misconfigured totals versus available test data.

If you want the test to end when data is exhausted, set `--total` high and rely on feeder exhaustion to terminate.

## Combining With Auth and Protocols

Feeders work for all supported protocols (HTTP, WebSocket, SSE, gRPC). See [Usage Examples](usage.md) for full scenarios.
