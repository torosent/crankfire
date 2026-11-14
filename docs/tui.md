---
layout: default
title: Terminal UI
---
{% raw %}

# Terminal UI

`crankfire tui` is a full-screen terminal app for keeping a library of load tests ("sessions"), running them, and browsing their history. It also manages [sets](sets.md): suites of sessions organized into stages and evaluated with thresholds.

```bash
crankfire tui                      # uses $CRANKFIRE_DATA_DIR or ~/.crankfire
crankfire tui --data-dir ./perf    # use a project-local library
```

Everything the TUI creates is plain YAML/JSON on disk (see [Data directory](#data-directory)), so you can version it, edit it by hand, or script it.

## Concepts

| Term | What it is |
|---|---|
| **Session** | A saved load-test configuration (the same settings as a [config file](configuration.md)) with a name, description and tags |
| **Run** | One execution of a session. Its summary is stored, plus `result.json` and `report.html` |
| **Set** | Stages of session items with overrides and thresholds. See [Sets](sets.md) |

## Sessions list (home screen)

| Key | Action |
|---|---|
| `↑`/`k`, `↓`/`j` | Move the cursor |
| `Enter` | Open session details |
| `n` | New session |
| `e` | Edit the selected session |
| `i` | Import a session from an existing config file |
| `r` | Run the selected session |
| `h` | Show run history |
| `d` | Delete (asks to confirm with `y`/`n`) |
| `s` | Switch to the sets list |
| `/` | Filter by tag. `Enter` applies, `Esc` cancels |
| `x` | Clear the active filter |
| `q`, `Ctrl+C` | Quit |

**Tag filter syntax:** spaces mean AND, commas mean OR. `prod smoke,regression` matches sessions tagged `prod` **and** either `smoke` or `regression`. Tags may contain letters, digits, `.`, `_` and `-`, up to 64 characters.

> **Note:** On the home screen `q` quits immediately, even while you are typing in the filter prompt.

Deleting a session removes only its YAML file. Its run history stays under `runs/<session-id>/`, and sets that reference it will fail that item until you update them.

## Creating and editing a session

The editor has two modes.

**Form mode** covers the common fields: Name, Target, Protocol (`http`, `websocket`, `sse`, `grpc`), Total, Rate, Duration, Concurrency and Timeout.

| Key | Action |
|---|---|
| `Tab`/`↓` | Next field (Tab on the last field switches to YAML mode) |
| `Shift+Tab`/`↑` | Previous field |
| `F2` | Toggle YAML mode |
| `Ctrl+S` | Save |
| `Esc` | Cancel |

The form requires a name and a URL with scheme and host, such as `https://...` or `ws://...`. For targets without a scheme, like a gRPC `host:port`, use YAML mode or import a config file.

**YAML mode** edits the whole session, including everything the form cannot express: headers, body, endpoints, load patterns, auth, feeders, protocol blocks, thresholds and tracing. `F2` returns to the form; invalid YAML keeps you in YAML mode with a warning. Saving from YAML mode keeps the session ID. Do not edit `id:` and then switch back to the form with `F2`: saving from the form would then create a new session file and leave the old one.

> **Important:** Inside a session file, the `config:` block uses lowercased Go field names, **not** the config-file keys. For example `targeturl` instead of `target`, and `loadpatterns`, `bodyfile`, `fromrps`, `clientsecret`, `protofile`, `samplerate`. Durations are strings such as `30s`. Extractors are the exception: they keep their normal keys (`jsonpath`, `regex`, `var`, `on_error`).

```yaml
schema_version: 1
id: 01JC9V6M3T6Q3WJ8T6K6Y6R0ZB
name: checkout-api
description: Baseline for the checkout service
tags: [prod, smoke]
config:
  targeturl: https://api.example.com/checkout
  method: POST
  headers:
    Content-Type: application/json
  body: '{"sku":"123"}'
  concurrency: 20
  rate: 100
  duration: 1m
  timeout: 5s
  protocol: http
  loadpatterns:
    - name: warmup
      type: ramp
      fromrps: 10
      torps: 100
      duration: 30s
  thresholds:
    - "http_req_duration:p95 < 500"
```

The easiest way to get a correct session is to **import** a normal config file (`i`), then fine-tune it in YAML mode.

## Importing a config file

Press `i`, enter the path to a JSON/YAML [config file](configuration.md) and a session name, then press `Enter`. The file is parsed with the same loader as `crankfire --config`.

> **Security:** Importing copies every value into the session file, including secrets that were resolved from `CRANKFIRE_AUTH_*` environment variables. Session files are written with `0600` permissions inside a `0700` directory, but they are plaintext.

## Session details

Shows the session's name, ID, description, target, protocol and total. `r` runs it, `h` opens its history, `Esc` goes back. The footer also shows `[e] edit`, but that key does nothing here; edit from the sessions list.

## Running a session

Press `r` on the list or the details screen. The test runs inside the TUI process and shows a reduced version of the [`--dashboard`](dashboard-reporting.md#live-dashboard) view: run summary, RPS, request context and the latency trend with P50/P95/P99. The endpoint, status-bucket, protocol-metric and load-pattern panels stay empty. Use `report.html` or `result.json` for those details after the run.

| Key | Action |
|---|---|
| `q`, `Esc`, `Ctrl+C` | Cancel the run |

When the run finishes or is cancelled, Crankfire writes `result.json` and `report.html` into the run directory, records the run summary, and returns to the previous screen. A run with any failed requests is recorded with status `failed`.

> **Note:** Thresholds in a session are not evaluated for TUI single runs. Use `crankfire --config` or a [set](sets.md) when you need a pass/fail verdict.

## Run history

A table of past runs with Started, Status, Total, P95 (ms) and Errors.

| Key | Action |
|---|---|
| `↑`/`k`, `↓`/`j` | Move |
| `o` | Open the run's `report.html` in your browser (`open`, or `xdg-open` on Linux) |
| `Esc` | Back |

## Sets screens

Press `s` on the home screen to open the sets list. The full set workflow, including the YAML format, overrides, thresholds, compare and diff, is described in [Sets](sets.md). The keys are:

| Screen | Keys |
|---|---|
| Sets list | `j`/`k` move · `Enter` details · `n` new · `t` new from template · `d` delete (**no confirmation**) · `r` refresh · `/` filter · `q`/`Esc` quit the **whole TUI** (there is no way back to the sessions list) |
| Set details | `e` edit · `R` run · `h` history · `Esc` back |
| Set editor | `Tab` switch Name/Description · `y` YAML mode (stages and items) · `f` back to form · `Ctrl+S` save · `Esc` cancel. `y` and `f` work everywhere on this screen, so you cannot type a `y` in the form or an `f` in the YAML. Edit `sets/<id>.yaml` in a text editor instead when you need to |
| Template picker | `j`/`k` + `Enter` choose · then edit `key=value` params: `Tab`/`Shift+Tab` move, `Ctrl+N` add row, `Ctrl+S` create · `Esc` back |
| Set run | Live per-stage, per-item status · `q`/`Esc` cancel, or open the compare view when finished |
| Set history | `space` mark a run · `Enter` compare items in a run · `d` diff the two marked runs · `Esc` back |
| Compare | Metric × item table; `★` marks the best item per metric |
| Diff | Verdict (`improved`, `regressed`, `mixed`, `unchanged`) and per-item deltas |

The tag filter on the sets list matches a set when any session it references matches the filter.

> **Known issue:** Screens reached with `R`, `h` (from set details) and `t` (from the sets list) currently open without loading. The set run does not start, history is empty, and no templates are listed. Coming back to the sets list also shows it empty until you press `r`. Until this is fixed, use `crankfire set run`, `crankfire set diff` and `crankfire set new --from-template` from the [CLI](sets.md#cli).

## Data directory

The data directory is resolved in this order: `--data-dir`, then `$CRANKFIRE_DATA_DIR`, then `~/.crankfire`.

```
~/.crankfire/
├── sessions/<id>.yaml                 # one file per session (IDs are ULIDs)
├── runs/<session-id>/<timestamp>/     # one directory per TUI run
│   ├── run.json                       # status + summary (requests, errors, p50/p95/p99)
│   ├── result.json                    # full JSON report
│   └── report.html                    # HTML report
├── sets/<id>.yaml                     # set definitions
├── runs/sets/<set-id>/<timestamp>/set-run.json
├── templates/<id>.yaml                # set templates (written by hand)
└── daemon.lock                        # held while `crankfire daemon` runs
```

Writes are atomic (write to a temp file, then rename) and serialized with lock files, so the TUI, `crankfire session`/`set` and the daemon can share one directory.

## Related commands

- `crankfire session list|edit` manages session metadata and tags from scripts. See [Sets → Session CLI](sets.md#session-cli).
- `crankfire set ...` and `crankfire daemon` run and schedule sets. See [Sets](sets.md).
{% endraw %}
