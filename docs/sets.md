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
