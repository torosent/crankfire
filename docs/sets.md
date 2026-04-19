# Sets of Load Tests

A **Set** groups multiple sessions into stages that execute as a coordinated suite. Use sets to:
- Compare protocol/parameter variants side-by-side
- Run a smoke → load → soak pipeline with abort-on-failure gates
- Encode CI/CD pass/fail criteria via thresholds

## Anatomy

```yaml
schema_version: 1
id: 01HXSET1
name: auth-regression
description: Regression suite for login flow
stages:
  - name: warmup
    on_failure: abort     # default; use `continue` to keep running
    items:
      - name: warm
        session_id: 01HSESS0
  - name: load
    items:
      - name: login
        session_id: 01HSESS_LOGIN
        overrides:
          target_url: https://staging.example.com/login
          total_requests: 10000
          concurrency: 50
          headers:
            X-Test-Run: regression
          auth_token: "Bearer ${CI_TEST_TOKEN}"
      - name: search
        session_id: 01HSESS_SEARCH
thresholds:
  - { metric: p95,        op: lt, value: 500, scope: aggregate }
  - { metric: error_rate, op: lt, value: 0.01, scope: per_item }
  - { metric: p99,        op: lt, value: 1000, scope: login }
```

## Override semantics

- `Override` fields are pointers: only fields you set override the session's config.
- `headers` are **merged** with the session's headers (override values win).
- `auth_token` supports `${ENV_VAR}` substitution at run time.

## Thresholds

| metric        | meaning                          |
|---------------|----------------------------------|
| `p50`/`p95`/`p99` | latency percentile (ms)      |
| `error_rate`  | failed/total                     |
| `rps`         | requests per second              |
| `total_errors`| absolute error count             |

`scope`:
- `aggregate` (default) — applied to merged metrics across all items
- `per_item` — applied to every item individually
- `<item-name>` — applied to that item only

## CLI

```sh
crankfire set list
crankfire set show <id>
crankfire set run <id>
crankfire set run <id> --threshold p95:lt:500 --override login.concurrency=100
crankfire set run <id> --json > result.json
crankfire set run <id> --html /tmp/report.html
```

Exit codes: `0` success, `1` usage/load error, `2` threshold failure, `3` runner error.

## TUI

From the sessions list press `s` to switch to the sets list. Inside a set, `e` edits, `R` runs, and `h` opens history. The compare screen shows winners (`★`) per metric.

## On-disk layout

```
$DATA_DIR/
  sets/<id>.yaml
  runs/sets/<id>/<RFC3339Nano>/
    set-run.json
    items/<item-name>/...
```

## Tags

Sessions can be tagged for organizational filtering.

```yaml
# my-session.yaml
tags: [prod, smoke]
```

CLI:

```bash
crankfire session edit <id> --add-tag prod --remove-tag old
crankfire session list --tag prod
crankfire session list --tag prod --tag smoke,regression  # AND of OR
```

In the TUI sessions/sets list, press `/` to open a slash-search prompt.
Filter syntax: spaces = AND, commas = OR.

## Templates

Templates live at `<dataDir>/templates/<id>.yaml` and use
[Go text/template](https://pkg.go.dev/text/template) with these funcs:
`default`, `lower`, `upper`. Missing params render empty.

A template differs from a regular Set only by the top-level marker:

```yaml
template: true
name: api-{{ .Env }}-baseline
description: rate {{ default "100" .Rate }}
stages:
- name: smoke
  items: []
```

Materialize into a Set:

```bash
crankfire set new --from-template api-baseline --param Env=prod --param Rate=500
```

In the TUI, press `t` from the sets list to open the picker.

## Schedules

A Set can declare a cron schedule that fires runs when `crankfire daemon`
is running:

```yaml
schedule: "*/5 * * * *"  # every 5 minutes
schedule: "@daily"        # macros also work
```

Run the daemon in the foreground:

```bash
crankfire daemon --data-dir ~/.crankfire
```

The daemon:
- Holds an exclusive lock at `<dataDir>/daemon.lock`.
- Skips overlapping fires.
- Reloads schedules on `SIGHUP`.
- Drains in-flight runs (up to 30s) on `SIGINT`/`SIGTERM`.
- Logs JSON lines to stdout.

Missed fires during downtime are silently skipped.

## Diff

Compare any two runs of the same set:

```bash
crankfire set diff <run-id-a> <run-id-b>
crankfire set diff <a> <b> --json
crankfire set diff <a> <b> --html out.html
```

Verdict heuristic: any item with P95 latency +5% OR error-rate +0.5pp
counts as a regression. CLI always exits 0 — diff is informational.

In the TUI, on the set history screen, mark two runs with `space`, then
press `d`.
