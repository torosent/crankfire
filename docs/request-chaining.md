---
layout: default
title: Request Chaining
---

# Request Chaining

Request chaining allows you to extract values from API responses and use them in subsequent requests. This is essential for testing workflows where later requests depend on data returned by earlier ones.

Common use cases:
- Create a resource and use its ID in subsequent requests
- Extract authentication tokens and pass them to other endpoints
- Extract session IDs for stateful testing
- Coordinate multi-step API workflows in a single load test

## How It Works

1. **Extract**: Define extractors on an endpoint to pull values from its response
2. **Store**: Extracted values are stored in a per-worker variable store
3. **Use**: Reference extracted values in URLs, headers, and bodies of subsequent requests using `{{variable_name}}` placeholders
4. **Fallback**: Optionally provide default values with `{{variable_name|default_value}}` syntax

## Extractor Configuration

Extractors are defined at the endpoint level in the `extractors` array. Each extractor specifies:

- **jsonpath** or **regex**: The extraction pattern (mutually exclusive)
- **var**: The variable name to store the extracted value
- **on_error** (optional, default false): Whether to extract from error responses (4xx/5xx)

### JSON Path Extraction

Extract values from JSON responses using JSON path notation.

```yaml
endpoints:
  - name: create-user
    path: /users
    method: POST
    body: '{"name":"Alice","email":"alice@example.com"}'
    extractors:
      - jsonpath: "id"                    # Simple field
        var: "user_id"
      - jsonpath: "metadata.session_id"   # Nested field
        var: "session_id"
      - jsonpath: "items.0.sku"          # Array index
        var: "first_sku"
      - jsonpath: "$"                    # Entire JSON object
        var: "full_response"
```

JSON path formats supported:
- `id` or `$.id` – Top-level field
- `user.profile.name` or `$.user.profile.name` – Nested fields
- `items.0.id` – Array access by index
- `$` – Entire response as a string

### Regex Extraction

Extract values from any response using regular expressions with capture groups.

```yaml
endpoints:
  - name: get-token
    path: /auth/token
    method: GET
    extractors:
      - regex: '"access_token":"([^"]+)"'  # Capture first group
        var: "token"
      - regex: '\d+'                       # Match first number
        var: "count"
```

If your regex has capture groups (parentheses), the first capture group is extracted. If no capture group exists, the entire match is extracted.

## Using Extracted Values

Once extracted, values are available as placeholders in:
- **URL paths and query strings**
- **HTTP headers**
- **Request bodies**

### Example: Complete Workflow

```yaml
target: https://api.example.com

endpoints:
  - name: create-order
    path: /orders
    method: POST
    weight: 10
    body: '{"items":2,"total":99.99}'
    extractors:
      - jsonpath: "id"
        var: "order_id"
      - jsonpath: "confirmation_code"
        var: "confirmation"

  - name: get-order
    path: /orders/{{order_id}}
    method: GET
    weight: 5
    extractors:
      - jsonpath: "status"
        var: "order_status"

  - name: confirm-order
    path: /orders/{{order_id}}/confirm
    method: POST
    weight: 1
    body: '{"code":"{{confirmation|default-code}}"}'
    headers:
      X-Order-Status: "{{order_status}}"

concurrency: 10
rate: 50
duration: 1m
```

In this example:
1. `create-order` extracts `order_id` and `confirmation` from the response
2. `get-order` uses `{{order_id}}` in its URL path
3. `confirm-order` uses both `{{order_id}}` in the URL and `{{confirmation|default-code}}` in the body (with a default fallback)
4. `confirm-order` also uses `{{order_status}}` (extracted by `get-order`) in a header

## Default Values

If an extracted value is not found (extraction fails), the variable remains empty. You can specify a fallback default using the `|` syntax:

```yaml
# Without default - placeholder stays as-is if extraction fails
path: /users/{{user_id}}

# With default - use "anonymous" if user_id extraction fails
path: /users/{{user_id|anonymous}}

# Empty default - remove placeholder if not found
value: "Bearer {{token|}}"  # Results in "Bearer " if token not found
```

Default values work with both extracted variables and feeder data.

## Extraction from Error Responses

By default, extractors only run on successful responses (2xx status codes). To extract from error responses (4xx/5xx), set `on_error: true`:

```yaml
extractors:
  - jsonpath: "error_code"
    var: "error_code"
    on_error: true  # Extract even from 400/500 responses
```

This is useful for capturing error IDs, error messages, or retry tokens from failed responses.

## Variable Scope and Persistence

