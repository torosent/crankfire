# TUI Phase 2b — Tags, Templates, Schedules, Diff, Trace Backends: Design Spec

**Status:** Approved 2026-04-19
**Depends on:** Phase 2a (`docs/superpowers/specs/2026-04-19-tui-phase2a-design.md`)
**Out of scope:** distributed daemon (single-host only), per-tenant schedules, web UI

---

## 1. Goal

Land the five capabilities Phase 2a explicitly deferred — in one focused but
multi-feature release — without breaking any Phase 1 / 2a behavior:

1. **Tags & filters** on sessions, with `/`-search in the TUI sessions list and
   sets list (the latter filters transitively via item-referenced sessions).
2. **Set templates** — `templates/<id>.yaml` rendered through Go `text/template`,
   instantiated to a real `sets/<id>.yaml` via `crankfire set new --from-template`.
3. **Cron schedules** on sets, fired by an explicit `crankfire daemon` foreground
   process.
4. **History-diff** between any two `SetRun`s of the same set, both in the TUI
   (multi-select on the history screen) and via `crankfire set diff`.
5. **Trace-backend integration** — pure docs (`docs/tracing-backends.md`) with
   verified configs for Tempo, Jaeger, Honeycomb, OTLP collector, and a local
   Jaeger Docker recipe. No code.

The result: Crankfire becomes a tag-aware, parameterizable, schedulable load
tester whose results can be diffed against past runs without leaving the binary.

## 2. Non-goals

- **Tags on Sets directly.** Tags live on Sessions only; Sets inherit by
  transitive filter. (Avoids two filter scopes and double-bookkeeping.)
- **Key/value tags.** Tags are plain string labels (`prod`, `smoke`,
  `regression`). No `env=prod` syntax.
- **Declared template-parameter schema.** Templates use freeform Go-template
  variables; missing keys render empty. No CLI prompts; no TUI param form
  beyond a generic "add key/value" row.
- **Daemon lifecycle CLI** (`daemon stop` / `daemon status`). Users use signals
  + the lockfile. Keep the surface area minimal.
- **Schedule backfill / catch-up.** Missed fires while the daemon is down are
  silently skipped — only the next future fire matters.
- **Trace ID storage / deep-linking.** The trace-backend story stays at the
  documentation layer in this phase. Capturing trace IDs alongside RunSummary
  and rendering "View trace" links is deferred.
- **Cross-host distributed sets, multi-tenant scheduling, or a web/REST API.**

## 3. Architecture overview

Phase 2b is a strict superset of Phase 2a — same packages, same data flow, just
new pieces added at the edges:

```
                 ┌────────────────────────────────────────────────┐
                 │              cmd/crankfire/main.go             │
                 │   dispatches: tui | set | daemon | cli.Run     │
                 └─┬──────────┬──────────┬──────────┬─────────────┘
                   │          │          │          │
                   ▼          ▼          ▼          ▼
              ┌────────┐ ┌────────┐ ┌─────────┐ ┌────────────┐
              │  tui   │ │  cli   │ │ daemon  │ │ legacy CLI │
              └───┬────┘ └───┬────┘ └────┬────┘ └─────┬──────┘
                  │          │           │            │
                  └─────┬────┴────┬──────┘            │
                        │         │                   │
                        ▼         ▼                   ▼
                ┌──────────────────────────────────────────┐
                │              internal/store              │
                │  + Session.Tags + templates/ + lock      │
                └──┬─────────────────────────┬─────────────┘
                   ▼                         ▼
          ┌────────────────┐        ┌────────────────────────┐
          │ internal/      │        │  internal/scheduler    │ NEW
          │ template       │ NEW    │  (cron.Cron + ticker)  │
          │ (text/template │        └──────────┬─────────────┘
          │  + render)     │                   │
          └────────────────┘                   ▼
                                  ┌──────────────────────────┐
                                  │  internal/setrunner      │
                                  │  + diff.go (Phase 2b)    │
                                  │  Runner.Run unchanged    │
                                  └──────────────────────────┘
```

### New packages

