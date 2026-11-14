---
layout: default
title: "Architecture: Workflows"
---
{% raw %}

[Docs home](../index.md) · [Overview](1-project-overview.md) · [Architecture](2-architecture-overview.md) · **Workflows**

# Workflow Overview

## 1. One-shot CLI run

`cli.Run` (`internal/cli/run.go`) handles a plain `crankfire [flags]` invocation.

```mermaid
sequenceDiagram
  participant U as User
  participant Run as cli.Run
  participant L as config.Loader
  participant B as cli.BuildRunner
  participant R as runner.Runner
  participant C as metrics.Collector
  participant O as output
  U->>Run: args
  Run->>L: Load(args) then Validate()
  Run->>Run: loadHAREndpoints, threshold.ParseMultiple
  Run->>B: BuildRunner(ctx, cfg)
  B-->>Run: runner, collector, cleanup
  Run->>Run: start dashboard OR progress ticker
  Run->>C: Start()
  Run->>Run: start HTML snapshot ticker (if --html-output)
  Run->>R: Run(signal ctx)
  R-->>Run: Result{Total, Errors, Duration}
  Run->>C: Stats(), final Snapshot()
  Run->>O: PrintReport / PrintJSONReport, GenerateHTMLReport
  Run-->>U: nil, or error (exit 1)
```

Steps:

1. `config.NewLoader().Load(args)` builds the config in three layers: defaults, then the config file, then flags that were explicitly set. With no arguments it prints help.
2. `cfg.Validate()` collects every validation issue and prints warnings to stderr, for example rate above 1000 or concurrency above 500.
3. `loadHAREndpoints` converts `--har` into `cfg.Endpoints`, then clears `HARFile` so a later call does not import it twice.
4. Thresholds are parsed before the run, so a typo fails fast.
5. `BuildRunner` sets up tracing, auth, the feeder, the collector, the requester chain and `runner.New`. It returns an idempotent `cleanup`. `BuildRunner` does not call `Validate()`. Only this one-shot path validates; the TUI, `set run` and the daemon call `BuildRunner` directly with unvalidated session configs.
6. `signal.NotifyContext` cancels the run on SIGINT or SIGTERM. In dashboard mode, `q`, `esc` and `ctrl+c` call the same cancel function. Cancelling stops in-flight requests at once (no grace window), and they are recorded as failures, so an interrupted run normally exits 1.
7. `--dashboard` starts `livedash`. If neither JSON nor dashboard output is requested, a 1 s `ProgressReporter` writes to stdout.
8. `collector.Start()` stamps the start time. Then, with `--html-output`, a goroutine calls `collector.Snapshot()` every second to build the time series.
9. After `Run` returns, the dashboard stops before the report is printed.
10. Thresholds are evaluated against `collector.Stats`.
11. The text or JSON report goes to stdout. The path of the HTML report goes to stderr.
12. The return value is an error if any threshold failed, otherwise an error if `result.Errors > 0`, otherwise `nil`.

## 2. HTTP request lifecycle

```mermaid
sequenceDiagram
  participant W as Worker
  participant S as endpointSelector
  participant RT as retryRequester
  participant LG as loggingRequester
  participant H as httpRequester
  participant RB as RequestBuilder
  participant X as extractor
  participant C as metrics.Collector
  W->>S: Do(workerCtx)
  S->>S: pick weighted template, store in ctx
  S->>RT: Do(ctx)
  loop up to retries+1 attempts
    RT->>LG: Do(ctx)
    LG->>H: Do(ctx)
    H->>H: start span
    H->>RB: Build(ctx) - feeder.Next, placeholders, auth header
    H->>H: inject trace headers, client.Do, drain body
    H->>X: ExtractAll (status < 400, or on_error)
    X-->>H: vars into per-worker store
    H->>C: RecordRequest (every attempt)
    H-->>LG: nil or HTTPError
  end
  RT-->>W: final error
```

The outer layers are optional:

