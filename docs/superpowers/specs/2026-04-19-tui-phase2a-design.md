# TUI Phase 2a — Sets of Load Tests: Design Spec

**Status:** Approved 2026-04-19
**Depends on:** Phase 1 (`docs/superpowers/specs/2026-04-19-tui-phase1-design.md`)
**Defers to Phase 2b:** tags/filters, set templates, cron schedule, history-diff across set runs

---

## 1. Goal

Define and execute *Sets* of load-test sessions: ordered stages of one-or-more sessions
with structured per-item overrides, run them sequentially within a stage / in parallel
within an item group, surface a side-by-side comparison view (live in TUI + post-run
HTML), and gate CI with set-level thresholds via a headless `crankfire set run` command.

This delivers four user-visible capabilities on top of Phase 1:

- **Set CRUD** in the TUI and on disk.
- **Mixed-pipeline execution** (sequential stages, parallel items within a stage).
- **Comparison view** (live TUI + static HTML report).
- **Headless `crankfire set run`** with non-zero exit on threshold failure (CI gate).

## 2. Non-goals

- Tags / filters across sets (Phase 2b).
- Set templates (Phase 2b).
- Cron-style scheduled runs (Phase 2b).
- History-diff across past set runs (Phase 2b).
- Cross-machine distributed sets (out of scope, possibly never).
- Editing of sessions from inside the set screens — sets *reference* sessions by ID.

## 3. Architecture overview

```
                 ┌────────────────────────────────────────────────┐
                 │              cmd/crankfire/main.go             │
                 │   dispatches: tui | set | (default cli.Run)    │
                 └────────┬────────────────────────┬──────────────┘
                          │                        │
                          ▼                        ▼
              ┌────────────────────────┐  ┌──────────────────────┐
              │ internal/cli           │  │ internal/cli/set.go  │
              │ Run / BuildRunner      │  │ Set list / run (CI)  │
              └────────────┬───────────┘  └──────────┬───────────┘
                           │                         │
                           ▼                         ▼
                ┌──────────────────────────────────────────┐
                │          internal/setrunner             │
                │  Orchestrator, Overrides, Compare        │
                │  - Walks stages sequentially             │
                │  - Fans items in a stage to goroutines   │
                │  - Calls cli.BuildRunner per item        │
                │  - Aggregates Stats, evaluates thresholds│
                └──────────┬───────────────────────────────┘
                           │
              ┌────────────┴───────────────┐
              ▼                            ▼
   ┌─────────────────────┐      ┌──────────────────────────┐
   │ internal/store      │      │ internal/output/setreport│
   │ + SetStore methods  │      │ HTML compare report       │
   └─────────────────────┘      └──────────────────────────┘
                           │
                           ▼
                ┌─────────────────────────────────────┐
                │      internal/tui/screens           │
                │  sets_list / sets_detail /          │
                │  sets_edit / set_run / compare /    │
                │  sets_history                       │
                └─────────────────────────────────────┘
```

**Reuse from Phase 1 (no rewrites):**
`cli.BuildRunner`, `internal/runner`, `internal/metrics`, `internal/store` (extended),
`internal/tui/runview`, `internal/tui/widgets`, screen-stack push/pop, `validateID()`.

## 4. Data model

### 4.1 Set

Stored at `<dataDir>/sets/<set-id>.yaml`. ULID `id`, validated by the same `validateID()`
introduced in Phase 1's hardening.

```yaml
schema_version: 1
id: 01HX...                 # ULID, store-owned, immutable
name: "auth-regression"
description: "Smoke + sustained load against staging auth endpoints"
created_at: 2026-04-19T12:00:00Z
updated_at: 2026-04-19T12:00:00Z

thresholds:                 # optional; evaluated post-run; failures => CI exit 2
  - metric: p95
    op: lt
    value: 500              # ms
    scope: aggregate        # aggregate | per_item | "<item-name>"
  - metric: error_rate
    op: lt
    value: 0.01
    scope: aggregate

stages:                     # ordered; each stage runs after the previous completes
  - name: warmup
    on_failure: continue    # continue | abort  (default: abort)
    items:                  # within a stage: parallel
      - name: warmup-auth   # human-readable; defaults to session name
        session_id: 01HW... # required; references existing Session
        overrides:          # optional structured patch (typed, see §4.3)
          total_requests: 100
          rate: 10
          target_url: "https://stg.example.com/auth"
  - name: load
    items:
      - name: login-load
        session_id: 01HW...
        overrides:
          duration: 60s
          concurrency: 50
      - name: token-refresh-load
        session_id: 01HW...
```

Validation on save (in `SaveSet`):

- `name` non-empty, `len ≤ 200`.
- At least one `stage` with at least one `item`.
- Every `session_id` resolves via `GetSession`; otherwise reject with `ErrInvalidSet`.
- Every `item.name` (defaulted from session name) is unique within its set.
- Every `threshold` parses (see §4.4).

