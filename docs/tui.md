# Terminal UI Guide

The Crankfire Terminal UI (`crankfire tui`) provides an interactive dashboard for managing load-test sessions, editing configurations, running tests, and viewing historical results.

## Overview

The TUI is organized into five main screens:

1. **Session List** — Browse, create, edit, delete, and run saved sessions
2. **Edit Screen** — Create or modify a session configuration (form or YAML mode)
3. **Run Screen** — Real-time view of an active load test
4. **History Screen** — View past runs for a session and open reports
5. **Details Screen** — View session or run details

## Data Directory

Sessions and results are stored in `~/.crankfire/` by default. You can override this with:

```bash
crankfire tui --data-dir /custom/path
```

Or by setting the `$CRANKFIRE_DATA_DIR` environment variable:

```bash
export CRANKFIRE_DATA_DIR=/data/crankfire
crankfire tui
```

Directory structure:

```
~/.crankfire/
├── sessions/
│   ├── my-api-test.json
│   ├── websocket-load.json
│   └── grpc-benchmark.json
└── runs/
    ├── my-api-test/
    │   ├── 2025-04-19T14:30:00Z/
    │   │   ├── result.json
    │   │   └── report.html
    │   └── 2025-04-19T14:35:00Z/
    │       ├── result.json
    │       └── report.html
    └── websocket-load/
        └── 2025-04-19T15:00:00Z/
            ├── result.json
            └── report.html
```

## Session List Screen

This is the main screen when you launch `crankfire tui`. It shows a list of all saved sessions.

### Key Bindings

| Key | Action |
|-----|--------|
| `n` | Create a new session |
| `e` | Edit the selected session |
| `d` | Delete the selected session |
| `i` | Import a session from a YAML/JSON file |
| `r` | Run the selected session immediately |
| `h` | View run history for the selected session |
| `Enter` | View full details of the selected session |
| `q` or `Esc` | Quit the TUI |

### Workflow Example

1. Press `n` to create a new session
2. Enter a session name (e.g., `api-baseline`)
3. You'll be taken to the **Edit Screen**

## Edit Screen

Use this screen to create or modify a session configuration. The configuration is the same YAML/JSON format as a Crankfire config file.

### Modes

- **Form Mode** (default): Interactive form with labeled fields (target, method, concurrency, etc.)
- **YAML Mode**: Edit raw YAML configuration

### Key Bindings

| Key | Action |
|-----|--------|
| `Tab` | Move to next field |
| `Shift+Tab` | Move to previous field |
| `F2` | Toggle between Form and YAML mode |
| `Ctrl+S` | Save the session |
| `Esc` | Cancel and return to Session List |

### Form Mode Fields

- **Target**: The endpoint URL (required)
- **Method**: HTTP method (GET, POST, PUT, DELETE, etc.; default: GET)
- **Protocol**: Protocol mode (http, websocket, sse, grpc; default: http)
- **Concurrency**: Number of parallel workers
- **Rate**: Requests per second (0 = unlimited)
- **Duration**: Test duration (e.g., 30s, 1m)
- **Total**: Total number of requests
- **Timeout**: Per-request timeout (default: 30s)
- **Body**: Request body (for POST/PUT/PATCH)
- **Headers**: Add custom headers
- **Load Patterns**: Configure ramp/step/spike patterns
- **Thresholds**: Set performance pass/fail criteria

### YAML Mode

Press `F2` to switch to YAML mode and edit the raw configuration. This is useful for:

- Copying and pasting complex configurations
- Setting advanced options not exposed in Form mode
- Editing multi-endpoint setups with weighted distributions
- Configuring data feeders and request chaining

Example YAML configuration:

```yaml
target: https://api.example.com
method: GET
concurrency: 20
rate: 100
duration: 1m
headers:
  Authorization: Bearer token123
  Content-Type: application/json
load_patterns:
  - name: ramp
    type: ramp
    from_rps: 10
    to_rps: 100
    duration: 30s
thresholds:
  - http_req_duration:p95 < 500
  - http_req_failed:rate < 0.01
```

Press `Ctrl+S` to save.

## Run Screen

When you press `r` to run a session, the TUI switches to the **Run Screen**, showing real-time progress and metrics.

### Display Elements

- **Test Header**: Session name, target URL, and elapsed time
- **Real-time Metrics Table**:
  - Total requests sent
  - Successful / failed request counts
  - Request rate (RPS)
  - Latency percentiles (min, p50, p90, p95, p99, max)
  - Status code buckets
