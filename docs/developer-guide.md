---
layout: default
title: Developer Guide
---

# Developer Guide

This guide is for people who want to contribute to Crankfire or integrate it with other tooling.

## Repository Layout

- `cmd/crankfire` – CLI entrypoint, flag parsing, top‑level orchestration.
- `internal/runner` – core scheduling, arrival models, retry logic.
- `internal/httpclient`, `internal/grpcclient`, `internal/sse`, `internal/websocket` – protocol implementations.
- `internal/metrics` – collectors, histograms, JSON output.
- `internal/output` – progress and report formatting, dashboard.
- `internal/auth` – OAuth2/OIDC helpers.
- `internal/feeder` – CSV/JSON data feeders and templating.

## Local Development

### Requirements

- Go (see `go.mod` for the minimum supported version).

### Run Tests

From the repo root:

```bash
go test ./...
```

### Build

```bash
go build -o build/crankfire ./cmd/crankfire
```

## Adding a Feature

1. Decide whether it belongs in the CLI (`cmd/crankfire`) or an internal package.
2. Add tests in the corresponding `*_test.go` files.
3. Keep public behavior discoverable via docs (`README.md` and `docs/`).

Follow existing patterns for configuration structs and JSON/YAML tags to keep the config surface consistent.

## Extending Protocol Support

When adding protocol‑specific options:

- Keep core scheduling and metrics in shared packages.
- Add protocol‑specific fields to config while preserving backwards compatibility.
- Ensure JSON/reporting includes protocol metrics in a stable format.

## Coding Style

- Prefer small, focused functions.
- Use clear names (avoid single‑letter variables except in tight loops).
- Keep error messages actionable.

## Filing Issues & PRs

- Include a minimal reproduction or configuration snippet.
- Attach relevant JSON output or logs when reporting metrics or behavior issues.
- For performance work, describe the before/after behavior and measurement method.