### 4.2 SetRun

Stored at `<dataDir>/runs/sets/<set-id>/<RFC3339Nano>/`:

```
runs/sets/<set-id>/2026-04-19T12-30-00.000000000Z/
├── set-run.json            # aggregate metadata (see below)
├── compare.html            # static comparison report
├── compare.json            # structured comparison data (machine-readable)
└── items/
    ├── warmup-auth/        # one dir per item; matches item.name
    │   ├── result.json     # written by existing run path
    │   └── report.html
    ├── login-load/...
    └── token-refresh-load/...
```

`set-run.json` schema:

```json
{
  "schema_version": 1,
  "set_id": "01HX...",
  "set_name": "auth-regression",
  "started_at": "2026-04-19T12:30:00Z",
  "ended_at":   "2026-04-19T12:32:14Z",
  "status": "completed",                  // running | completed | failed | cancelled
  "stages": [
    {
      "name": "warmup",
      "started_at": "...", "ended_at": "...",
      "items": [
        {
          "name": "warmup-auth",
          "session_id": "01HW...",
          "run_dir": "items/warmup-auth",
          "status": "completed",
          "summary": { /* RunSummary from Phase 1 */ }
        }
      ]
    }
  ],
  "thresholds": [
    {
      "metric": "p95", "op": "lt", "value": 500, "scope": "aggregate",
      "actual": 312.4, "passed": true
    }
  ],
  "all_thresholds_passed": true,
  "error_message": ""
}
```

### 4.3 Overrides — structured, typed

Defined in `internal/setrunner/overrides.go`:

```go
type Override struct {
    TargetURL      *string           `yaml:"target_url,omitempty"`
    Method         *string           `yaml:"method,omitempty"`
    Headers        map[string]string `yaml:"headers,omitempty"`     // merged onto base
    Body           *string           `yaml:"body,omitempty"`
    TotalRequests  *int              `yaml:"total_requests,omitempty"`
    Rate           *int              `yaml:"rate,omitempty"`
    Concurrency    *int              `yaml:"concurrency,omitempty"`
    Duration       *time.Duration    `yaml:"duration,omitempty"`
    Timeout        *time.Duration    `yaml:"timeout,omitempty"`
    AuthToken      *string           `yaml:"auth_token,omitempty"` // env-style; reads $VAR if "${...}"
    Tags           map[string]string `yaml:"tags,omitempty"`        // free-form labels added to metrics
}

// Apply returns a clone of base with non-nil fields replaced and Headers merged.
func (o Override) Apply(base config.Config) config.Config
```

- Pointers distinguish "not set" from "set to zero".
- `Headers` is merged (not replaced) so callers can extend a base header set.
- `AuthToken` supports `${ENV_NAME}` substitution, resolved at run time
  (not at save time) — so set YAML can be committed without secrets.
- Unknown YAML fields under `overrides:` cause a save-time validation error
  to catch typos.

### 4.4 Thresholds

Format borrowed from Phase 1's `internal/threshold` package style:

```yaml
- metric: p50 | p95 | p99 | error_rate | rps | total_errors
  op: lt | le | gt | ge | eq
  value: <number>
  scope: aggregate | per_item | "<item-name>"
```

- `aggregate` evaluates against the set's combined stats.
- `per_item` requires the threshold to hold for **every** item.
- `"<item-name>"` evaluates only against that item.
- Existing `internal/threshold` package is reused for the actual evaluator;
  `setrunner` adapts its inputs (per-item `Stats` and an aggregated `Stats`).

## 5. Components

### 5.1 `internal/store` extensions

New methods on the `Store` interface:

```go
ListSets(ctx) ([]Set, error)
GetSet(ctx, id string) (Set, error)
SaveSet(ctx, s Set) error                // validates all session_ids resolve
DeleteSet(ctx, id string) error
CreateSetRun(ctx, setID string) (SetRun, error)
FinalizeSetRun(ctx, run SetRun) error    // run already populated
ListSetRuns(ctx, setID string) ([]SetRun, error)
```

`SetRun` includes `Dir` for nested artifact writes by `setrunner` and the
HTML report generator. Every method that consumes an ID validates with
the existing `validateID()`.

### 5.2 `internal/setrunner`

The orchestrator. Owns no UI. Pure functions + one `Run` driver:

```go
type Runner struct {
    Store   store.Store
    Build   func(ctx context.Context, cfg config.Config) (*runner.Runner,
                                                          *metrics.Collector,
                                                          func(), error)
}

type Progress struct {
    Stage       string
    StageIndex  int
    Item        string
    Snapshot    metrics.DataPoint   // CurrentRPS, etc.
    PerItem     map[string]metrics.Stats // last-known per-item stats
}

// Run executes the set, sending Progress updates on the returned channel.
// The channel closes when the run finishes (success, failure, or cancelled).
func (r *Runner) Run(ctx context.Context, set store.Set, run *store.SetRun) (
    <-chan Progress, error)
```

