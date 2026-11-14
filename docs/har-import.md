---
layout: default
title: HAR Import
---
{% raw %}

# HAR Import

Record a browser session, save it as an HTTP Archive (HAR), and replay the captured requests as a weighted HTTP load test.

## Recording a HAR

- **Chrome / Edge**: DevTools, then the Network tab. Right-click a request and choose **Save all as HAR with content**.
- **Firefox**: DevTools, then the Network tab. Click the gear icon and choose **Save All As HAR**.
- **Safari**: Web Inspector, then the Network tab, then **Export**.
- Proxies such as Charles or Fiddler can also export HAR files.

## Usage

```bash
crankfire --har recording.har --total 100
crankfire --har recording.har --har-filter "host:api.example.com" --concurrency 10 --duration 1m
crankfire --har recording.har --har-filter "host:api.example.com;method:GET,POST" --rate 50 --duration 2m
```

You can also set it in a config file:

```yaml
har_file: ./recording.har            # relative to the working directory
har_filter: "host:api.example.com;method:GET,POST"
concurrency: 10
duration: 1m
thresholds:
  - "http_req_duration:p95 < 800"
```

You don't need a `target`, because every imported endpoint carries its full URL. Before the run starts, Crankfire prints a summary to stderr, for example `HAR: replaying 12 endpoints to 2 host(s): …`, along with a warning that HAR files may contain sensitive data.

## Filter syntax

A filter is made of clauses separated by `;`. Each clause is `key:value1,value2`. Within a clause, any value can match (OR). For an entry to be kept, it must pass every clause (AND).

| Key | Matches | Example |
|-----|---------|---------|
| `host` | Exact `host[:port]` from the URL. No wildcards, and the port must match if one is recorded. | `host:api.example.com,auth.example.com` |
| `method` | HTTP method, case-insensitive | `method:GET,POST` |

Any other key fails with an error: `unknown HAR filter key "x" (supported: host, method)`.

## What gets imported

Each remaining HAR entry becomes one endpoint:

| HAR field | Endpoint | Notes |
|-----------|----------|-------|
| `request.url` | `url` | Full URL, including the query string |
| URL path | `name` (and `path`) | Several entries with the same path share one metrics row |
| `request.method` | `method` | |
| `request.headers` | `headers` | After the header filtering described below |
| `request.postData.text` | `body` | For any method that has post data. `params` are ignored. |
| — | `weight: 1` | Every entry is equally likely |

Responses are ignored. Entries are replayed by random weighted selection, not in the recorded order. To control the mix, write the requests out by hand as a regular `endpoints:` list with weights. Crankfire has no command that exports a HAR as YAML.

Global `headers`, `auth` and feeders still apply. Endpoint headers from the HAR override global headers that have the same name.

## What gets dropped

**Static assets.** Entries whose URL path ends in any of these extensions are dropped (case-insensitive):
`.js .css .png .jpg .jpeg .gif .svg .woff .woff2 .ttf .eot .ico .map`

**Headers.** These headers are removed:
- hop-by-hop headers: `Connection`, `Keep-Alive`, `Proxy-Authenticate`, `Proxy-Authorization`, `TE`, `Trailers`, `Transfer-Encoding`, `Upgrade`
- `Host` and `Content-Length`, which Go sets itself
- HTTP/2 pseudo-headers such as `:authority` and `:path`

Static-asset filtering is always on. It can't be turned off, and hosts can't be excluded from the CLI. To include or exclude more, edit the HAR or filter with `host:`.

## Security

> **Note:** A HAR records exactly what your browser sent. That includes `Cookie` and `Authorization` headers, session tokens in URLs, and form bodies. Crankfire **keeps** all of them.

- Before replaying a HAR, scrub it or record it with a throwaway test account.
- Never commit HAR files that contain real credentials.
- Use `--har-filter host:…` so your test doesn't hit third-party analytics, CDNs or identity providers you don't own.
- Only replay traffic against systems you're authorised to test.

## Tips

- Start small (`--total 20 --log-errors`) to check that the recorded requests still succeed. Expired cookies and CSRF tokens are common causes of failure.
- To replace a recorded token, configure an [`auth:` block](authentication.md). Its `Authorization` header overrides the recorded one.
- Combine with [thresholds](thresholds.md) and `--html-output` to turn a recorded user journey into a regression check.
{% endraw %}