- **`internal/template/`** — single-purpose Go-template renderer over raw YAML
  bytes. Turns a template + `map[string]string` of params into an unmarshaled
  `store.Set`. Pure function; no I/O. Unit-tested against happy-path,
  missing-key (zero), and invalid-template inputs.
- **`internal/scheduler/`** — wraps `github.com/robfig/cron/v3`. Owns the
  `Manager` that tracks cron entries by SetID, supports `Add`, `Remove`,
  `Reload(ctx)`, and `Run(ctx)` (blocking; runs until ctx cancelled). Calls
  back into a caller-supplied `func(setID string)` on each fire — the daemon
  binds that to the existing `setrunner.Runner.Run`.

### Touched existing packages

- **`internal/store`**: add `Tags []string` to `Session`; new
  `ListTemplates/GetTemplate/SaveTemplate/DeleteTemplate` on the `Store`
  interface; new `Set.Schedule string` field; `daemon.lock` path helper.
- **`internal/setrunner`**: new `diff.go` with `Diff(a, b store.SetRun) DiffResult`
  reusing `Compare`'s row computation and adding delta columns.
- **`internal/cli`**: `set new --from-template`, `set diff`, `daemon`,
  plus `--tag` flag on `set list` and a new `session list --tag` subcommand.
- **`internal/tui/screens`**: slash-search bar shared by sessions list + sets
  list; `t` template-picker on sets list; multi-select + `d` on sets-history
  screen; new diff screen.
- **`docs/`**: new `tracing-backends.md`; updates to `sets.md` and `README.md`.

### Disk layout (additive)

```
<dataDir>/
  sessions/<id>.yaml         # + tags: [prod, smoke]
  sets/<id>.yaml             # + schedule: "0 2 * * *"
  templates/<id>.yaml        # NEW — template: true, body uses {{ .X }}
  runs/sets/<set-id>/<ts>/   # unchanged
  daemon.lock                # NEW — flock'd by `crankfire daemon`
```

Schema versions bump to `1` for sessions and sets (Phase 2a was `0`); old files
load fine via additive YAML decoding (`Tags` and `Schedule` default to zero).

## 4. Tags & filters

### 4.1 Data model

```go
// internal/store/store.go
type Session struct {
    // ...existing fields...
    Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`
}
```

`SaveSession` deduplicates and sorts `Tags` for deterministic YAML output.
Tag values must match `^[a-zA-Z0-9._-]{1,64}$` (otherwise `ErrInvalidTag`).

### 4.2 Filter syntax

A filter expression is a string of **AND-groups separated by spaces**, each
AND-group a **comma-separated OR-list**:

```
tag1                     → has tag1
tag1 tag2                → has tag1 AND has tag2
tag1,tag2                → has tag1 OR has tag2
prod smoke,regression    → has prod AND (has smoke OR has regression)
```

Implementation: `internal/tagfilter/parser.go` exposes
`Parse(s string) (Matcher, error)` where `Matcher.Matches(tags []string) bool`
runs the AND-of-ORs match. Pure, table-tested.

### 4.3 TUI behavior

Both `screens.SessionsList` and `screens.SetsList` add:

- `/` opens a single-line filter prompt at the bottom row (lipgloss-styled,
  textinput.Model). `Enter` applies; `Esc` cancels and clears.
- An **active-filter chip** is rendered just below the title row when a filter
  is set: `[ filter: prod smoke ]   x:clear`. Pressing `x` clears.
- Filtering is purely client-side: load all sessions/sets once, then re-filter
  the slice on every keystroke for instant feedback. No store re-query.
- Sets list filter is **transitive**: a set matches if **any** of its items'
  referenced sessions match the filter. (We resolve item → session lazily and
  cache per render.)

### 4.4 CLI behavior

```
crankfire session list --tag prod --tag smoke,regression
crankfire session edit <id> --add-tag X --remove-tag Y
crankfire set list --tag prod
```

`--tag` is repeatable; each repetition is one AND-group. Inside a single
`--tag`, comma = OR. `session edit` is the new headless tag-management entry
point — no full editor, just tag mutation.

## 5. Set templates

### 5.1 Storage

```
<dataDir>/templates/<id>.yaml
```

Schema is identical to Phase 2a's `Set`, plus a top-level `template: true`
marker. `SaveTemplate` rejects files without that marker; `SaveSet` rejects
files **with** it (so users cannot accidentally `set run` a template).

### 5.2 Rendering

```go
// internal/template/render.go
func Render(rawYAML []byte, params map[string]string) ([]byte, error)
```

- Uses `text/template` with `Option("missingkey=zero")` so missing params
  render empty rather than `<no value>`.
- Funcs registered: `default`, `lower`, `upper` (small useful set; not full
  Sprig — YAGNI).
- The output is then passed to `yaml.Unmarshal` into `store.Set` for
  validation; any error is surfaced as `ErrInvalidTemplate`.

### 5.3 Instantiation

**CLI:**

```
crankfire set new --from-template <tpl-id> \
                  --param Rate=1000 --param Env=prod \
                  --name "smoke-prod"