- The selector exists only when `cfg.Endpoints` is not empty (including HAR imports).
- The retry layer exists only with `retries > 0`.
- The logging layer exists only with `--log-errors`.

Without them, the worker calls `httpRequester` directly.

`httpRequester` records every attempt in the collector, but `Result` counts one entry per scheduled request. With retries enabled the two counts measure different things. An attempt that fails and then succeeds on retry still adds to the collector's `failures` and to thresholds, but not to `Result.Errors`.

A request is retried after HTTP 429, a 5xx, or a network or build error. These are not retried:

- context cancellation;
- request timeouts: `net/http` timeout errors match `context.DeadlineExceeded`, which the policy refuses (`internal/cli/run.go:294`);
- other 4xx responses.

The backoff is `100ms·2^(n-1)`, capped at 5 s, plus up to 50% jitter.

## 3. Runner scheduling and shutdown

```mermaid
stateDiagram-v2
  [*] --> Scheduling
  Scheduling --> Scheduling: arrival.Wait then permit sent
  Scheduling --> Draining: total reached, duration elapsed, pattern ended, or Wait error
  Scheduling --> Cancelled: parent ctx cancelled
  Draining --> Done: workers finish within GracefulShutdown
  Draining --> Done: grace timer fires, runCtx cancelled
  Draining --> Done: negative grace, runCtx cancelled at once
  Draining --> Cancelled: parent ctx cancelled
  Cancelled --> Done: workers return
  Done --> [*]
```

`GracefulShutdown` defaults to 5 s. A negative value cancels in-flight requests immediately. The grace window applies only when the run ends on its own. A parent cancel (Ctrl+C, `q` in the dashboard) cancels `runCtx` at once, so in-flight requests fail and are recorded as errors. See [Runner Engine](deep-dive/runner-engine.md).

## 4. Set run

`setrunner.Runner` runs a set (`internal/setrunner/runner.go`). It has two entry points:

- **`Run(ctx, setID, events)`** loads the set itself. The TUI uses it (with a 64-slot events channel), and so does the daemon (with nil events).
- **`RunSet(ctx, set, events)`** takes a set that is already in memory. `crankfire set run` uses it: it calls `GetSet`, adds the `--threshold` and `--override` flags, then calls `RunSet(ctx, set, nil)`.

```mermaid
sequenceDiagram
  participant CLI as set run CLI
  participant TD as TUI / daemon
  participant SR as setrunner.Runner
  participant ST as store
  participant SB as cli SetBuilder
  CLI->>ST: GetSet, apply flag overrides
  CLI->>SR: RunSet(ctx, set, nil)
  TD->>SR: Run(ctx, setID, events)
  SR->>ST: GetSet (Run only), CreateSetRun
  loop each stage, sequential
    par each item, parallel
      SR->>ST: GetSession
      SR->>SR: ApplyOverrides
      SR->>SB: Build(cfg) then Run(ctx)
      SB-->>SR: RunSummary
    end
    SR->>SR: on_failure abort or continue
  end
  SR->>SR: aggregate, EvaluateThresholds
  SR->>ST: FinalizeSetRun - set-run.json
  SR-->>Caller: SetRun
```

A failed threshold turns a `completed` set run into `failed`. Items of a stage run in their own goroutines. Events are sent to the caller's channel without blocking, so an event is dropped if the buffer is full.

`main.go` gives `crankfire set` and `crankfire session` a plain `context.Background()`, with no signal handling. Ctrl+C during `set run` kills the process: `set-run.json` stays `running` and cleanup does not run.

## 5. Daemon scheduling

```mermaid
sequenceDiagram
  participant D as cli.RunDaemon
  participant M as scheduler.Manager
  participant ST as store
  participant SR as setrunner.Runner
  D->>D: flock daemon.lock or exit 1
  D->>M: Reload(ctx, store)
  M->>ST: ListSets - collect schedule fields
  D->>M: Run(ctx)
  Note over D,M: SIGHUP triggers Reload again
  M->>M: cron fires for setID
  alt set already running in this process
    M->>M: skip fire
  else
    M->>SR: Run(ctx, setID)
  end
  Note over D,M: SIGINT or SIGTERM cancels running set runs, then waits for them
```