Implementation notes:

- One goroutine per item within a stage.
- Per-item context derives from the parent `ctx` so `Esc`/`Ctrl-C` cancels everything.
- Stage `on_failure: abort` (default) cancels remaining stages on first item failure.
- After all stages: build the `compare.json` + `compare.html`, evaluate thresholds,
  populate `SetRun`, call `FinalizeSetRun`.
- Per-item stats sourced from each item's `metrics.Collector.Snapshot()`.

### 5.3 `internal/setrunner/compare.go`

Pure transform: `[]ItemResult` → `Comparison`:

```go
type Comparison struct {
    Items   []ComparisonItem
    Metrics []ComparisonMetric  // one row per metric (p50, p95, ...)
    Baseline string             // first item by default; used for delta column
}

type ComparisonItem struct {
    Name   string
    Stats  metrics.Stats
}

type ComparisonMetric struct {
    Name   string                  // "p95"
    Values map[string]float64      // item name -> value
    Deltas map[string]float64      // item name -> %diff vs baseline (Baseline omitted)
}
```

Used by both `internal/output/setreport` (HTML) and `internal/tui/screens/compare.go`.

### 5.4 `internal/output/setreport`

Single function:

```go
func WriteHTML(comp Comparison, set store.Set, run store.SetRun, w io.Writer) error
```

Template style follows existing `internal/output` HTML report — table-driven, no JS
dependencies, sparkline rendered as inline SVG per item.

### 5.5 `internal/cli/set.go`

Subcommand router invoked from `cmd/crankfire/main.go` when first arg is `set`:

```
crankfire set list                          # prints id  name  stages  last-run-status
crankfire set run <name|id> [flags]
   --data-dir <path>
   --json                                   # emit set-run.json to stdout on completion
   --no-html                                # skip writing compare.html
   --threshold "metric op value [scope]"   # add ad-hoc thresholds (repeatable)
   --override <item-name>:key=value         # ad-hoc override (repeatable; structured)
```

Exit codes:

- `0` — set completed and all thresholds passed.
- `1` — usage / config / load error.
- `2` — set ran but at least one threshold failed.
- `3` — at least one item failed to run (runner error).
- `130` — cancelled (SIGINT).

### 5.6 TUI screens

New files under `internal/tui/screens/`:

- `sets_list.go` — analogous to Phase 1 sessions list; keys
  `n` new, `e` edit, `d` delete, `r` run, `h` history, Enter detail, `s` toggle to Sessions, `q` quit.
- `sets_detail.go` — name, description, stages (count + names), last run status, threshold list.
- `sets_edit.go` — guided form (name, description, thresholds editor) + `F2` toggle to YAML, mirroring Phase 1's edit screen.
- `set_run.go` — live screen. Shows stage progress (e.g. `[1/3] load`), per-item cards in a vertical list (each card = compact `runview`: progress bar + RPS sparkline + p95 + errors), threshold banner that updates on completion.
- `set_history.go` — list past `SetRun`s for a set; Enter opens the comparison screen for that run; `o` opens the run's `compare.html`.
- `compare.go` — post-run side-by-side table reusing `widgets.PercentileTable` with delta column highlighted.

The Sessions list gets one new key (`s`) to push the Sets list, and vice versa.
This is the only Phase 1 file modification beyond store/types.

### 5.7 `cmd/crankfire/main.go` dispatcher

```go
switch firstArg {
case "tui":  // existing
case "set":  // new — calls cli.RunSet(args[1:])
default:     // existing cli.Run(args)
}
```

## 6. Data flow: a complete `set run`

```
1. cli.RunSet
     └─ store.GetSet(ctx, id)
2. setrunner.Runner.Run(ctx, set, run)
     ├─ for each stage (sequential):
     │    ├─ create per-item ctx (derived from set ctx)
     │    ├─ for each item in stage (parallel goroutines):
     │    │    ├─ apply Override.Apply(session.Config) → effective Config
     │    │    ├─ store.CreateRun(ctx, session_id)         # individual run record
     │    │    ├─ cli.BuildRunner(ctx, effectiveCfg)       # reused
     │    │    ├─ runner.Run(ctx)                          # blocks
     │    │    ├─ collect metrics.Stats
     │    │    ├─ store.FinalizeRun(...)                   # phase-1 path
     │    │    └─ write items/<name>/result.json + report.html (existing helpers)
     │    └─ wait; if any item failed and on_failure=abort, cancel remaining stages
     ├─ comp := compare.Build([]ItemResult)
     ├─ output/setreport.WriteHTML(comp, set, run)
     ├─ thresholds := evaluate(set.Thresholds, comp, perItemStats)
     ├─ populate SetRun.Stages / Thresholds / Status
     └─ store.FinalizeSetRun(run)
3. cli.RunSet exits with code based on threshold + run statuses.
```