```

Behavior: load template → render with params → unmarshal as `Set` → assign a
fresh ULID (and `--name` if provided, else copy template's name with a `-NNNN`
suffix) → `SaveSet` → print the new set ID. Exits non-zero on render or
validation failure.

**TUI:** From the sets-list screen, `t` opens a template-picker (list of
`templates/`). Enter selects → opens a one-off **freeform key/value form**
(rows of `key | value` plus `[+ add row]`). Submit triggers the same
render-and-save flow; the screen then jumps into the existing sets-edit
screen with the freshly-instantiated set loaded so the user can refine
before running.

**No declared parameter schema.** Authors document their own params in the
template's `description:` field. Trade-off accepted in Section 9.

## 6. Cron schedules + daemon

### 6.1 Set field

```go
// internal/store/sets.go
type Set struct {
    // ...existing fields...
    Schedule string `yaml:"schedule,omitempty" json:"schedule,omitempty"`
}
```

`Schedule` is either empty (no schedule) or a 5-field cron expression compatible
with `robfig/cron/v3` standard parser (also accepts `@daily`, `@hourly`, etc.).
`SaveSet` validates by attempting to parse; invalid expressions return
`ErrInvalidSchedule`.

Timezone: **local time of the daemon process**. Documented; no per-set TZ in 2b.

### 6.2 Scheduler package

```go
// internal/scheduler/manager.go
type Manager struct { /* wraps *cron.Cron */ }

func New(onFire func(ctx context.Context, setID string)) *Manager
func (m *Manager) AddOrReplace(setID, expr string) error
func (m *Manager) Remove(setID string)
func (m *Manager) Reload(ctx context.Context, st store.Store) error
func (m *Manager) Run(ctx context.Context)   // blocks until ctx done
```

`Reload` lists all sets, computes the desired set of (setID, expr) pairs, and
diffs against current entries (add new, remove deleted, replace changed).
Logs each transition.

### 6.3 Daemon

```
crankfire daemon [--data-dir DIR]
```

Behavior:

1. Resolve data dir, open store, acquire `<data-dir>/daemon.lock` via flock
   (LOCK_EX | LOCK_NB). If contention: log clear message, exit `ExitUsage`.
2. Build a `scheduler.Manager` with `onFire = runSet` where `runSet` calls
   `setrunner.Runner.Run(ctx, setID, nil)` and logs the result.
3. `Reload` once at startup.
4. Install signal handlers:
   - `SIGINT` / `SIGTERM` → cancel root ctx → `Manager` stops accepting new
     fires → wait up to 30s for in-flight `Runner.Run` to drain → release
     lock → exit 0.
   - `SIGHUP` → call `Reload` (re-scan sets dir, swap entries).
5. Logs as JSON lines on stdout (`{"ts":"…","level":"info","setID":"…","event":"fire-start"}`).
   Errors as `level: "error"`. No file logging — let the user pipe to whatever.

Each fired run produces an ordinary `SetRun` under
`runs/sets/<set-id>/<ts>/`, so it shows up in TUI history alongside
manually-triggered runs.

**Concurrency policy:** if a set is still running when its next fire arrives,
the new fire is **dropped** with a warning log. (Avoids overlapping runs of
the same set; documented.)

### 6.4 Out of scope here

- Web UI / REST control plane.
- Multi-host coordination (just one daemon per data dir).
- Authentication; daemon is local-only.
- Persistent "last fire" log for backfill — if you need it, run cron yourself.

## 7. History-diff

### 7.1 Engine

```go
// internal/setrunner/diff.go
type DiffRow struct {
    ItemName     string
    Stage        string
    P50DeltaMs   float64
    P95DeltaMs   float64
    P99DeltaMs   float64
    ErrRateDelta float64   // signed; negative = improvement
    RPSDelta     float64   // signed; positive = improvement
    APresent     bool
    BPresent     bool
}

