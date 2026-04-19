# Crankfire TUI — Phase 1 Design

**Status:** Draft
**Date:** 2026-04-19
**Scope:** Phase 1 of a multi-phase TUI initiative. Phase 1 covers session storage, the
`crankfire tui` shell, session CRUD + import, and the live single-run view. Suites,
cross-session history, run comparison, and scheduling are explicitly deferred.

## Goals

- Let users save, edit, and re-run named load-test configurations from a terminal UI.
- Persist every run's results and HTML report so users can revisit them later.
- Render a live, in-process run view that matches the information density of today's
  CLI dashboard, without forcing users to re-learn the tool.
- Stay additive: every existing CLI flag and behavior keeps working unchanged.

## Non-Goals (Phase 1)

- Suites / sets of sessions and any cross-session orchestration.
- Cross-session history browser, run comparison, scheduling.
- Editing CSV/JSON feeder data files inside the TUI.
- HAR import inside the TUI (the CLI path continues to work).
- Migrating sessions across schema versions (a `schema_version` field is reserved
  now; migration logic ships with Phase 2 if needed).

## High-Level Architecture

A new `crankfire tui` subcommand launches a Bubble Tea program that talks to a
filesystem-backed store and, when running a session, drives the existing
`internal/runner` in-process. The live view is implemented natively in Bubble Tea +
Lipgloss (porting today's `gizak/termui` rendering); the legacy CLI flow keeps using
`internal/dashboard` unchanged.

```
cmd/crankfire/main.go
        │
        ├── existing flag/CLI flow ────────────────► internal/runner (unchanged)
        │
        └── tui subcommand ──► internal/tui ──► internal/store
                                       │
                                       └────► internal/runner (in-process)
                                       └────► internal/tui/runview (live view)
```

**Layering rule:** The TUI depends on `runner` and `store`; nothing else depends on
the TUI. The store has no dependency on the TUI.

## New Packages

| Package | Purpose |
|---|---|
| `internal/store` | Filesystem store for sessions + runs. |
| `internal/tui` | Bubble Tea root model, screen router, key bindings, styles. |
| `internal/tui/screens` | One file per screen, each kept small (target <300 LOC). |
| `internal/tui/runview` | Bubble Tea live run dashboard. |
| `internal/tui/widgets` | Reusable Lipgloss-rendered widgets (progress bar, sparkline, percentile table, endpoint table). |

## CLI Surface

- `crankfire tui` — launch the TUI.
- `--data-dir <path>` and env `CRANKFIRE_DATA_DIR` — override the default data
  directory (`~/.crankfire/`, or `$XDG_DATA_HOME/crankfire/` when set).
- All existing flags continue to work unchanged.

## Storage Layer (`internal/store`)

### On-disk layout

```
<data-dir>/
├── sessions/
│   ├── <session-id>.yaml
│   └── ...
└── runs/
    └── <session-id>/
        └── <RFC3339-timestamp>/
            ├── run.json       # run metadata + summary stats
            ├── result.json    # full metrics snapshot (existing JSON output format)
            └── report.html    # rendered HTML report (existing renderer)
```

### Session YAML schema

The session wraps the existing crankfire config under a `config:` key plus a small
metadata header. This makes "import an existing config file" trivial.

```yaml
schema_version: 1
id: 01HZX...                # ULID, stable across renames
name: "Checkout API smoke"
description: "Quick smoke against staging"
created_at: 2026-04-19T10:00:00Z
updated_at: 2026-04-19T10:00:00Z
config:
  # exact existing crankfire YAML config schema, unchanged
  target: https://...
  total: 1000
  ...
```

### Public API

```go
type Store interface {
    // Sessions
    ListSessions(ctx context.Context) ([]Session, error)
    GetSession(ctx context.Context, id string) (Session, error)
    SaveSession(ctx context.Context, s Session) error
    DeleteSession(ctx context.Context, id string) error
    ImportSessionFromConfigFile(ctx context.Context, path, name string) (Session, error)

    // Runs
    ListRuns(ctx context.Context, sessionID string) ([]Run, error) // newest first
    CreateRun(ctx context.Context, sessionID string) (Run, error)  // allocates dir
    FinalizeRun(ctx context.Context, run Run, summary RunSummary) error
}
```

`Session.Config` reuses the existing `internal/config.Config` type so the schema is
never duplicated.

### Concurrency & safety

- Atomic writes: write to `*.tmp` then `os.Rename`.
- One `flock` per session file during write to prevent concurrent corruption from
  two TUI instances. Reads do not lock.
- IDs are ULIDs (sortable, no central counter).

### Errors

`store.ErrNotFound`, `store.ErrAlreadyExists`, `store.ErrInvalidConfig`, all wrapped
with `fmt.Errorf("...: %w", err)` per repo convention.

## TUI Shell (`internal/tui`)

### Framework

`charmbracelet/bubbletea` + `bubbles` (list, textinput, textarea, viewport, spinner,
help) + `lipgloss` (styling). MIT-licensed, mature, single-binary friendly, no cgo.

### Top-level model

A screen-router. Exactly one screen is active at a time; screens push/pop on a stack
so `Esc` always navigates back. Each screen is its own `tea.Model` in its own file.

### Screens

| Screen | File | Purpose |
|---|---|---|
| Session List | `screens/list.go` | Browse sessions; keys: `n` new, `e` edit, `d` delete, `i` import, `r` run, `h` history, `Enter` view detail, `q` quit. |
| Session Detail | `screens/detail.go` | Show metadata + config preview + recent runs; `r` run, `e` edit. |
| Session Edit | `screens/edit.go` | Guided form (target, protocol, total, rate, duration, concurrency, timeout, headers) for common fields; `Tab` toggles into raw YAML mode for full schema access. |
| Import | `screens/import.go` | File picker (filepath input + glob suggestions) → `store.ImportSessionFromConfigFile` → drops user into Edit. |
| Confirm Delete | `screens/confirm.go` | Generic yes/no modal. |
| Run | `screens/run.go` | Wraps `runview` for an active run. |
| Run History | `screens/history.go` | List past runs for a session with status + key stats; inline summary panel (totals, P50/P90/P95/P99, error rate, duration); `o` opens `report.html` via `xdg-open`/`open`/`start`. |

### Editing model (form ↔ YAML)

The Edit screen ships with a guided form for the most common fields. `Tab` toggles
into a raw YAML textarea for full schema access.

- Form → YAML: form values are merged into the underlying YAML on toggle.
- YAML → form: on toggle back, the form re-derives from the parsed YAML.
- If the YAML cannot be re-parsed back into the form schema, the toggle is disabled
  with a hint explaining why and pointing to the offending line.
- Validation runs on every keystroke (debounced 200ms) using `internal/config.Load` —
  same rules as the CLI. Save (`Ctrl+S`) is blocked on invalid configs.

### Global UX

- Help bar (`bubbles/help`) at the bottom shows context-specific keys.
- Header shows current data-dir and session count.
- Mouse support enabled (Bubble Tea default), but everything works keyboard-only.
- Honors `NO_COLOR` and adapts to dark/light backgrounds via Lipgloss.
- Layout renders correctly down to 80×24.

### Wiring

```go
// cmd/crankfire/main.go
func run(args []string) error {
    if len(args) >= 1 && args[0] == "tui" {
        return tui.Run(tui.Options{
            DataDir: resolveDataDir(args[1:]), // --data-dir flag
        })
    }
    // ...existing flow unchanged
}
```

`tui.Run` builds the store, builds the root model, and calls
`tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()`.

## Live Run View (`internal/tui/runview`)

### Lifecycle

1. User triggers a run from Session List/Detail. The TUI calls `store.CreateRun` to
   allocate the run directory.
2. runview builds the same dependency graph as today's CLI: auth provider → feeder →
   requester (via `requester_factory`) → metrics aggregator → runner — using the
   session's saved `config`.
3. The runner executes in a goroutine. A 250ms ticker emits `tea.Msg` snapshots of
   `metrics.Aggregator` into the Bubble Tea program (same snapshot type the existing
   dashboard already consumes).
4. On completion (or `Esc`/`Ctrl+C` cancel via `context.Cancel`), runview calls
   `output.WriteJSON` and `output.WriteHTMLReport` against the run directory, then
   `store.FinalizeRun` with the summary, and returns to Session Detail.

### Layout

```
┌─ Crankfire — running "Checkout API smoke" ────── 00:01:42 / 00:05:00 ┐
│ Target: https://api.example.com/checkout   Protocol: HTTP            │
│ Workers: 50   Rate: 500 rps   Pattern: ramp 0→500 (60s) → flat       │
├──────────────────────────── Progress ────────────────────────────────┤
│ [████████████████░░░░░░░░░░░░░░░░] 34%   Reqs: 17,250 / 50,000       │
│                                                                      │
├─── Throughput ────────────┬─── Latency (ms) ────────────┬─ Status ───┤
│ ▁▂▃▅▆▇█▇▆▅▆▇█▇▆▅▆▇█       │ p50:   42                   │ 2xx 16,801 │
│ now:    487 rps           │ p90:   88                   │ 4xx     12 │
│ avg:    472 rps           │ p95:  121                   │ 5xx      7 │
│ peak:   501 rps           │ p99:  204                   │ err     430│
├──────────────────────── Top Endpoints ───────────────────────────────┤
│  POST /checkout    9,841   p95 130ms   err 1.1%                      │
│  GET  /cart        7,409   p95  92ms   err 0.0%                      │
├──── [p] pause snapshots  [c] cancel run  [r] open report (when done)─┘
```

### Implementation notes

- Reuses `internal/metrics` snapshot data structures verbatim — no metric
  duplication, no parallel codepaths.
- A small `widgets/` subpackage holds reusable Lipgloss-rendered pieces (progress
  bar, sparkline, percentile table, endpoint table).
- runview keeps a 120-sample rolling ring buffer of throughput snapshots in memory
  and renders a Lipgloss block-glyph sparkline above the throughput numbers. No
  changes to `internal/metrics`.
- Cancel sends `cancel()` on the runner context; the run is finalized with
  `status: "cancelled"` and partial metrics are still written.
- If the run errors (e.g., invalid target), runview shows the error in the status
  panel and finalizes with `status: "failed"`.
- `internal/dashboard` (termui) is **not** touched; CLI users keep the same
  experience.

## Error Handling

- **Storage errors** surface in a Lipgloss-styled status bar at the bottom of the
  current screen; non-fatal failures keep the user in place. Fatal errors during
  `tui.Run` startup print to stderr and exit non-zero.
- **Config validation** in the Edit screen reuses `internal/config.Load`/`Validate`.
- **Runner errors** during a run are rendered inline in runview and the run is
  finalized with `status: "failed"`; the partial `result.json` is still written.
- **Panics** in screens are recovered at the top-level `tea.Program` and reported as
  a fatal error screen; a stack trace is dumped to `<data-dir>/crash.log`.
- All errors wrap with `fmt.Errorf("...: %w", err)`. Sentinel errors
  (`store.ErrNotFound`, etc.) are exported for tests.

## Testing

### Unit tests (`go test -race ./...`)

- Table-driven, slice-based, `t.Run` subtests, `got`/`want`, stdlib assertions only
  (per repo convention).
- `internal/store`: full CRUD against `t.TempDir()`, atomic-write verification,
  lock-contention test, ULID monotonicity, import round-trip.
- `internal/tui/screens/*`: send `tea.KeyMsg` and custom messages to `Update`,
  assert state transitions and `View()` substrings; fakes (e.g., `fakeStore`) defined
  in test files per repo pattern.
- `internal/tui/runview`: feed synthetic snapshot messages, assert rendered
  substrings and finalize-on-completion behavior. Run execution tested with a
  `fakeRequester` (existing pattern from `internal/runner` tests).

### Integration tests (`-tags=integration`) in `cmd/crankfire/`

- One end-to-end test: launch `crankfire tui` against a `t.TempDir()` data dir using
  `vt10x`, create a session, run it against a local `httptest` server, assert a
  `runs/<id>/result.json` is written. Gated like all existing integration tests.

### Coverage

Maintain current package coverage on the new packages (no regression vs. existing
baseline).

## New Dependencies

| Module | License | Notes |
|---|---|---|
| `github.com/charmbracelet/bubbletea` | MIT | TUI runtime. |
| `github.com/charmbracelet/bubbles` | MIT | Reusable widgets. |
| `github.com/charmbracelet/lipgloss` | MIT | Styling. |
| `github.com/oklog/ulid/v2` | Apache-2.0 | Session/run IDs. |
| `github.com/gofrs/flock` | BSD-3 | Per-session-file write locks. |
| `github.com/hinshun/vt10x` | MIT | Test-only, behind `integration` tag. |

All single-binary friendly, no cgo. README's "single binary, minimal runtime
dependencies" promise stays intact.

## Out of Scope (deferred)

- Suite / set creation, ordering, sequential orchestration, suite-level reports.
- Cross-session history browser, run comparison, scheduling.
- Editing CSV/JSON feeder data files inside the TUI.
- HAR import inside the TUI (CLI path continues to work).
- Schema migrations across session versions (`schema_version` is reserved now;
  migration ships with Phase 2 if needed).

## Open Risks

- **Bubble Tea / metrics snapshot ergonomics:** the existing dashboard pulls
  snapshots synchronously; the TUI must marshal them as `tea.Msg`. Risk is small —
  the snapshot type is already a value type — but worth verifying early in
  implementation.
- **Form ↔ YAML round-trip fidelity:** users editing YAML in ways the form can't
  represent (e.g., feeders, complex auth) must not lose data on toggle. Mitigation:
  toggle is disabled with a clear hint when the YAML cannot be losslessly rendered
  back into the form.