- **Per-worker**: Each worker (concurrent thread) has its own variable store
- **Persistent within worker**: Variables persist across all requests made by a single worker
- **Isolated**: Workers don't share variables with each other

This means:
- Worker 1 can extract `user_id=123` and use it in all its subsequent requests
- Worker 2 can extract `user_id=456` independently
- Variables are cleared when a worker finishes

## Combining with Feeders

Extractors and feeders both provide placeholders. If a value exists in both, **extractors take precedence**:

```yaml
feeder:
  path: ./users.csv  # Contains: user_id, email

endpoints:
  - name: create-session
    path: /users/{{user_id}}  # Uses feeder user_id
    method: POST
    extractors:
      - jsonpath: "session_id"
        var: "user_id"  # Overwrites feeder user_id

  - name: use-session
    path: /sessions/{{user_id}}  # Uses extracted session_id
    method: GET
```

## Best Practices

1. **Use descriptive variable names**: `{{order_id}}` is clearer than `{{id}}`
2. **Test extraction patterns locally**: Use `jq` or regex testers before adding to config
3. **Handle missing values**: Always consider using defaults for optional extractions
4. **Verify extraction works**: Check logs for "JSONPath not found" warnings
5. **Keep JSON paths simple**: Complex nested paths are fragile; consider API redesign if needed
6. **Use weights**: Coordinate request frequency so dependent requests run after their dependencies
7. **Regex capture groups**: Always use `()` for single captures to avoid extracting too much

## Limitations

- **JSON only for jsonpath**: The `jsonpath` extractor requires valid JSON responses
- **Single value per extractor**: Each extractor extracts one value; use multiple extractors for multiple values
- **No transformation**: Extracted values are used as-is; no post-processing is available
- **No cross-worker sharing**: Variables are isolated per worker and cannot be shared between workers
- **No nested variable references**: You cannot use `{{outer_{{inner}}}}` syntax
- **Regex is greedy**: Use non-greedy patterns `(.*?)` if needed to avoid overmatching

## Example: Multi-Step Order Workflow

```yaml
target: https://orders-api.example.com

endpoints:
  # Step 1: Create order and extract ID
  - name: create-order
    path: /v1/orders
    method: POST
    weight: 10
    body: |
      {
        "customer_id": "{{customer_id}}",
        "items": [
          {"sku": "WIDGET-1", "qty": 2},
          {"sku": "GADGET-1", "qty": 1}
        ]
      }
    extractors:
      - jsonpath: "order_id"
        var: order_id
      - jsonpath: "invoice_url"
        var: invoice_url

  # Step 2: Get order details and extract shipment ID
  - name: get-order
    path: /v1/orders/{{order_id}}
    method: GET
    weight: 5
    extractors:
      - jsonpath: "shipment.tracking_number"
        var: tracking_number
        on_error: false

  # Step 3: Confirm shipment
  - name: confirm-shipment
    path: /v1/orders/{{order_id}}/shipment/confirm
    method: PATCH
    weight: 3
    body: |
      {
        "tracking": "{{tracking_number|pending}}",
        "notification_url": "{{invoice_url}}"
      }

  # Step 4: Get final status
  - name: get-final-status
    path: /v1/orders/{{order_id}}/status
    method: GET
    weight: 1

feeder:
  path: ./customers.csv
  type: csv

concurrency: 20
rate: 100
duration: 5m
```

Running this configuration will:
1. Extract `order_id` and `invoice_url` from the create response
2. Use `{{order_id}}` in subsequent requests to the correct endpoint
3. Extract `tracking_number` from the order details
4. Pass tracking information to the confirm endpoint with a fallback to "pending"
5. Use the same `order_id` across all four requests
6. Each worker maintains its own `order_id`, so concurrent workers don't interfere

## Debugging Extraction Issues

If extraction isn't working:

1. **Check for "JSONPath not found" warnings** in stderr
2. **Verify JSON is valid**: Use `jq` to test the path
3. **Check variable spelling**: `{{user_id}}` ≠ `{{userId}}`
4. **Test regex separately**: Use an online regex tester
5. **Enable detailed logging**: Warnings are printed for failed extractions
6. **Verify endpoint is returning expected data**: Manually call the API to confirm response format

Example testing:

```bash
# Test JSON path extraction
curl https://api.example.com/users | jq .id

# Test regex extraction
curl https://api.example.com/auth | grep -oP '"token":"[^"]+"'
```

## See Also

- [Data Feeders](feeders.md) – Combine feeders with extractors for powerful data-driven testing
- [Endpoints](configuration.md#endpoints-and-weights) – Configure multiple endpoints with extraction
- [Authentication](authentication.md) – Extract auth tokens from login endpoints
