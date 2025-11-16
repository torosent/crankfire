# Authentication

Crankfire includes first-class support for OAuth2 and OIDC flows so you can load test protected APIs safely. Tokens are automatically injected into HTTP headers, reused by WebSocket/SSE runs (they share the same headers), and propagated as `authorization` metadata for gRPC calls.

Auth is configured via config files only; flags are intentionally not provided to avoid leaking secrets via shell history or process listings.

## Supported Flows

- **OAuth2 Client Credentials** – ideal for service-to-service calls.
- **OAuth2 Resource Owner Password** – legacy user/password flow.
- **OIDC Implicit / Auth Code** – use a static token you obtained elsewhere.

## Client Credentials

```yaml
target: https://api.example.com

auth:
  type: oauth2_client_credentials
  token_url: https://idp.example.com/oauth/token
  client_id: your-client-id
  client_secret: your-client-secret
  scopes:
    - read
    - write
  refresh_before_expiry: 30s

concurrency: 20
duration: 5m
```

Crankfire will:

1. Fetch an access token before starting the test.
2. Inject it as an `Authorization: Bearer` header on each request.
3. Refresh it in the background before it expires.

## Resource Owner Password

```json
{
  "target": "https://api.example.com",
  "auth": {
    "type": "oauth2_resource_owner",
    "token_url": "https://idp.example.com/oauth/token",
    "client_id": "client-id",
    "client_secret": "client-secret",
    "username": "user@example.com",
    "password": "userpass",
    "scopes": ["api"]
  },
  "concurrency": 10,
  "total": 1000
}
```

Prefer client credentials whenever possible; password flows are best kept to legacy systems.

## Static Tokens (OIDC)

For implicit/auth-code flows or pre-issued tokens:

```yaml
target: https://api.example.com

auth:
  type: oidc_implicit
  static_token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...

concurrency: 15
duration: 2m
```

## Secrets & Environment Variables

Avoid embedding secrets directly in config files. Use environment variables and substitute them when generating configs, or rely on Crankfire’s dedicated env vars:

- `CRANKFIRE_AUTH_CLIENT_SECRET`
- `CRANKFIRE_AUTH_PASSWORD`
- `CRANKFIRE_AUTH_STATIC_TOKEN`

These values are read at runtime and never persisted.

## Manual Headers

For non-OAuth APIs, stick to manual headers:

```bash
crankfire --target https://api.example.com \
  --header "Authorization=Bearer token123" \
  --header "X-API-Key=your-api-key" \
  --total 100
```

## Security Guidance

- Use least-privilege scopes.
- Prefer short-lived tokens.
- Never commit raw secrets to version control.
- Turn off `--grpc-insecure` in production.