type DiffResult struct {
    A, B           store.SetRun
    Rows           []DiffRow
    OverallVerdict string  // "improved", "regressed", "mixed", "unchanged"
}

func Diff(a, b store.SetRun) DiffResult
```

`OverallVerdict` heuristic: any `P95DeltaMs > +5%` or `ErrRateDelta > +0.005`
on **any** row → `"regressed"`; symmetric for `"improved"`; both → `"mixed"`;
neither → `"unchanged"`. Thresholds are constants in the file, easy to tune.

Reuses `Compare`'s per-item row math; adds the deltas. Pure function;
table-tested.

### 7.2 CLI

```
crankfire set diff <run-id-a> <run-id-b> [--json | --html PATH]
```

Default output is a text table with arrow indicators (`↑` regression for
latency/error, `↓` improvement). Always exits 0 — diffing is informational,
not a gate. (Future could add a `--fail-on-regression` flag; not in 2b.)

Run-ID resolution: full path scan under `runs/sets/*/*/` for `set-run.json`
files whose ID matches; ambiguous IDs across different sets error out with a
"please disambiguate by prefix" message.

### 7.3 TUI

In the existing `screens.SetsHistory` screen:

- `space` toggles a "marked" flag on the cursor row; status bar shows
  `[N selected]`.
- `d` is enabled only when **exactly 2** rows are marked. Pressing `d` opens
  a new `screens.SetDiff` screen rendering the same `DiffResult`.
- `c` (or `Esc` in the new screen) returns to history.

The diff screen is read-only; no editing, no actions besides "back."

## 8. Trace backend integration (docs only)

New file: `docs/tracing-backends.md`.

Sections (each ~40 lines, copy-pasteable):

1. **Tempo (Grafana Cloud)** — OTLP gRPC, Basic-auth header in env.
2. **Tempo (self-hosted)** — OTLP gRPC, no auth.
3. **Jaeger (OTLP receiver)** — OTLP gRPC, default port.
4. **Honeycomb** — OTLP HTTP, `x-honeycomb-team` header in env.
5. **OpenTelemetry Collector sidecar** — generic OTLP gRPC config.
6. **Local Jaeger via Docker** — single `docker run …` line + verified
   Crankfire flags + screenshot of expected UI behavior.

Each section lists:
- The exact `--tracing-*` flags (or `tracing:` YAML stanza).
- Equivalent `OTEL_*` environment variables.
- Sample header config for auth.
- A "what you'll see" paragraph describing the trace tree shape Crankfire
  produces (one root span per request; child spans for connect / TLS / write /
  read where applicable).

`README.md` gains a one-line link to it under the "Observability" section.
No code changes.

## 9. Trade-offs accepted

- **Freeform template params** mean no compile-time validation that callers
  pass the right keys, no auto-discovery, and TUI param entry is generic
  rather than guided. Trade-off: massively simpler implementation; param
  documentation lives in the template's `description` field. If we ever need
  guided UX, we add an optional `parameters:` block later (forward-compatible).
- **Skip-missed schedule policy** can surprise users coming from cron
  expecting catch-up. Trade-off: avoids burst-on-restart problems and a
  persistent fire-log; documented prominently.
- **Single daemon per data dir** (no clustering) is fine for local /
  single-CI-runner use cases, which is Crankfire's current target.
- **Trace integration is docs-only.** Capturing trace IDs alongside
  RunSummary and rendering deep-links would require schema changes and TUI
  rework; deferring keeps 2b focused.
- **Tags as plain labels, sessions-only.** Sets inherit transitively. Loses
  `env=prod`-style structured tags and direct set tagging; gains schema
  simplicity. Forward-compatible if we add structured tags later (labels
  remain a special case).

## 10. Failure modes

- **Invalid template** at `set new` time → `ExitUsage` with line:col from
  text/template's error. Original template untouched.
- **Invalid schedule** at `SaveSet` time → `ErrInvalidSchedule` surfaced in
  TUI editor / CLI. Set NOT persisted.
- **Daemon lock contention** → exit `ExitUsage` with "another daemon is
  running, see <data-dir>/daemon.lock".
- **Set fires while previous run still in flight** → log warning, drop fire.
  (Tested.)
- **Diff against runs of different sets** → `set diff` errors with
  "runs belong to different sets (<a-set-id> vs <b-set-id>)".
- **Tag parse error** in CLI `--tag` → exit `ExitUsage` with offending token.
- **HUP during in-flight run** → reload completes; in-flight run is
  unaffected (we only swap entries, never cancel running fires).

## 11. Migration / compatibility

- Pure additions. Existing Phase 1 / 2a behavior, files, and YAML schemas
  are unchanged.
- New `Tags` field on `Session` and `Schedule` field on `Set` default to
  zero — old YAML files load without modification.
- New top-level dir: `<dataDir>/templates/`. New file:
  `<dataDir>/daemon.lock`.
- New `Store` methods (`ListTemplates`, `GetTemplate`, `SaveTemplate`,
  `DeleteTemplate`) extend the interface; existing in-memory test fakes from
  Phase 2a need a one-line stub addition.
- New CLI subcommand `daemon`; `set` gets new flags (`--tag`, `--from-template`,
  `--param`) and new `set new` / `set diff` subcommands. No flag collisions.
- `SchemaVersion` for sessions and sets bumps to `1`. Loader accepts both `0`
  and `1` (additive fields default zero).

## 12. Acceptance criteria

A user can:

1. Tag a session with `tags: [prod, smoke]` either in YAML or via
   `crankfire session edit <id> --add-tag prod --add-tag smoke`.
2. Type `/prod smoke,regression` in the TUI sessions list and see only
   sessions matching the AND-of-ORs filter; the same filter on the sets list
   shows only sets whose items reference matching sessions.
3. Author a template `templates/api-baseline.yaml` using `{{ .Rate }}` and
   `{{ .Env }}`, then run
   `crankfire set new --from-template api-baseline --param Rate=500 --param Env=prod --name api-baseline-prod`
   to materialize it as a real Set, edit/run it like any other Set.
4. Set `schedule: "*/5 * * * *"` on a Set, start `crankfire daemon`, walk
   away, come back five minutes later, and find a `SetRun` in the history
   that fired automatically.
5. Open the sets-history screen, mark two runs with `space`, press `d`, and
   see a side-by-side delta table; same data via
   `crankfire set diff <id-a> <id-b> --html out.html`.
6. Open `docs/tracing-backends.md`, copy the Tempo or Jaeger config
   verbatim, restart Crankfire, and see traces flowing.

## 13. Implementation outline (sketch — full plan in writing-plans phase)

1. `internal/tagfilter/` — parser + matcher + tests.
2. `store.Session.Tags` field + `SaveSession` dedupe/validate.
3. CLI: `session list --tag`, `session edit --add-tag/--remove-tag`.
4. TUI: shared filter widget + sessions-list integration.
5. TUI: sets-list transitive filter integration.
6. `internal/template/` package — render + tests.
7. `store.Template*` methods + on-disk format.
8. CLI: `set new --from-template --param ...`.
9. TUI: template picker + freeform param form on sets-list `t`.
10. `Set.Schedule` field + validator.
11. `internal/scheduler/` package — `Manager` + tests (with a fake clock).
12. CLI: `crankfire daemon` + flock + signal handling + integration test.
13. `internal/setrunner/diff.go` — `Diff` + tests.
14. CLI: `crankfire set diff` (text/json/html).
15. TUI: multi-select on history + diff screen.
16. `docs/tracing-backends.md` + README update.
17. End-to-end integration test: tagged sessions, scheduled set, daemon
    fires it, diff against a prior run.
18. `docs/sets.md` updates (tags, templates, schedule, diff).
