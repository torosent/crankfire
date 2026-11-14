---
layout: default
title: Sets, Scheduling & Session CLI
---
{% raw %}

# Sets, Scheduling & Session CLI

A **set** runs several saved [sessions](tui.md#concepts) as one suite. Sessions are grouped into stages, each item can have its own overrides, and the whole run is evaluated against thresholds. Typical uses:

- Run a smoke → load → soak pipeline that stops when an early stage fails.
- Compare variants side by side (protocols, payload sizes, regions) in a single stage.
- Get one CI verdict for several services.
- Run suites on a cron schedule with `crankfire daemon`.

Sets, sessions and their run history live in the [data directory](tui.md#data-directory). Its location is `$CRANKFIRE_DATA_DIR` or `~/.crankfire`.

## Set file format

Sets are stored as `sets/<id>.yaml`. Create them in the TUI (`s`, then `n`), from a [template](#templates), or write the file by hand.

```yaml
schema_version: 1
id: 01JC9W0K4ZK8C3V7S8J4E2N1QX
name: checkout-regression
description: Smoke, then compare two checkout variants
schedule: "0 2 * * *"          # optional; used only by `crankfire daemon`
stages:
  - name: smoke
    on_failure: abort           # default; `continue` runs the next stage anyway
    items:
      - name: smoke
        session_id: 01JC9V6M3T6Q3WJ8T6K6Y6R0ZB
        overrides:
          total_requests: 200
          concurrency: 5
  - name: load
    items:
      - name: checkout-v1
        session_id: 01JC9V6M3T6Q3WJ8T6K6Y6R0ZB
        overrides:
          target_url: https://v1.staging.example.com/checkout
          concurrency: 50
          duration: 2m
      - name: checkout-v2
        session_id: 01JC9V6M3T6Q3WJ8T6K6Y6R0ZB
        overrides:
          target_url: https://v2.staging.example.com/checkout
          concurrency: 50
          duration: 2m
thresholds:
  - { metric: p95,        op: lt, value: 500 }                    # aggregate (default)
  - { metric: error_rate, op: lt, value: 0.01, scope: per_item }
  - { metric: p99,        op: lt, value: 1000, scope: checkout-v2 }
```

These rules are checked when a set is saved from the TUI or created with `set new`. Files you write by hand are **not** checked until they run, so follow the rules yourself. Also make the `id:` field equal the file name: `set list` and the daemon use `id:`, while `set run <id>` uses the file name.


- `name` is required (at most 200 bytes).
- There must be at least one stage, and every stage needs at least one item.
- Item names must be unique within a stage. Give **every** item a `name` that is unique across the **whole** set. Per-item thresholds, compare and diff all match items by name, so unnamed or duplicate items overwrite each other.
- Every `session_id` must refer to an existing session.
- `on_failure` must be empty, `abort` or `continue`.
- `schedule` must be a valid cron expression (see [Scheduling](#scheduling-with-crankfire-daemon)).

## How a set runs

1. **Stages run one after another. Items in the same stage run in parallel**, each as its own in-process load test.
2. Each item loads its session, applies the item's `overrides`, and runs.
3. After each stage:
   - If the run was cancelled, the set stops with status `cancelled`.
   - If an item did not complete and the stage has `on_failure: abort` (the default), the set stops with status `failed`. With `on_failure: continue` the next stage runs anyway.
4. After all stages, thresholds are evaluated. A failed threshold changes a `completed` set to `failed`.
5. The result is written to `runs/sets/<set-id>/<run-id>/set-run.json`. The run ID is a UTC timestamp such as `2026-11-14T09-30-00.000000000Z`.

> **Note:** An item fails only when its session is missing or its load test cannot be built (for example a missing target, or a bad auth, feeder or HAR setting). Request errors do **not** fail an item, and a cancelled item is still recorded as `completed` (the set itself becomes `cancelled`). Use `error_rate` or `total_errors` thresholds to gate on errors.
>
> A session's own `thresholds` are ignored inside a set. Only the set's thresholds apply.

Set item runs are recorded inside `set-run.json`. They do not appear in the session's own run history.

## Overrides

Overrides change a session's settings for one item only. The saved session is not modified.

| Key | Replaces |
|---|---|
| `target_url` | `target` |
| `method` | `method` |
| `body` | `body` |
| `headers` | Merged into the session's headers; override values win |
| `total_requests` | `total` |
| `rate` | `rate` |
| `concurrency` | `concurrency` |
| `duration` | `duration` (for example `2m`) |
| `timeout` | `timeout` |
| `auth_token` | `auth.static_token`; supports `${ENV_VAR}` expansion (unset variables expand to empty) |

> **Note:** `auth_token` takes effect only when the session already uses a static-token auth type (`oidc_implicit` or `oidc_auth_code`). Give the raw token: Crankfire adds the `Bearer ` prefix itself.

## Set thresholds

Set thresholds use a structured format. It is different from the [single-run threshold strings](thresholds.md).

| Field | Values |
|---|---|
| `metric` | `p50`, `p95`, `p99` (ms), `error_rate` (0–1), `rps`, `total_errors` |
| `op` | `lt`, `le`, `gt`, `ge`, `eq` |
| `value` | number |
| `scope` | `aggregate` (default), `per_item`, or an item name |

Scopes:

- **`aggregate`** is evaluated once against the worst case across all items: the **maximum** p50, p95, p99 and error rate, and the **sum** of RPS and total errors.
- **`per_item`** is evaluated separately for every item, including items that failed to start. Those items report all-zero metrics, so a rule such as `p95 lt 500` passes for them.
- **`<item-name>`** is evaluated against that item only. It **fails** if the item's stage never ran, for example because an earlier stage aborted.

A set with no thresholds passes as long as every stage completes.

> **Note:** Unknown `metric` or `op` values are not rejected when you save. They fail at run time with an actual value of 0. `eq` compares floats exactly.

## CLI

```bash
crankfire set list [--tag EXPR]...
crankfire set show <set-id>
crankfire set run <set-id> [flags]
crankfire set new --from-template <template-id> [--param key=value]... [--name NAME]
crankfire set diff <run-id-a> <run-id-b> [--json | --html FILE]
```

### `set list`

Prints `ID NAME STAGES`. `--tag` filters sets by the tags of the sessions they reference: a set matches when any one of its sessions satisfies every `--tag` expression. Repeated `--tag` flags are combined with AND. Inside one expression, spaces mean AND and commas mean OR.

### `set show`

Prints the set as YAML.

### `set run`

```bash
crankfire set run 01JC9W0K4ZK8C3V7S8J4E2N1QX
crankfire set run <id> --json > set-run.json
crankfire set run <id> --html set-report.html
crankfire set run <id> --threshold p95:lt:400 --threshold error_rate:lt:0.02:per_item
crankfire set run <id> --override checkout-v2.concurrency=100 --override smoke.target_url=https://staging.example.com
```

| Flag | Description |
|---|---|
| `--json` | Print the full `SetRun` as JSON instead of the text table |
| `--html FILE` | Write a self-contained HTML report (no external assets) with per-item metrics and a compare table |
| `--threshold metric:op:value[:scope]` | Add a threshold for this run only. Scope defaults to `aggregate`. Repeatable |
| `--override item.field=value` | Override an item for this run only. Fields: `target_url`, `auth_token`, `total_requests`, `rate`, `concurrency`, `duration`, `timeout`. Repeatable |

Flags apply only to this run; the saved set file is not changed.

**Exit codes:**

| Code | Meaning |
|---|---|
| `0` | Set completed and all thresholds passed |
| `1` | Usage error, the set was not found, or a bad `--threshold` / `--override` value |
| `2` | One or more thresholds failed. This is checked first, so it wins even when the set also aborted |
| `3` | The set did not complete (a stage aborted, for example because a session is missing, or the run was cancelled), or a runner or report error occurred |

### `set diff`

Compares two runs of the **same** set. Run IDs are the run directory names under `runs/sets/<set-id>/`.

```bash
crankfire set diff 2026-11-13T02-00-00.000000000Z 2026-11-14T02-00-00.000000000Z
crankfire set diff <a> <b> --json
crankfire set diff <a> <b> --html diff.html
```

Items are matched by name. Each item shows B − A deltas for p50, p95, p99, error rate and RPS. Items that appear in only one run are marked `(A-only)` or `(B-only)` and do not affect the verdict. The overall verdict is:

- **`regressed`** if any item's p95 rose by more than 5%, or its error rate rose by more than 0.005 (0.5 percentage points);
- **`improved`** for the mirror case;
- **`mixed`** if both happened;
- **`unchanged`** otherwise. A diff is informational: it exits `0` whatever the verdict, and `1` only when a run is missing, ambiguous, or the two runs belong to different sets.

## Templates

Templates are reusable set definitions with parameters. They are files you write by hand at `<data-dir>/templates/<id>.yaml`. A template is rendered with Go [`text/template`](https://pkg.go.dev/text/template), using the functions `default`, `lower` and `upper`. Missing parameters render as empty strings. A template may start with a top-level `template: true` line, which is removed before the YAML is parsed. Quote any value that contains template quotes:

```yaml
template: true
name: api-{{ .Env }}-baseline
description: 'Baseline at {{ default "100" .Rate }} rps'
stages:
  - name: load
    items:
      - name: api
        session_id: 01JC9V6M3T6Q3WJ8T6K6Y6R0ZB
        overrides:
          target_url: https://{{ lower .Env }}.example.com
          rate: {{ default "100" .Rate }}
```

Create a set from it:

```bash
crankfire set new --from-template api-baseline --param Env=Staging --param Rate=250
```

The command prints the new set's ID. The TUI template picker (`t` on the sets list) is currently affected by a [known issue](tui.md#sets-screens), so use the CLI.

## Scheduling with `crankfire daemon`

Add a `schedule` to a set, then run the daemon:

```yaml
schedule: "*/15 * * * *"   # every 15 minutes
# schedule: "@daily"
# schedule: "@every 1h"
```

```bash
crankfire daemon                       # uses $CRANKFIRE_DATA_DIR or ~/.crankfire
crankfire daemon --data-dir ./perf
```

Schedules use standard 5-field cron (minute, hour, day of month, month, day of week) or descriptors such as `@hourly`, `@daily` and `@every 5m`. There is no seconds field. Times are in the daemon's local time zone.

The daemon:

- Allows one instance per data directory. It holds `<data-dir>/daemon.lock`, and a second daemon exits with "another daemon is already running".
- Reads schedules at startup. Send **`SIGHUP`** to reload them after you edit set files; the daemon does not watch for changes. One invalid `schedule` in **any** set file stops the daemon from starting (exit 3), and stops a reload partway.
- Skips a fire if the previous run of the same set is still in progress. Fires missed while the daemon was down are not replayed.
- Writes each scheduled run as a normal set run, so you can compare runs with `set diff`.
- Logs JSON lines to stdout, with keys in alphabetical order: `{"event":"fire-start","level":"info","setID":"…","ts":"…"}`. Some events add a `msg` key. Events are `started`, `fire-start`, `fire-completed`, `fire-failed`, `signal`, `reload-ok`, `reload-failed`, `drained`, `drain-timeout` and `stopped`. A run that fails its thresholds still logs `fire-completed`; `fire-failed` means a runner or store error.
- Shuts down on `SIGINT` or `SIGTERM`. Running set runs are cancelled and allowed to finish writing their results.

> **Note:** The single-run guard works only inside one daemon. A manual `crankfire set run` or a run started from the TUI can overlap a scheduled run of the same set.

Example systemd unit:

```ini
[Unit]
Description=Crankfire scheduled load tests

[Service]
Environment=CRANKFIRE_DATA_DIR=/var/lib/crankfire
ExecStart=/usr/local/bin/crankfire daemon
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

## Session CLI

Sessions are created in the [TUI](tui.md), either new or imported from a config file. Their metadata can be changed from scripts:

```bash
crankfire session list
crankfire session list --tag prod --tag smoke,regression   # prod AND (smoke OR regression)
crankfire session edit <id> --add-tag prod --remove-tag old
crankfire session edit <id> --name "checkout baseline" --description "v2 API"
```

`session list` prints `ID NAME TAGS`. `--add-tag` and `--remove-tag` can be repeated, and tags are kept de-duplicated and sorted. Valid tags match `[a-zA-Z0-9._-]{1,64}`.

> **Note:** `crankfire session` and `crankfire set` do not accept `--data-dir`. Set `CRANKFIRE_DATA_DIR` to point them at a non-default directory.

## CI example

```yaml
- name: Run regression set
  env:
    CRANKFIRE_DATA_DIR: ${{ github.workspace }}/perf
  run: |
    crankfire set run 01JC9W0K4ZK8C3V7S8J4E2N1QX --html set-report.html
```

Commit the `perf/` directory (sessions, sets and templates) to the repository. Exclude `perf/runs/`, `perf/**/*.lock` and `perf/daemon.lock` from version control.
{% endraw %}
