---
layout: default
title: Request Chaining
---
{% raw %}

# Request Chaining

Request chaining takes a value from one response and uses it in later requests, for example a token from a login call, the ID of a newly created resource, or a session ID. It works only in **HTTP** mode with [weighted endpoints](configuration.md#weighted-endpoints).

```yaml
target: https://api.example.com
concurrency: 10
duration: 1m
endpoints:
  - name: login
    weight: 1
    method: POST
    path: /auth/login
    headers: { Content-Type: application/json }
    body: '{"email":"perf@example.com","password":"secret"}'
    extractors:
      - { jsonpath: $.token, var: auth_token }
      - { jsonpath: $.user.id, var: user_id }

  - name: profile
    weight: 5
    path: /users/{{user_id|me}}
    headers:
      Authorization: "Bearer {{auth_token|}}"
```

## Placeholders

- `{{name}}`: the value of `name`. If nothing matches, it becomes an empty string.
- `{{name|fallback}}`: the value of `name`, or `fallback` if no value is found.
- `{{name|}}`: the value of `name`, or an explicitly empty string.

When a request is built, each placeholder is looked up in this order:

1. **Variables extracted earlier by this worker**
2. **Fields of the current feeder record** (see [Data Feeders](feeders.md))
3. **The default** after `|`

Values are inserted as-is, with no URL or JSON escaping.

Placeholders work in URLs, endpoint `path`s, header values and bodies. Names are matched exactly; spaces inside the braces are not trimmed. Go-template syntax such as `{{.field}}` is **not** supported in requests.

## Extractors

You attach extractors to an endpoint. Each extractor needs exactly one of `jsonpath` or `regex`, plus a `var` name. The name must be a valid identifier: letters, digits and underscores, and it cannot start with a digit.

| Key | Description |
|-----|-------------|
| `jsonpath` | A path in [gjson](https://github.com/tidwall/gjson/blob/master/SYNTAX.md) syntax. A leading `$.` is stripped, and `$` on its own means the whole document. |
| `regex` | Go regular expression. The first capture group is extracted, or the whole match when the pattern has no groups. |
| `var` | Variable to store the value in. |
| `on_error` | Also run this extractor when the response status is `>= 400`. Default `false`. |

```yaml
extractors:
  - jsonpath: id                 # top-level field
    var: order_id
  - jsonpath: $.items.0.sku      # array index
    var: first_sku
  - jsonpath: $                  # whole body
    var: raw
  - regex: 'session=([a-f0-9]+)'
    var: session_id
  - jsonpath: error.code
    var: last_error
    on_error: true
```

When extractors run:

- Extractors run after the response body has been read. Only the first **1 MB** of the body is examined.
- On responses with status **below 400**, every extractor runs. On status `>= 400`, only those with `on_error: true` run.
- Objects and arrays are stored as raw JSON text. Numbers are stored in their shortest form (`1500.0` becomes `1500`), and `null` becomes an empty string.

> **Note:** If an extractor finds nothing (path missing or regex no match), Crankfire prints a warning to stderr and stores an **empty string**. That empty value counts as "set", so later lookups return `""` and **ignore** any `|default`. Defaults apply only while a variable has never been set in that worker.

## Scope: one store per worker

Each worker goroutine (`concurrency` of them) has its own variable store, which lasts for the whole run.

- A value one worker extracts is visible to that worker's later requests only.
- Workers never share variables. Ten workers means ten independent sessions.
- A later extraction to the same `var` overwrites the earlier value.

## Ordering: endpoints are picked at random

Endpoints are chosen **at random by weight for each request**. There is no "step 1, then step 2". A worker's first request might be `profile` before it has ever run `login`. Design for that:

- **Give dependent requests safe defaults**, such as `{{user_id|me}}` or `{{auth_token|}}`, so they still produce a valid, if unauthenticated, request.
- **Keep producer endpoints at a low weight.** Once a worker has a value it keeps it, so occasional re-logins are enough.
- **Expect some early failures.** The first few requests per worker may fail until a value exists. Allow for that in your [thresholds](thresholds.md).
- **For an auth token, prefer the `auth:` block** ([Authentication](authentication.md)). It fetches the token once on first use and shares it across all workers, so you don't need chaining at all.

## Limits

- HTTP only. WebSocket, SSE and gRPC ignore extractors and variables.
- Values are strings; there is no type conversion or arithmetic.
- Regexes are compiled on every response, so complex patterns cost CPU at high request rates.
{% endraw %}