- **Sparkline Graph** (optional): Visual latency trend over time
- **Status Messages**: Errors, warnings, and progress updates

### Key Bindings

| Key | Action |
|-----|--------|
| `c` or `Esc` | Cancel the running test |
| `p` | Pause snapshots (stop updating the display, but test continues) |
| `q` | Return to Session List after test completes |

### Example Run Output

```
─── Load Test: api-baseline ───────────────────────────────────
Target: https://api.example.com
Status: Running (25s elapsed)

Requests:     2,500 total | 2,487 success | 13 failed
Request Rate: 100.0 RPS
Latency:      12ms min | 45ms p50 | 68ms p95 | 112ms p99 | 156ms max

Status Buckets:
  HTTP 200: 2,487
  HTTP 503: 13

───────────────────────────────────────────────────────────────
p: pause | c: cancel | q: back
```

## History Screen

View all past runs for a selected session and open their HTML reports.

### Key Bindings

| Key | Action |
|-----|--------|
| `o` | Open the selected run's HTML report in your default browser |
| `Enter` | View run details |
| `Esc` or `q` | Back to Session List |

Each run shows:

- **Timestamp**: When the test was executed
- **Duration**: How long the test took
- **Total Requests**: Requests sent in this run
- **Status**: Success count and failure count

Runs are stored in `~/.crankfire/runs/<session-id>/<timestamp>/`, and each includes:

- `result.json` — Structured metrics (used for CI/CD integration)
- `report.html` — Interactive HTML report with charts

## Details Screen

View full details of a session or run. Press `Enter` on any session or run to view its complete configuration and results.

## Example Workflows

### Workflow 1: Create and Run a Quick HTTP Load Test

1. Launch `crankfire tui`
2. Press `n` to create a new session; name it `quick-test`
3. In Edit screen:
   - Set Target: `https://api.example.com`
   - Set Concurrency: `10`
   - Set Total: `100`
   - Press `Ctrl+S` to save
4. Press `r` to run
5. Watch the real-time metrics on the Run screen
6. After the test completes, press `q` to return to the session list

### Workflow 2: Edit an Existing Session

1. From the Session List, select a session and press `e`
2. In Edit screen, modify the configuration (e.g., increase concurrency or add headers)
3. Press `Ctrl+S` to save
4. Press `Esc` to return to the Session List

### Workflow 3: Switch to YAML Mode for Advanced Config

1. From the Session List, select a session and press `e`
2. Press `F2` to toggle to YAML mode
3. Edit the raw configuration, e.g., add multi-endpoint setups:

```yaml
endpoints:
  - name: list-users
    weight: 8
    method: GET
    path: /users
  - name: create-order
    weight: 2
    method: POST
    path: /orders
    body: '{"product_id": "123", "quantity": 1}'
```

4. Press `Ctrl+S` to save and return to Session List

### Workflow 4: View and Compare Historical Runs

1. From the Session List, select a session
2. Press `h` to view run history
3. Browse the list of past runs (sorted by timestamp, most recent first)
4. Press `o` to open the HTML report of a run in your browser for detailed analysis
5. Press `Esc` to return to the Session List

## Tips and Tricks

- **Keyboard Navigation**: Use `Tab` and `Shift+Tab` to move between fields in Form mode
- **Copy Configuration**: Export a session as YAML, then paste it into another tool or version control
- **Reuse Sessions**: Once you've configured a session, you can run it multiple times without reconfiguring
- **Archive Results**: All run results (JSON and HTML reports) are automatically saved to `~/.crankfire/runs/`
- **Compare Baselines**: Use the History screen to compare metrics across multiple runs of the same session

## Troubleshooting

### Session not saving
- Ensure the `~/.crankfire/` directory exists and is writable
- Check the `$CRANKFIRE_DATA_DIR` environment variable if you've set a custom path

### Cannot open HTML report
- Verify that the report file exists: `~/.crankfire/runs/<session-id>/<timestamp>/report.html`
- Ensure your default browser is configured correctly

### Run screen shows errors
- Check the target URL and ensure it's reachable
- Verify authentication credentials if the endpoint requires them
- Review thresholds for false negatives

## See Also

- [Crankfire README](../README.md)
- [Configuration & CLI Reference](https://torosent.github.io/crankfire/configuration.html)
- [Getting Started Guide](https://torosent.github.io/crankfire/getting-started.html)