Scheduled runs use the daemon's root context, so a shutdown signal cancels in-flight set runs first. `cron.Stop()` then waits for them with no time limit. The daemon's own 30 s drain timeout (`daemonDrainTimeout`) is reached only after that wait, so it never takes effect.

Logs are JSON lines written to stdout, with the fields `ts`, `level`, `event`, `setID` and `msg`.

## 6. TUI single-session run

```mermaid
sequenceDiagram
  participant Scr as screens.Run
  participant ST as store
  participant B as cli.BuildRunner
  participant R as runner.Runner
  participant RV as runview.Model
  Scr->>ST: CreateRun - run.json running
  Scr->>B: BuildRunner(runCtx, sess.Config)
  Scr->>R: go Run(runCtx)
  loop every 250ms tick
    Scr->>RV: SnapshotMsg (Snap only)
    Scr->>Scr: poll finishPending registry
  end
  R-->>Scr: run goroutine registers finish
  Scr->>ST: write result.json, report.html, FinalizeRun
  Scr-->>Scr: PopMsg back to previous screen
```

## Data Flow

```mermaid
flowchart LR
  A["flags + config file"] --> B["config.Config"]
  H["HAR file"] --> B
  F["CSV / JSON feeder"] --> Q["request build"]
  V["per-worker variables"] --> Q
  B --> Q
  Q --> T["target system"]
  T --> E["extractors"]
  E --> V
  T --> M["metrics.Collector"]
  M --> R["text / JSON / HTML / dashboard"]
  M --> TH["thresholds -> exit status"]
```

## State Management

| State | Lives in | Scope / lifetime |
|-------|----------|------------------|
| Run configuration | `config.Config`, passed by value | One run |
| Pacing | `arrivalController`, `patternPlan` | Runner |
| Request counters | atomics in `runner.Run` | Runner |
| Latency, outcomes | `metrics.Collector` (32 shards) and `history` slice (unbounded) | Run |
| Chained variables | `variables.Store` in worker context | One worker goroutine |
| Feeder cursor | `feeder.Feeder` behind a mutex, shared | Run |
| OAuth token | `auth.OAuth2Provider` cache | Run |
| Connections | `pool.ConnectionPool` (WS/SSE), `sync.Map` (gRPC), `http.Transport` | Run |
| Sessions, sets, templates | `<data>/sessions`, `sets`, `templates` YAML | Persistent |
| Run history | `<data>/runs/<session>/<ts>/`, `<data>/runs/sets/<set>/<ts>/` | Persistent |
| TUI navigation | `tui.Root.stack` | Process |

## Error Handling and Exit Codes

**Default command (`cli.Run`)**

- Any error returned by `Run` makes the process print `error: ...` and **exit 1**.
- That includes parse and validation errors, failed thresholds, and any failed request (`result.Errors > 0`), even when every threshold passed.
- `--help` exits 0.
- The typed constants `ExitThresholdFailed=2` and `ExitRunnerError=3` are **not** used here.

**`crankfire set run`** (constants in `internal/cli/set.go`)

| Code | Meaning |
|------|---------|
| 0 | Every threshold passed and the set run completed |
| 1 | Usage, parse or lookup error |
| 2 | A threshold failed (checked first) |
| 3 | Runner error, or the run finished with a status other than completed |

**Other subcommands**

- `session` exits 1 for usage errors and for failures to get or save a session. It exits 3 only when listing sessions fails or the store cannot be opened (in `main.go`).
- `daemon` exits 1 if another daemon holds the lock, 3 on store or initial reload failure, and 0 after a clean shutdown.

**Per-request errors**

- They never abort a run. They are counted, put into status buckets (failures only), optionally logged with `--log-errors` (HTTP only), and reflected in thresholds.
- Error classification is described in [Protocol Clients](deep-dive/protocol-clients.md#status-bucket-taxonomy).
{% endraw %}
