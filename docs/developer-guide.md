---
layout: default
title: Developer Guide
---
{% raw %}

# Developer Guide

This page is for contributors. For how the code fits together, read the [architecture docs](architecture/1-project-overview.md) first.

## Prerequisites

- Go **1.26** or later (see `go.mod`)
- Optional: Node.js, to validate the Mermaid diagrams in the docs
- Optional: Docker, for tracing backends and container builds

## Build and run

```bash
go build -o build/crankfire ./cmd/crankfire
./build/crankfire --target https://example.com --total 10
```

`build/` is git-ignored.

## Tests

```bash
# Unit tests (what CI runs first)
go test -race ./...

# Integration tests: build and drive the real binary, TUI and set flows
go test -v -tags=integration -race -timeout 15m ./cmd/crankfire/
```

CI (`.github/workflows/ci.yml`) runs both commands on every pull request and every push to `main`, and uploads coverage to Codecov.

Tests worth knowing:

| Location | What it covers |
|---|---|
| `internal/cli/features_matrix_*_test.go` | Feature × protocol matrix (auth, feeders, load patterns, thresholds, protocols), run against in-process test servers |
| `internal/cli/integration_test.go`, `*_integration_test.go`, `build_e2e_test.go` | End-to-end wiring through `cli.Run` and `BuildRunner` |
| `internal/cli/samples_test.go` | Checks that the sample configs in `scripts/` still parse |
| `internal/output/setreport/testdata/expected.html` | Golden file for the set HTML report |
| `internal/store/path_traversal_test.go` | ID validation and path-traversal guards |
| `cmd/crankfire/*_test.go` (`integration` tag) | Binary-level tests for the TUI, sets and the daemon |

### Shell scripts

`scripts/` contains manual end-to-end checks. Most of them build the binary into `build/` (`test-thresholds.sh` expects it to exist already). Only the scripts that use `doc_samples_helpers.sh` run against the local servers in `scripts/testservers/`. `test-thresholds.sh`, `test-dashboard.sh`, `test-html-report.sh` and `test-integration.sh` send traffic to `https://httpbin.org`, and `test-protocols.sh` also uses a public WebSocket echo service, so they need internet access:

| Script | Purpose |
|---|---|
| `test-integration.sh` | All features together |
| `test-protocols.sh` | HTTP, WebSocket, SSE, gRPC |
| `test-chaining.sh`, `test-thresholds.sh`, `test-dashboard.sh`, `test-html-report.sh` | Feature-specific checks |
| `verify-doc-samples.sh` | Meant to check a fixed list of configs in `scripts/doc-samples/`. It runs `crankfire --config X --help`, and `--help` exits before the file is read, so it does not catch broken configs today |
| `validate-mermaid.js` | Renders every Mermaid block under `docs/` with mermaid-cli |

## Repository layout

```
cmd/crankfire/          main.go: subcommand dispatch (tui, daemon, session, set, default run)
internal/
  cli/                  Orchestration: Run, BuildRunner, per-protocol requesters,
                        endpoint selection, session/set/daemon subcommands
  cli/livedash/         Bubble Tea driver for --dashboard
  config/               Config struct, flag definitions, file loader, validation
  runner/               Scheduler + worker pool, arrival models, load patterns, retry middleware
  metrics/              Sharded HDR-histogram collector, stats, time-series snapshots
  clientmetrics/        Counters shared by the WebSocket/SSE/gRPC clients
  output/               Text, JSON, progress and HTML reports
  output/setreport/     HTML report for set runs
  httpclient/           Tuned http.Client and RequestBuilder (placeholders, feeders, auth)
  websocket/ sse/ grpcclient/   Protocol clients
  pool/                 Connection pool for WebSocket/SSE clients
  auth/                 OAuth2 and static-token providers
  feeder/               CSV/JSON data feeders
  placeholders/         {{key}} / {{key|default}} substitution
  variables/            Per-worker variable store for request chaining
  extractor/            JSONPath (gjson) and regex extractors
  har/                  HAR parsing and conversion to endpoints
  threshold/            Single-run threshold parser and evaluator
  tracing/              OpenTelemetry provider, spans, propagation
  store/                Data directory: sessions, runs, sets, templates
  setrunner/            Set execution, overrides, set thresholds, compare and diff
  scheduler/            Cron manager used by the daemon
  tagfilter/            Tag filter expressions (AND/OR)
  template/             text/template rendering for set templates
  tui/                  Terminal UI: root model, screens/, runview/, widgets/
scripts/                Sample configs, test servers, verification scripts
docs/                   This site (GitHub Pages, Jekyll)
```

## Common changes

### Adding a CLI flag or config key

1. Add the field to `config.Config` (or a nested struct) in `internal/config/config.go`.
2. Register the flag in `internal/config/flags.go`. In `applyFlagOverrides`, copy it **only when `fs.Changed(...)`**, following the existing pattern.
3. Parse the config-file key in `internal/config/loader.go` with `lookupSetting`, accepting the snake, kebab and concatenated spellings.
4. Validate it in `Config.Validate`.
5. If the flag's help text shows a default, also set that default in `Load` **and** `LoadFromFile` (`loader.go`; the TUI import uses `LoadFromFile`). Otherwise the documented default never applies to runs that don't pass the flag.

   Note that `Config.Validate` runs only for `crankfire [flags]`. TUI, set and daemon runs go straight to `cli.BuildRunner`, so `BuildRunner` must handle bad values safely.
6. Document it in [configuration.md](configuration.md).

### Adding a protocol

1. Write the client under `internal/<protocol>/`, recording into `clientmetrics`.
2. Add a requester in `internal/cli/<protocol>_requester.go` that implements `runner.Requester`. Embed `baseRequesterHelper` for latency, status buckets, auth headers and tracing.
3. Wire it into `buildRequester` (`internal/cli/build.go`). If the requester holds connections, implement `io.Closer`.
4. Add the protocol to `config.Protocol` validation, and to the TUI edit form's protocol check.
5. Add cases to the `features_matrix_*` tests.

### Adding a TUI screen

Screens are Bubble Tea models in `internal/tui/screens`. Navigate with `PushMsg`/`PopMsg`, which are handled by `internal/tui/root.go`. Root calls `Init()` only when it pushes or pops a screen, so put any loading work in `Init()` and reach the screen through a push.

## Docs site

The `docs/` folder is published with GitHub Pages using the Cayman theme (`docs/_config.yml`).

- Every page needs front matter (`layout: default`, `title`).
- Jekyll runs Liquid on every page, code blocks included, so `{{ ... }}` and `{% ... %}` would be removed from the published page. Each page body is therefore wrapped in Liquid `raw` and `endraw` tags, placed right after the front matter and at the end of the file. Keep that wrapper when you add a page, and never write those two tags inside the page text.
- Link between pages with relative `.md` paths. Pages converts them to `.html`.
- Mermaid diagrams go in fenced code blocks marked `mermaid`. `docs/_includes/head-custom.html` loads Mermaid and renders them in the browser.
- Validate diagrams with `node scripts/validate-mermaid.js`, which downloads mermaid-cli through `npx`.

## Releasing

Pushing a `v*` tag (or a manual `workflow_dispatch` run for an existing tag) runs `.github/workflows/release.yml`. It runs the unit tests, then:

- builds `crankfire_<os>_<arch>` archives for linux and darwin (amd64, arm64) as tar.gz, and windows/amd64 as zip,
- publishes a GitHub release,
- pushes a multi-arch container image to `ghcr.io/torosent/crankfire`, built from the repository `Dockerfile`,
- updates the Homebrew tap formula.
{% endraw %}
