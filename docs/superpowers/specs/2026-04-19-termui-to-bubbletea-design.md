# Termui → Bubbletea Migration (Live Dashboard Consolidation)

**Date:** 2026-04-19
**Status:** Design approved, pending implementation

## Problem

Crankfire ships two live terminal UIs that overlap in purpose:

- `internal/dashboard/` — the legacy `--dashboard` view shown during a CLI
  load test, built on `github.com/gizak/termui/v3`.
- `internal/tui/runview/` — a bubbletea component used by the new interactive
  TUI's Run screen (`internal/tui/screens/run.go`).

Maintaining both means two rendering stacks, two dependency trees, and two
places to evolve when metrics change. The goal is to remove `termui` entirely
and route `--dashboard` through `runview`, giving us one live-view component
shared between the CLI and TUI.

## Goals

- Delete `internal/dashboard/` and the `github.com/gizak/termui/v3` dependency.
- Preserve the `--dashboard` CLI behavior: same flag, same shutdown wiring,
  near-equivalent textual output.
- Extend `runview` (additively) so it can render everything the termui
  dashboard does today: header, latency stats, endpoints, load-pattern
  progress, protocol metrics, failing status codes.
- Keep `runview`'s existing API backward-compatible so the TUI Run screen
  continues to work without changes (and can opt into richer data).

## Non-goals

- Visual restyling beyond what is needed to render the same data in
  bubbletea.
- Changing the `--dashboard` flag semantics, output cadence, or shutdown
  contract.
- Refactoring the metrics collector or report generators.

## Architecture

```
                   ┌──────────────────────────┐
                   │ internal/tui/runview     │  pure component
                   │  • Model / Update / View │  (bubbletea Model)
                   │  • Header, sparkline,    │
                   │    latency, endpoints,   │
                   │    load pattern, status, │
                   │    protocol metrics      │
                   └─────────────▲────────────┘
                                 │ SnapshotMsg
              ┌──────────────────┴──────────────────┐
              │                                     │
   ┌──────────┴──────────┐               ┌──────────┴────────────┐
   │ internal/cli/livedash│ NEW           │ internal/tui/screens   │
   │  • tea.Program       │               │   /run.go (existing)   │
   │  • snapshot ticker   │               │                        │
   │  • q/ctrl+c → cancel │               │                        │
   │  • WindowSizeMsg     │               │                        │
   │  Used by             │               │ Used by interactive    │
   │  internal/cli/run.go │               │ TUI                    │
   └──────────────────────┘               └────────────────────────┘
```

- **`internal/tui/runview/`** is the shared view component.
- **`internal/cli/livedash/`** is a new thin driver that wraps `runview.Model`
  in a `tea.Program` for use by the CLI.
- **`internal/dashboard/`** is deleted. `internal/cli/run.go` calls into
  `livedash` instead.

## Component design

### `runview.Options` (additive)

```go
type Options struct {
    Title       string         // existing
    Total       int64          // existing — overall request budget
    Header      []string       // NEW: lines rendered above progress bar
    LoadPattern *LoadPattern   // NEW: optional pattern descriptor
}

type LoadPattern struct {
    Name  string
    Total time.Duration
    Steps []PatternStep // label, duration, start offset
}
```

`Header` and `LoadPattern` are static-ish (set once when constructing the
view); per-tick numbers come through `SnapshotMsg`.

### `runview.SnapshotMsg` (additive, backward-compatible)

```go
type SnapshotMsg struct {
    Snap            metrics.DataPoint                    // existing
    Endpoints       []widgets.EndpointRow                // existing
    Stats           *metrics.Stats                       // NEW: optional richer stats
    StatusBuckets   map[string]map[string]int            // NEW
    ProtocolMetrics map[string]map[string]interface{}    // NEW
    Elapsed         time.Duration                        // NEW
}
```

The existing TUI Run screen will keep sending the small message and continue
to work; the CLI driver populates the new fields for full feature parity.

### `runview.View()` layout

Sections render top-to-bottom, each omitted if it has no data:

1. Title.
2. Header lines (target URL + test config + progress %).
3. Progress bar (existing).
4. Throughput sparkline + current RPS (existing).
5. Latency table — min, mean, p50, p90, p95, p99 (extends existing
   percentile table).
6. Errors / Total counts (existing).
7. **Either** load-pattern strip (current step highlighted, `►`/`✓`/`·`
   markers) **or** protocol metrics — same screen slot as termui dashboard.