## 7. Error handling

- `setrunner` per-item errors are captured in the item's `Status` + `error` field
  in `set-run.json`; the set as a whole continues unless `on_failure: abort`.
- Cancellation propagates through context. `FinalizeSetRun` is always called in a
  deferred block so partial state is persisted.
- Threshold parse errors at `SaveSet` time, not at run time.
- HTML report generation failure is non-fatal (logged as warning; `compare.json` still written).
- Atomic writes for `set.yaml` and `set-run.json` follow Phase 1's `writeAtomic` pattern.

## 8. Testing strategy

Unit tests (no build tag):

- `internal/store`: SetStore CRUD round-trip, validation rejects sets referencing
  unknown sessions, path-traversal IDs, atomic save under flock.
- `internal/setrunner/overrides`: Apply correctness — pointer fields,
  Headers merge, env var substitution, unknown-field rejection.
- `internal/setrunner/compare`: deterministic Comparison from fixed `[]ItemResult`,
  baseline selection, delta math.
- `internal/setrunner`: orchestrator with a fake `Build` that returns a stub runner;
  verifies stage ordering, parallelism within a stage (use channels to assert
  concurrent start), `on_failure: abort` semantics, cancel via context.
- `internal/output/setreport`: golden-file HTML render.
- `internal/cli/set`: arg parsing, exit codes given fake fixture runs.
- `internal/tui/screens`: model-update tests for each new screen (key handling,
  rendering with fake store data) — same style as Phase 1.

Integration test (`//go:build integration`):

- `cmd/crankfire/set_integration_test.go`: spin up `httptest.Server`, create two
  sessions targeting it via the store API, build a 2-stage set
  (warmup parallel-1, load parallel-2 with override `total: 100`), run via
  `cli.RunSet` against the live server, assert exit code 0, assert
  `set-run.json` and `compare.html` exist, assert one threshold pass + one
  intentional fail produces exit code 2.

Verification commands (must all pass before tasks are marked done):

```bash
go build ./...
go test -race ./...
go test -v -tags=integration -race -timeout 15m ./...
```

## 9. Security considerations

- `validateID()` already gates every fs path on the `Store` side; `SaveSet`
  also validates each `item.session_id` resolves to a real session.
- Unknown YAML fields under `overrides:` cause `SaveSet` to reject the set
  (prevents silent drift if the schema gains a field).
- `AuthToken` env substitution happens at runtime only; raw `${ENV}` is what's
  on disk. Never log resolved values.
- HTML report generator uses `html/template` (auto-escaped); no user-provided
  HTML can land in the report.

## 10. Observability

- Each item run already emits OTEL spans (Phase 1 tracing). Add a parent span
  `crankfire.set_run` with `set.id`, `set.name`, `stage.name`, `item.name`
  attributes, parented to the user-supplied `--tracing-endpoint` config if any.
- `compare.html` renders an OTEL trace ID per item if tracing was enabled
  (rendered as plain text in the table; turning the ID into a clickable link to
  the user's trace backend is deferred to Phase 2b).

## 11. Open questions deferred to Phase 2b

- Tags / filters across sets and sessions.
- Set templates (parameterised sets).
- Cron schedule + daemon mode.
- History-diff: comparing two past `SetRun`s of the same set.

## 12. Migration / compatibility

- Pure additions. Existing Phase 1 behaviour, files, and YAML schemas are unchanged.
- New top-level dirs: `<dataDir>/sets/`, `<dataDir>/runs/sets/`.
- New `Store` methods are additive; existing implementations satisfy the old contract.
- New CLI subcommand `set`; no flag collisions with existing root command.

## 13. Acceptance criteria

A user can:

1. Open `crankfire tui`, press `s` to switch to Sets, press `n` to create a set
   referencing two existing sessions in two stages, save it.
2. Press `r`, watch the set run live with per-item progress + RPS sparklines, see
   stage indicator advance, and land on the Compare screen with a delta column.
3. Open `~/.crankfire/runs/sets/<id>/<ts>/compare.html` in a browser and see
   the same data.
4. Run `crankfire set run auth-regression --threshold "p95 lt 500 aggregate"`
   in CI; on threshold failure the command exits 2.

## 14. Estimated implementation footprint

- **New packages:** `internal/setrunner`, `internal/output/setreport`.
- **New files:** ~15 source + ~15 test under existing packages.
- **Modified files:** `internal/store/store.go`, `internal/store/fs.go`,
  `cmd/crankfire/main.go`, `internal/tui/screens/list.go` (one new key binding),
  `README.md`, `docs/tui.md` (or new `docs/sets.md`).
- **No removed files.**
