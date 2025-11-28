---
layout: default
title: HAR Import
---

# HAR Import

Crankfire can import HTTP Archive (HAR) files and convert them to load test endpoints. This allows you to record browser sessions and replay them as load tests.

## What is HAR?

HAR (HTTP Archive) is a JSON-based format for recording HTTP transactions. Most browsers and developer tools can export HAR files:

- **Chrome**: Network tab → Right-click → "Save all as HAR with content"
- **Firefox**: Network tab → Gear icon → "Save All As HAR"
- **Safari**: Network tab → Export
- **Charles/Fiddler**: Export as HAR

## Quick Start

### Basic Usage

```bash
crankfire --har recording.har --total 100
```

This loads all HTTP requests from the HAR file and executes them as a load test.

### With Filtering

Filter by host to include only specific domains:

```bash
crankfire --har recording.har --har-filter "host:api.example.com" --total 100
```

Filter by HTTP method:

```bash
crankfire --har recording.har --har-filter "method:GET,POST" --total 100
```

Combine filters with semicolons:

```bash
crankfire --har recording.har --har-filter "host:api.example.com;method:POST" --total 100
```

## CLI Flags

| Flag | Description | Example |
|------|-------------|---------|
| `--har` | Path to HAR file | `--har recording.har` |
| `--har-filter` | Filter HAR entries | `--har-filter "host:api.example.com"` |

## Config File Usage

You can also specify HAR import in a config file:

```yaml
har_file: ./recording.har
har_filter: "host:api.example.com;method:GET,POST"
concurrency: 10
duration: 1m
```

Then run:

```bash
crankfire --config loadtest.yaml
```

## Filter Format

The `--har-filter` flag accepts a semicolon-separated list of filters:

### Host Filter

Include only requests to specific hosts:

```
host:api.example.com
host:api.example.com,cdn.example.com
```

### Method Filter

Include only specific HTTP methods:

```
method:GET
method:GET,POST,PUT
```

### Combined Filters

Combine multiple filters with semicolons:

```
host:api.example.com;method:POST
```

## Automatic Filtering

Crankfire automatically excludes static assets by default:

- JavaScript files (`.js`, `.map`)
- CSS files (`.css`)
- Images (`.png`, `.jpg`, `.jpeg`, `.gif`, `.svg`, `.ico`)
- Fonts (`.woff`, `.woff2`, `.ttf`, `.eot`)

This keeps your load test focused on API endpoints rather than static content.

## What Gets Imported

From each HAR entry, Crankfire extracts:

| HAR Field | Endpoint Field | Notes |
|-----------|----------------|-------|
| `request.method` | `method` | HTTP method (GET, POST, etc.) |
| `request.url` | `url` | Full URL including query string |
| `request.headers` | `headers` | Request headers (hop-by-hop headers filtered) |
| `request.postData.text` | `body` | Request body for POST/PUT/PATCH |
| URL path | `name` | Endpoint name for metrics |

### Filtered Headers

The following hop-by-hop headers are automatically removed:

- Connection
- Keep-Alive
- Proxy-Authenticate
- Proxy-Authorization
- TE
- Trailers
- Transfer-Encoding
- Upgrade

## Merging with Config Endpoints

HAR endpoints can be combined with endpoints defined in your config file:

```yaml
target: https://api.example.com
har_file: ./recording.har

# These endpoints are added to HAR endpoints
endpoints:
  - name: "health-check"
    path: /health
    weight: 1
```

The HAR endpoints are appended to any existing endpoints, giving you flexibility to mix recorded and manually-defined requests.

## Security Warning

⚠️ **HAR files may contain sensitive data:**

- Authentication tokens and cookies
- API keys
- Personal data from form submissions

When you load a HAR file, Crankfire displays a warning:

```
WARNING: HAR file may contain sensitive data (cookies, auth tokens). 
Review endpoints before use in production.
```

**Best Practices:**

1. Review HAR files before using them in load tests
2. Remove or redact sensitive headers (Authorization, Cookie)
3. Don't commit HAR files with real credentials to version control
4. Consider using [Data Feeders](feeders.md) for parameterized auth tokens

## Workflow Example

### Step 1: Record Browser Session

1. Open browser DevTools (F12)
2. Go to Network tab
3. Clear existing entries
4. Perform the user workflow you want to test
5. Right-click → "Save all as HAR"

### Step 2: Review and Filter

Check what's in your HAR file:

```bash
# Count entries
cat recording.har | jq '.log.entries | length'

# List URLs
cat recording.har | jq '.log.entries[].request.url'
```

### Step 3: Run Load Test

```bash
crankfire \
  --har recording.har \
  --har-filter "host:api.example.com" \
  --concurrency 10 \
  --duration 1m \
  --dashboard
```

### Step 4: Generate Report

```bash
crankfire \
  --har recording.har \
  --har-filter "host:api.example.com" \
  --concurrency 10 \
  --total 1000 \
  --html-output har-test-report.html
```

## Limitations

- **Single protocol**: HAR import only works with HTTP protocol (not WebSocket, SSE, or gRPC)
- **Static replay**: HAR captures specific values; use [Data Feeders](feeders.md) for dynamic parameterization
- **No response validation**: Crankfire doesn't compare responses to HAR-recorded responses
- **Cookie state**: Cookies are included as headers but aren't managed across requests

## Next Steps

- Add dynamic data with [Data Feeders](feeders.md)
- Set performance criteria with [Thresholds](thresholds.md)
- View live metrics with [Dashboard](dashboard-reporting.md)
