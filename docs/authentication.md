---
layout: default
title: Authentication
---
{% raw %}

# Authentication

Crankfire can obtain a Bearer token and attach it to every request. Auth is configured only through an `auth:` block in a config file; there are no auth CLI flags.

## Supported types

| `type` | What it does | Required fields |
|--------|--------------|-----------------|
| `oauth2_client_credentials` | OAuth2 client-credentials grant | `token_url`, `client_id`, `client_secret` |
| `oauth2_resource_owner` | OAuth2 password grant (legacy, prints a warning) | the fields above plus `username` and `password` |
| `oidc_implicit`, `oidc_auth_code` | Sends a token you already have, unchanged | `static_token` |

Optional fields:

- `scopes`: a list, joined with spaces into the `scope` parameter.
- `refresh_before_expiry`: a duration, default `30s`.

### Client credentials

```yaml
target: https://api.example.com
auth:
  type: oauth2_client_credentials
  token_url: https://idp.example.com/oauth/token
  client_id: load-tester
  # client_secret is read from CRANKFIRE_AUTH_CLIENT_SECRET
  scopes:
    - api.read
    - api.write
  refresh_before_expiry: 1m
```

```bash
export CRANKFIRE_AUTH_CLIENT_SECRET='…'
crankfire --config auth.yml --concurrency 20 --duration 1m
```

### Resource owner (password)

```yaml
auth:
  type: oauth2_resource_owner
  token_url: https://idp.example.com/oauth/token
  client_id: load-tester
  username: perf-user@example.com
  # client_secret and password come from CRANKFIRE_AUTH_CLIENT_SECRET / CRANKFIRE_AUTH_PASSWORD
```

This grant is deprecated in OAuth 2.1, so Crankfire prints a warning when you use it. Prefer client credentials.

### Pre-issued token

```yaml
auth:
  type: oidc_auth_code
  # static_token is read from CRANKFIRE_AUTH_STATIC_TOKEN
```

Crankfire sends the token as `Authorization: Bearer <token>`. It never refreshes it, so the token must stay valid for the whole run.

## How tokens are fetched

For the two OAuth2 types:

- **The first fetch is lazy.** No token is requested at startup. The first request that needs one triggers the fetch, so that request's latency includes the token call.
- **The token request** is a form-encoded `POST` to `token_url` with `grant_type` (and `username`/`password` for the password grant). `client_id` and `client_secret` are sent with **HTTP Basic auth**, not in the form. The call has its own 30s timeout. The response must be `200` with a JSON `access_token`.
- **Caching.** The token is cached for `expires_in` (one hour if the server omits it), minus whichever is smaller: `refresh_before_expiry` or half the token's lifetime.
- **Concurrency.** When several workers need a token at once, they share one in-flight request.
- **Refresh.** There is no background refresh and no `refresh_token` flow. A new token is fetched on demand once the cached one expires. A failed fetch is not cached, so the next request tries again straight away.

## Where the token goes

| Protocol | Placement |
|----------|-----------|
| HTTP | `Authorization: Bearer <token>`. This **replaces** any `Authorization` header you set yourself. |
| WebSocket | `Authorization: Bearer <token>` on the handshake. Replaces any `Authorization` header you set. |
| SSE | `Authorization: Bearer <token>` on the stream request. Replaces any `Authorization` header you set. |
| gRPC | `authorization: Bearer <token>` metadata. Replaces any `authorization` metadata you set. |

## Secrets and environment variables

Crankfire reads three environment variables:

| Variable | Fills |
|----------|-------|
| `CRANKFIRE_AUTH_CLIENT_SECRET` | `auth.client_secret` |
| `CRANKFIRE_AUTH_PASSWORD` | `auth.password` |
| `CRANKFIRE_AUTH_STATIC_TOKEN` | `auth.static_token` |

Each variable is used only when the config file has an `auth:` block **and** the matching field is empty or missing. A value in the file always wins.

> **Note:** Config files are not environment-expanded. `client_secret: ${SECRET}` sends the literal string `${SECRET}`. Leave the field out and export the matching variable instead.

## Security notes

- Only test systems you own or are authorised to test.
- Keep secrets out of committed config files. Use the environment variables above.
- When you import a config in the [Terminal UI](tui.md), Crankfire stores the resolved secrets, including any taken from environment variables, in **plaintext** in `<data dir>/sessions/<id>.yaml` (`~/.crankfire` by default, or `$CRANKFIRE_DATA_DIR`). The directory is created with mode `0700` and the files with mode `0600`.
- Use short-lived, minimally scoped tokens made for load testing.
- [HAR imports](har-import.md) keep `Authorization` and `Cookie` headers. Check them before replaying.
{% endraw %}