8. Endpoint table — top 10, with method/path/share/RPS/p99/errors columns
   (extends existing `EndpointTable`).
9. Failing status codes — top 10 rows.
10. Footer key hints.

The view reacts to `tea.WindowSizeMsg` so widths adapt; behavior on tiny
terminals is best-effort (truncate, do not crash).

### `internal/cli/livedash/`

```go
type Opts struct {
    Title       string
    Header      []string
    Total       int64
    LoadPattern *runview.LoadPattern
    Interval    time.Duration // default 500ms
}

type Driver struct { /* tea.Program, model, collector, opts, wg */ }

func New(c *metrics.Collector, opts Opts, shutdown func()) *Driver
func (d *Driver) Start() error            // launches tea.Program (alt screen)
func (d *Driver) Stop() metrics.Stats     // signals quit, waits, returns final stats
```

Internal model:

- On each tick: `collector.Snapshot()`; build `runview.SnapshotMsg` from
  `collector.Stats(elapsed)` (full stats + endpoints + status buckets +
  protocol metrics) and the latest `DataPoint`; forward to embedded
  `runview.Model`.
- `q`, `esc`, `ctrl+c` → call `shutdown()` once, then `tea.Quit`.
- `tea.WindowSizeMsg` → forward to `runview.Model`.

`Stop()` records final stats by calling `collector.Stats(time.Since(start))`
once the program exits, mirroring the current `dashboard.GetFinalStats`
contract.

### `internal/cli/run.go` rewiring

- Drop `internal/dashboard` import.
- Replace the `if cfg.Dashboard { … dashboard.New … }` block with a
  `livedash.New(...)` + `Start()` + deferred `Stop()`.
- Move `buildDashPatternSteps` into `livedash` (renamed
  `buildLoadPattern`) and have it emit `*runview.LoadPattern`.
- Header lines built from the same fields currently passed into
  `dashboard.TestConfig`.

## Data flow

```
collector ──Snapshot()──┐
                        ▼
        livedash tick ──build SnapshotMsg──► runview.Model.Update
                                                     │
                                                     ▼
                                             runview.View()
```

CLI `Stop()` path:

```
SIGINT / q → shutdown() → cancels run ctx → run loop exits
                       → livedash program receives Quit → goroutine returns
                       → collector.Stats(elapsed) returned to caller
```

## Error handling

- `livedash.New` returns an error only if the bubbletea program cannot be
  constructed (rare — invalid renderer config).
- Snapshot ticks that race with collector teardown silently no-op; the
  collector already tolerates concurrent reads.
- Terminal resize during tear-down is ignored.
- If `livedash.Start` fails, callers fall back to the non-dashboard path
  (`internal/cli/run.go` already prints metrics via `output.PrintReport` on
  exit).

## Testing

- Extend `internal/tui/runview/runview_test.go`:
  - Header lines render verbatim above the progress bar.
  - Load-pattern strip highlights the active step and shows `✓` for past
    steps, `·` for future.
  - Status-bucket table renders top N failing codes.
  - Protocol metrics render when no load pattern is configured.
  - Snapshot with `Stats` populated picks up min/mean/p90.
- New `internal/cli/livedash/livedash_test.go`:
  - Program ticks produce a `SnapshotMsg` with the expected `Stats`,
    `Endpoints`, `StatusBuckets`, `ProtocolMetrics`, `Elapsed`.
  - `q` invokes the shutdown callback exactly once.
  - `Stop()` returns final stats matching `collector.Stats`.
- Existing TUI run-screen tests and integration tests must stay green.

## Cleanup

- Remove `internal/dashboard/` and its tests.
- `go mod tidy` to drop `gizak/termui/v3` (and any termui-only transitives).
- Update `docs/architecture/02-architecture-overview.md` reference.
- Update `.github/copilot-instructions.md` (architecture summary line that
  mentions `internal/dashboard/`).

## Risks / open questions

- **Snapshot frequency:** termui dashboard ticks at 500ms; the TUI run
  screen ticks at 250ms. We default `livedash.Interval` to 500ms to
  preserve current CLI cadence.
- **Visual parity:** bubbletea text rendering is line-based; the termui
  grid layout's exact pixel proportions cannot be reproduced. We aim for
  *informational* parity (every datum the termui view shows is present),
  not pixel parity.
- **Backward compatibility:** existing scripts that screenshot the dashboard
  may notice cosmetic changes. Acceptable per scope.
