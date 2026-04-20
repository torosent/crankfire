# Termui → Bubbletea Live Dashboard Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete `internal/dashboard/` and the `github.com/gizak/termui/v3` dependency, routing the CLI `--dashboard` flag through the existing bubbletea `internal/tui/runview/` component via a new thin `internal/cli/livedash/` driver.

**Architecture:** Extend `runview.Options` and `runview.SnapshotMsg` additively (preserving backward compatibility for the TUI Run screen) so the view can render header lines, full latency table, status-bucket table, protocol-metrics table, and a load-pattern strip. Build `internal/cli/livedash/` as a small `tea.Program` driver that owns the snapshot ticker, key/resize handling, and shutdown wiring; rewire `internal/cli/run.go` to use it. Then remove the legacy package and run `go mod tidy`.

**Tech Stack:** Go 1.25, `github.com/charmbracelet/bubbletea` v1.3.10, `github.com/torosent/crankfire/internal/metrics`, `github.com/torosent/crankfire/internal/tui/widgets`.

---

## File Structure

**New files:**
- `internal/tui/runview/loadpattern.go` — `LoadPattern` / `PatternStep` types and step-strip rendering helper.
- `internal/tui/runview/loadpattern_test.go` — unit tests for the strip rendering.
- `internal/tui/widgets/statusbuckets.go` — flatten + format helper for status-bucket table.
- `internal/tui/widgets/statusbuckets_test.go` — unit tests.
- `internal/tui/widgets/protocolmetrics.go` — formatter for protocol metrics block.
- `internal/tui/widgets/protocolmetrics_test.go` — unit tests.
- `internal/cli/livedash/livedash.go` — `Driver`, `Opts`, internal bubbletea model, snapshot building.
- `internal/cli/livedash/livedash_test.go` — driver tests (snapshot construction, quit, final stats).

**Modified files:**
- `internal/tui/runview/runview.go` — extend `Options` and `SnapshotMsg`, expand `View()`.
- `internal/tui/runview/runview_test.go` — add coverage for new fields.
- `internal/tui/widgets/tables.go` — extend `EndpointRow` and `EndpointTable` to render share/RPS/p99/errors.
- `internal/tui/widgets/widgets_test.go` — coverage for new endpoint columns.
- `internal/cli/run.go` — drop `internal/dashboard` import; use `livedash` instead; move `buildDashPatternSteps` into `livedash`.
- `docs/architecture/02-architecture-overview.md` — remove the termui node from the dependency diagram, route Dashboard → bubbletea.
- `.github/copilot-instructions.md` — replace the architecture-summary line that mentions `internal/dashboard/`.
- `go.mod`, `go.sum` — `go mod tidy` after removing termui.

**Deleted files:**
- `internal/dashboard/dashboard.go`
- `internal/dashboard/dashboard_test.go` (entire directory removed)

---

## Task 1: Add `LoadPattern` types and rendering helper to `runview`

**Files:**
- Create: `internal/tui/runview/loadpattern.go`
- Create: `internal/tui/runview/loadpattern_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/runview/loadpattern_test.go`:

```go
package runview_test

import (
	"strings"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/tui/runview"
)

func TestRenderLoadPatternStripHighlightsCurrentStep(t *testing.T) {
	lp := &runview.LoadPattern{
		Name:  "ramp+steady",
		Total: 30 * time.Second,
		Steps: []runview.PatternStep{
			{Label: "10 RPS", Duration: 10 * time.Second, Start: 0},
			{Label: "10→100 RPS", Duration: 10 * time.Second, Start: 10 * time.Second},
			{Label: "100 RPS", Duration: 10 * time.Second, Start: 20 * time.Second},
		},
	}
	out := runview.RenderLoadPatternStrip(lp, 15*time.Second, 30)
	if !strings.Contains(out, "ramp+steady") {
		t.Errorf("missing pattern name in: %s", out)
	}
	if !strings.Contains(out, "✓ 10 RPS") {
		t.Errorf("expected first step marked done, got: %s", out)
	}
	if !strings.Contains(out, "► 10→100 RPS") {
		t.Errorf("expected current step marked active, got: %s", out)
	}
	if !strings.Contains(out, "· 100 RPS") {
		t.Errorf("expected future step marked pending, got: %s", out)
	}
	if !strings.Contains(out, "50%") {
		t.Errorf("expected progress 50%%, got: %s", out)
	}
}

func TestRenderLoadPatternStripNilReturnsEmpty(t *testing.T) {
	if got := runview.RenderLoadPatternStrip(nil, 0, 30); got != "" {
		t.Errorf("expected empty string for nil pattern, got %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/runview/ -run TestRenderLoadPatternStrip -count=1`
Expected: FAIL — `undefined: runview.LoadPattern` / `undefined: runview.RenderLoadPatternStrip`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/tui/runview/loadpattern.go`:

```go
package runview

import (
	"fmt"
	"strings"
	"time"

	"github.com/torosent/crankfire/internal/tui/widgets"
)

// PatternStep describes one compiled segment of a load pattern.
type PatternStep struct {
	Label    string
	Duration time.Duration
	Start    time.Duration
}

// LoadPattern is the read-only descriptor passed via runview.Options.
type LoadPattern struct {
	Name  string
	Total time.Duration
	Steps []PatternStep
}

// RenderLoadPatternStrip returns a multi-line description of a load pattern's
// progress: a progress bar with elapsed/total, followed by a step strip with
// the active step highlighted. Returns "" if lp is nil.
func RenderLoadPatternStrip(lp *LoadPattern, elapsed time.Duration, barWidth int) string {
	if lp == nil {
		return ""
	}
	pct := 0.0
	if lp.Total > 0 {
		pct = float64(elapsed) / float64(lp.Total)
		if pct > 1 {
			pct = 1
		}
	}
	bar := widgets.Progress(barWidth, pct)
	totalStr := formatPatternDuration(lp.Total)
	if lp.Total <= 0 {
		totalStr = "?"
	}
	header := fmt.Sprintf("Load Pattern: %s\n%s %.0f%% | %s / %s",
		lp.Name, bar, pct*100,
		formatPatternDuration(elapsed.Round(time.Second)), totalStr)

	parts := make([]string, 0, len(lp.Steps))
	for _, s := range lp.Steps {
		label := s.Label + "×" + formatPatternDuration(s.Duration)
		switch {
		case elapsed >= s.Start && elapsed < s.Start+s.Duration:
			parts = append(parts, "► "+label)
		case elapsed >= s.Start+s.Duration:
			parts = append(parts, "✓ "+label)
		default:
			parts = append(parts, "· "+label)
		}
	}
	return header + "\n" + strings.Join(parts, "  ")
}

// formatPatternDuration renders a duration trimming "0s" / "0m" tails after
// a unit letter — "1m0s" → "1m", "2h0m0s" → "2h", "20s" stays as "20s".
func formatPatternDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	s := d.String()
	for _, suf := range []string{"0s", "0m"} {
		if len(s) >= 3 && strings.HasSuffix(s, suf) {
			prev := s[len(s)-3]
			if prev >= 'a' && prev <= 'z' {
				s = s[:len(s)-2]
			}
		}
	}
	if s == "" {
		return "0s"
	}
	return s
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/runview/ -run TestRenderLoadPatternStrip -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/runview/loadpattern.go internal/tui/runview/loadpattern_test.go
git commit -m "feat(runview): add LoadPattern types and step strip renderer

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 2: Add status-bucket table widget

**Files:**
- Create: `internal/tui/widgets/statusbuckets.go`
- Create: `internal/tui/widgets/statusbuckets_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/widgets/statusbuckets_test.go`:

```go
package widgets_test

import (
	"strings"
	"testing"

	"github.com/torosent/crankfire/internal/tui/widgets"
)

func TestStatusBucketsTableRendersTopRows(t *testing.T) {
	buckets := map[string]map[string]int{
		"http": {"500": 12, "503": 4},
		"grpc": {"UNAVAILABLE": 3},
	}
	out := widgets.StatusBucketsTable(buckets, 10)
	for _, want := range []string{"HTTP 500 12", "HTTP 503 4", "GRPC UNAVAILABLE 3"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestStatusBucketsTableEmptyReturnsEmpty(t *testing.T) {
	if got := widgets.StatusBucketsTable(nil, 10); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestStatusBucketsTableLimitsRows(t *testing.T) {
	buckets := map[string]map[string]int{
		"http": {"500": 5, "501": 4, "502": 3, "503": 2, "504": 1},
	}
	out := widgets.StatusBucketsTable(buckets, 2)
	if got := strings.Count(out, "\n"); got > 2 {
		t.Errorf("expected at most 2 rows, got %d:\n%s", got, out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/widgets/ -run TestStatusBuckets -count=1`
Expected: FAIL — `undefined: widgets.StatusBucketsTable`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/tui/widgets/statusbuckets.go`:

```go
package widgets

import (
	"fmt"
	"sort"
	"strings"
)

// StatusBucketsTable renders the top maxRows rows from a protocol→code→count
// map, sorted by descending count. Returns "" if buckets is empty.
func StatusBucketsTable(buckets map[string]map[string]int, maxRows int) string {
	type row struct {
		protocol string
		code     string
		count    int
	}
	var rows []row
	for proto, codes := range buckets {
		for code, count := range codes {
			rows = append(rows, row{protocol: proto, code: code, count: count})
		}
	}
	if len(rows) == 0 {
		return ""
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		if rows[i].protocol != rows[j].protocol {
			return rows[i].protocol < rows[j].protocol
		}
		return rows[i].code < rows[j].code
	})
	if maxRows > 0 && len(rows) > maxRows {
		rows = rows[:maxRows]
	}
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%s %s %d\n", strings.ToUpper(r.protocol), r.code, r.count)
	}
	return b.String()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/widgets/ -run TestStatusBuckets -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/widgets/statusbuckets.go internal/tui/widgets/statusbuckets_test.go
git commit -m "feat(widgets): add StatusBucketsTable formatter

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 3: Add protocol-metrics widget

**Files:**
- Create: `internal/tui/widgets/protocolmetrics.go`
- Create: `internal/tui/widgets/protocolmetrics_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/widgets/protocolmetrics_test.go`:

```go
package widgets_test

import (
	"strings"
	"testing"

	"github.com/torosent/crankfire/internal/tui/widgets"
)

func TestProtocolMetricsBlockRendersSortedAndLimited(t *testing.T) {
	in := map[string]map[string]interface{}{
		"websocket": {"connections": 12, "messages_sent": 3400, "messages_recv": 3399, "errors": 1, "extra": "x"},
		"http":      {"keepalive_reused": 0.92},
	}
	out := widgets.ProtocolMetricsBlock(in, 4)
	if !strings.Contains(out, "http:") || !strings.Contains(out, "websocket:") {
		t.Errorf("expected both protocols, got:\n%s", out)
	}
	if strings.Index(out, "http:") > strings.Index(out, "websocket:") {
		t.Errorf("expected http listed before websocket alphabetically:\n%s", out)
	}
	if strings.Contains(out, "extra") {
		t.Errorf("expected metric beyond limit to be dropped (limit=4):\n%s", out)
	}
	if !strings.Contains(out, "0.92") {
		t.Errorf("expected float value rendered, got:\n%s", out)
	}
	if !strings.Contains(out, "3400") {
		t.Errorf("expected int value rendered, got:\n%s", out)
	}
}

func TestProtocolMetricsBlockEmptyReturnsEmpty(t *testing.T) {
	if got := widgets.ProtocolMetricsBlock(nil, 4); got != "" {
		t.Errorf("expected empty for nil input, got %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/widgets/ -run TestProtocolMetrics -count=1`
Expected: FAIL — `undefined: widgets.ProtocolMetricsBlock`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/tui/widgets/protocolmetrics.go`:

```go
package widgets

import (
	"fmt"
	"sort"
	"strings"
)

// ProtocolMetricsBlock renders per-protocol metrics, alphabetically by
// protocol, limited to perProtocol metrics each. Returns "" if input empty.
func ProtocolMetricsBlock(metrics map[string]map[string]interface{}, perProtocol int) string {
	if len(metrics) == 0 {
		return ""
	}
	protocols := make([]string, 0, len(metrics))
	for p := range metrics {
		protocols = append(protocols, p)
	}
	sort.Strings(protocols)

	var b strings.Builder
	for _, p := range protocols {
		fmt.Fprintf(&b, "%s:\n", p)
		keys := make([]string, 0, len(metrics[p]))
		for k := range metrics[p] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		count := 0
		for _, k := range keys {
			if perProtocol > 0 && count >= perProtocol {
				break
			}
			fmt.Fprintf(&b, "  %s: %s\n", k, formatMetricValue(metrics[p][k]))
			count++
		}
	}
	return b.String()
}

func formatMetricValue(v interface{}) string {
	switch x := v.(type) {
	case int:
		return fmt.Sprintf("%d", x)
	case int64:
		return fmt.Sprintf("%d", x)
	case float64:
		if x > 1000 {
			return fmt.Sprintf("%.0f", x)
		}
		return fmt.Sprintf("%.2f", x)
	case string:
		return x
	default:
		return fmt.Sprintf("%v", v)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/widgets/ -run TestProtocolMetrics -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/widgets/protocolmetrics.go internal/tui/widgets/protocolmetrics_test.go
git commit -m "feat(widgets): add ProtocolMetricsBlock formatter

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 4: Extend `EndpointRow` / `EndpointTable` with share, RPS, errors

**Files:**
- Modify: `internal/tui/widgets/tables.go`
- Modify: `internal/tui/widgets/widgets_test.go`

- [ ] **Step 1: Read existing widgets_test.go**

Run: `cat internal/tui/widgets/widgets_test.go`
Note: existing tests reference current `EndpointRow{Method, Path, Count, P95Ms, ErrPct}` — we must keep those fields.

- [ ] **Step 2: Write the failing test**

Append to `internal/tui/widgets/widgets_test.go`:

```go
func TestEndpointTableRendersExtendedColumns(t *testing.T) {
	rows := []widgets.EndpointRow{
		{Method: "GET", Path: "/users", Count: 1000, SharePct: 60.0, RPS: 50.5, P95Ms: 120, P99Ms: 240, ErrPct: 1.5, Errors: 15},
	}
	out := widgets.EndpointTable(rows, 10)
	for _, want := range []string{"GET", "/users", "1000", "60.0%", "50.5", "p95", "120", "p99", "240", "err 15"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
```

(Add `import "strings"` and `"github.com/torosent/crankfire/internal/tui/widgets"` if not already present.)

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/tui/widgets/ -run TestEndpointTableRendersExtendedColumns -count=1`
Expected: FAIL — `unknown field 'SharePct' in struct literal of type widgets.EndpointRow`.

- [ ] **Step 4: Update `EndpointRow` and `EndpointTable`**

Replace the contents of `internal/tui/widgets/tables.go` with:

```go
// internal/tui/widgets/tables.go
package widgets

import (
	"fmt"
	"sort"
	"strings"
)

func PercentileTable(p map[string]float64) string {
	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%-5s %8.1f\n", k, p[k])
	}
	return b.String()
}

type EndpointRow struct {
	Method   string
	Path     string
	Count    int64
	SharePct float64
	RPS      float64
	P95Ms    float64
	P99Ms    float64
	ErrPct   float64
	Errors   int64
}

func EndpointTable(rows []EndpointRow, max int) string {
	if max > 0 && len(rows) > max {
		rows = rows[:max]
	}
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b,
			"%-6s %-24s %8d  %5.1f%%  rps %6.1f  p95 %5.0fms  p99 %5.0fms  err %d (%4.1f%%)\n",
			r.Method, r.Path, r.Count, r.SharePct, r.RPS, r.P95Ms, r.P99Ms, r.Errors, r.ErrPct,
		)
	}
	return b.String()
}
```

- [ ] **Step 5: Run all widgets tests**

Run: `go test ./internal/tui/widgets/ -count=1`
Expected: PASS (existing endpoint tests still pass because new fields are zero-valued for them).

- [ ] **Step 6: Verify nothing else broke**

Run: `go build ./...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/widgets/tables.go internal/tui/widgets/widgets_test.go
git commit -m "feat(widgets): extend EndpointRow with share, RPS, p99, error count

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 5: Extend `runview.Options` and `SnapshotMsg`

**Files:**
- Modify: `internal/tui/runview/runview.go`
- Modify: `internal/tui/runview/runview_test.go`

- [ ] **Step 1: Write failing tests for new fields**

Append to `internal/tui/runview/runview_test.go` (and add `"time"` to imports if not already present):

```go
func TestRunViewRendersHeaderLines(t *testing.T) {
	rv := runview.New(runview.Options{
		Title:  "Smoke",
		Total:  100,
		Header: []string{"Target: https://example.com", "Workers: 10 | Rate: 50/s"},
	})
	out := rv.View()
	for _, want := range []string{"Target: https://example.com", "Workers: 10 | Rate: 50/s"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRunViewRendersLoadPatternWhenConfigured(t *testing.T) {
	lp := &runview.LoadPattern{
		Name:  "ramp",
		Total: 10 * time.Second,
		Steps: []runview.PatternStep{
			{Label: "10 RPS", Duration: 5 * time.Second, Start: 0},
			{Label: "50 RPS", Duration: 5 * time.Second, Start: 5 * time.Second},
		},
	}
	rv := runview.New(runview.Options{Title: "x", LoadPattern: lp})
	rv2, _ := rv.Update(runview.SnapshotMsg{Elapsed: 6 * time.Second})
	out := rv2.View()
	if !strings.Contains(out, "Load Pattern: ramp") {
		t.Errorf("expected load pattern header, got:\n%s", out)
	}
	if !strings.Contains(out, "► 50 RPS") {
		t.Errorf("expected current step highlighted, got:\n%s", out)
	}
}

func TestRunViewRendersStatusBucketsAndProtocolMetrics(t *testing.T) {
	rv := runview.New(runview.Options{Title: "x"})
	rv2, _ := rv.Update(runview.SnapshotMsg{
		StatusBuckets:   map[string]map[string]int{"http": {"500": 7}},
		ProtocolMetrics: map[string]map[string]interface{}{"http": {"keepalive": 0.9}},
	})
	out := rv2.View()
	if !strings.Contains(out, "HTTP 500 7") {
		t.Errorf("expected status bucket row, got:\n%s", out)
	}
	if !strings.Contains(out, "http:") || !strings.Contains(out, "keepalive") {
		t.Errorf("expected protocol metrics block, got:\n%s", out)
	}
}

func TestRunViewLoadPatternHidesProtocolMetrics(t *testing.T) {
	lp := &runview.LoadPattern{Name: "p", Total: time.Second, Steps: []runview.PatternStep{{Label: "1", Duration: time.Second}}}
	rv := runview.New(runview.Options{Title: "x", LoadPattern: lp})
	rv2, _ := rv.Update(runview.SnapshotMsg{
		Elapsed:         time.Millisecond,
		ProtocolMetrics: map[string]map[string]interface{}{"http": {"keepalive": 0.9}},
	})
	out := rv2.View()
	if strings.Contains(out, "keepalive") {
		t.Errorf("protocol metrics should be hidden when load pattern present, got:\n%s", out)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/runview/ -count=1`
Expected: FAIL — unknown fields `Header`, `LoadPattern`, `Elapsed`, `StatusBuckets`, `ProtocolMetrics`.

- [ ] **Step 3: Replace `runview.go`**

Replace `internal/tui/runview/runview.go` with:

```go
// Package runview is a reusable bubbletea component that renders a live load
// test dashboard (header, progress, throughput sparkline, percentile and
// endpoint tables, load pattern progress, status buckets, protocol metrics)
// from metrics snapshots.
package runview

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/tui/widgets"
)

const sparklineCapacity = 120

// Options configures a Model at construction time.
type Options struct {
	Title       string
	Total       int64
	Header      []string
	LoadPattern *LoadPattern
}

// SnapshotMsg delivers a new metrics data point to the runview. New fields
// are optional; the original {Snap, Endpoints} pair remains supported.
type SnapshotMsg struct {
	Snap            metrics.DataPoint
	Endpoints       []widgets.EndpointRow
	Stats           *metrics.Stats
	StatusBuckets   map[string]map[string]int
	ProtocolMetrics map[string]map[string]interface{}
	Elapsed         time.Duration
}

// Model is the runview bubbletea model.
type Model struct {
	title           string
	total           int64
	header          []string
	loadPattern     *LoadPattern
	latest          metrics.DataPoint
	stats           *metrics.Stats
	endpoints       []widgets.EndpointRow
	statusBuckets   map[string]map[string]int
	protocolMetrics map[string]map[string]interface{}
	elapsed         time.Duration
	width           int
	ring            []float64
}

// New creates a runview Model from Options.
func New(opts Options) Model {
	return Model{
		title:       opts.Title,
		total:       opts.Total,
		header:      opts.Header,
		loadPattern: opts.LoadPattern,
		ring:        make([]float64, 0, sparklineCapacity),
		width:       80,
	}
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update handles SnapshotMsg and tea.WindowSizeMsg.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch v := msg.(type) {
	case SnapshotMsg:
		m.latest = v.Snap
		if v.Endpoints != nil {
			m.endpoints = v.Endpoints
		}
		if v.Stats != nil {
			m.stats = v.Stats
		}
		if v.StatusBuckets != nil {
			m.statusBuckets = v.StatusBuckets
		}
		if v.ProtocolMetrics != nil {
			m.protocolMetrics = v.ProtocolMetrics
		}
		if v.Elapsed > 0 {
			m.elapsed = v.Elapsed
		}
		m.ring = append(m.ring, v.Snap.CurrentRPS)
		if len(m.ring) > sparklineCapacity {
			m.ring = m.ring[len(m.ring)-sparklineCapacity:]
		}
	case tea.WindowSizeMsg:
		if v.Width > 0 {
			m.width = v.Width
		}
	}
	return m, nil
}

// SparklineLen returns the current number of samples held in the ring buffer.
func (m Model) SparklineLen() int { return len(m.ring) }

// View renders the dashboard.
func (m Model) View() string {
	var b strings.Builder

	if m.title != "" {
		fmt.Fprintf(&b, "%s\n", m.title)
	}

	for _, line := range m.header {
		b.WriteString(line)
		b.WriteString("\n")
	}

	frac := 0.0
	if m.total > 0 {
		frac = float64(m.latest.TotalRequests) / float64(m.total)
	}
	barWidth := m.barWidth()
	fmt.Fprintf(&b, "Progress %s %d/%d\n",
		widgets.Progress(barWidth, frac), m.latest.TotalRequests, m.total)

	fmt.Fprintf(&b, "Throughput %s %.0f rps\n",
		widgets.Sparkline(m.ring, barWidth), m.latest.CurrentRPS)

	b.WriteString("Latency (ms)\n")
	b.WriteString(widgets.PercentileTable(m.latencyMap()))

	fmt.Fprintf(&b, "Errors %d / Total %d\n", m.latest.Errors, m.latest.TotalRequests)

	// Load pattern OR protocol metrics — same slot.
	if m.loadPattern != nil {
		strip := RenderLoadPatternStrip(m.loadPattern, m.elapsed, barWidth)
		if strip != "" {
			b.WriteString(strip)
			b.WriteString("\n")
		}
	} else if pm := widgets.ProtocolMetricsBlock(m.protocolMetrics, 4); pm != "" {
		b.WriteString("Protocol Metrics\n")
		b.WriteString(pm)
	}

	if len(m.endpoints) > 0 {
		b.WriteString("Endpoints\n")
		b.WriteString(widgets.EndpointTable(m.endpoints, 10))
	}

	if sb := widgets.StatusBucketsTable(m.statusBuckets, 10); sb != "" {
		b.WriteString("Failing Status Codes\n")
		b.WriteString(sb)
	}

	b.WriteString("\n[q] quit  [p] pause")
	return b.String()
}

func (m Model) barWidth() int {
	w := m.width - 20
	if w < 10 {
		return 10
	}
	if w > 60 {
		return 60
	}
	return w
}

func (m Model) latencyMap() map[string]float64 {
	if m.stats != nil {
		return map[string]float64{
			"min":  m.stats.MinLatencyMs,
			"mean": m.stats.MeanLatencyMs,
			"p50":  m.stats.P50LatencyMs,
			"p90":  m.stats.P90LatencyMs,
			"p95":  m.stats.P95LatencyMs,
			"p99":  m.stats.P99LatencyMs,
		}
	}
	return map[string]float64{
		"p50": m.latest.P50LatencyMs,
		"p95": m.latest.P95LatencyMs,
		"p99": m.latest.P99LatencyMs,
	}
}
```

- [ ] **Step 4: Run all runview tests**

Run: `go test ./internal/tui/runview/ -count=1`
Expected: PASS.

- [ ] **Step 5: Verify the rest of the workspace still builds**

Run: `go build ./...`
Expected: PASS.

- [ ] **Step 6: Run TUI screens tests**

Run: `go test ./internal/tui/... -count=1`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/runview/runview.go internal/tui/runview/runview_test.go
git commit -m "feat(runview): support header, load pattern, status buckets, protocol metrics, full latency stats

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 6: Create `internal/cli/livedash` driver — basic Driver + Opts

**Files:**
- Create: `internal/cli/livedash/livedash.go`
- Create: `internal/cli/livedash/livedash_test.go`

- [ ] **Step 1: Inspect Collector API to confirm method signatures used in tests**

Run: `grep -n "^func (c \*Collector)" internal/metrics/collector.go`
Expected: confirms `Start()`, `Snapshot()`, `Stats(time.Duration) Stats`, `History() []DataPoint`, plus the request-recording methods. Use whatever names the collector actually exports for recording success/failure in the tests below; if they differ from `RecordSuccess`/`RecordFailure(latency, method, path, protocol, statusCode)`, adjust the test calls accordingly before running them.

- [ ] **Step 2: Write the failing test**

Create `internal/cli/livedash/livedash_test.go`:

```go
package livedash_test

import (
	"strings"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/cli/livedash"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/tui/runview"
)

func TestBuildSnapshotPopulatesAllFields(t *testing.T) {
	c := metrics.NewCollector()
	c.Start()
	c.RecordSuccess(50*time.Millisecond, "GET", "/", "http", "200")
	c.RecordSuccess(80*time.Millisecond, "GET", "/", "http", "200")
	c.RecordFailure(20*time.Millisecond, "GET", "/", "http", "500")
	c.Snapshot()

	snap := livedash.BuildSnapshot(c, 1*time.Second)
	if snap.Stats == nil {
		t.Fatalf("expected Stats populated")
	}
	if snap.Stats.Total != 3 {
		t.Errorf("Stats.Total = %d, want 3", snap.Stats.Total)
	}
	if snap.Elapsed != 1*time.Second {
		t.Errorf("Elapsed = %s, want 1s", snap.Elapsed)
	}
	if got := snap.StatusBuckets["http"]["500"]; got != 1 {
		t.Errorf("StatusBuckets http/500 = %d, want 1", got)
	}
	if len(snap.Endpoints) == 0 {
		t.Errorf("expected at least one endpoint row")
	}
	if snap.Endpoints[0].Path != "/" || snap.Endpoints[0].Method != "GET" {
		t.Errorf("endpoint = %+v, want method=GET path=/", snap.Endpoints[0])
	}
}

func TestRenderViaRunviewIncludesHeaderAndStatus(t *testing.T) {
	c := metrics.NewCollector()
	c.Start()
	c.RecordFailure(10*time.Millisecond, "GET", "/", "http", "500")
	c.Snapshot()

	d := livedash.New(c, livedash.Opts{
		Title:  "T",
		Header: []string{"Target: https://example.com"},
		Total:  10,
	}, func() {})

	snap := livedash.BuildSnapshot(c, 500*time.Millisecond)
	model := d.ModelForTest()
	updated, _ := model.Update(runview.SnapshotMsg(snap))
	out := updated.View()
	if !strings.Contains(out, "Target: https://example.com") {
		t.Errorf("missing header: %s", out)
	}
	if !strings.Contains(out, "HTTP 500 1") {
		t.Errorf("missing failing status row: %s", out)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/cli/livedash/ -count=1`
Expected: FAIL — package does not exist yet.

- [ ] **Step 4: Write minimal implementation**

Create `internal/cli/livedash/livedash.go`:

```go
// Package livedash is the CLI-facing driver that hosts a runview.Model in a
// bubbletea Program, polls the metrics collector on a fixed interval, and
// forwards user keystrokes (q/esc/ctrl+c) to a shutdown callback.
package livedash

import (
	"fmt"
	"sort"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/tui/runview"
	"github.com/torosent/crankfire/internal/tui/widgets"
)

const defaultTickInterval = 500 * time.Millisecond

// Opts configures the Driver and its embedded runview.
type Opts struct {
	Title       string
	Header      []string
	Total       int64
	LoadPattern *runview.LoadPattern
	Interval    time.Duration
}

// Driver wires a metrics collector to a runview.Model rendered by a
// bubbletea Program.
type Driver struct {
	collector *metrics.Collector
	opts      Opts
	shutdown  func()

	mu       sync.Mutex
	model    runview.Model
	program  *tea.Program
	wg       sync.WaitGroup
	started  time.Time
	finalDur time.Duration
}

// New constructs a Driver. shutdown is invoked exactly once when the user
// requests an exit (q/esc/ctrl+c).
func New(c *metrics.Collector, opts Opts, shutdown func()) *Driver {
	if opts.Interval <= 0 {
		opts.Interval = defaultTickInterval
	}
	return &Driver{
		collector: c,
		opts:      opts,
		shutdown:  shutdown,
		model: runview.New(runview.Options{
			Title:       opts.Title,
			Total:       opts.Total,
			Header:      opts.Header,
			LoadPattern: opts.LoadPattern,
		}),
	}
}

// ModelForTest exposes the internal runview model for white-box tests.
func (d *Driver) ModelForTest() runview.Model { return d.model }

// BuildSnapshot reads the current collector state and returns a SnapshotMsg
// suitable for forwarding to runview.
func BuildSnapshot(c *metrics.Collector, elapsed time.Duration) runview.SnapshotMsg {
	stats := c.Stats(elapsed)
	endpoints := buildEndpointRows(stats)
	var latest metrics.DataPoint
	if hist := c.History(); len(hist) > 0 {
		latest = hist[len(hist)-1]
	} else {
		latest = metrics.DataPoint{
			TotalRequests:      stats.Total,
			SuccessfulRequests: stats.Successes,
			Errors:             stats.Failures,
			CurrentRPS:         stats.RequestsPerSec,
			P50LatencyMs:       stats.P50LatencyMs,
			P95LatencyMs:       stats.P95LatencyMs,
			P99LatencyMs:       stats.P99LatencyMs,
		}
	}
	return runview.SnapshotMsg{
		Snap:            latest,
		Endpoints:       endpoints,
		Stats:           &stats,
		StatusBuckets:   stats.StatusBuckets,
		ProtocolMetrics: stats.ProtocolMetrics,
		Elapsed:         elapsed,
	}
}

func buildEndpointRows(stats metrics.Stats) []widgets.EndpointRow {
	if len(stats.Endpoints) == 0 {
		return nil
	}
	type kv struct {
		name string
		st   metrics.EndpointStats
	}
	rows := make([]kv, 0, len(stats.Endpoints))
	for name, st := range stats.Endpoints {
		rows = append(rows, kv{name: name, st: st})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].st.Total == rows[j].st.Total {
			return rows[i].name < rows[j].name
		}
		return rows[i].st.Total > rows[j].st.Total
	})
	out := make([]widgets.EndpointRow, 0, len(rows))
	for _, r := range rows {
		method, path := splitEndpoint(r.name)
		share := 0.0
		errPct := 0.0
		if stats.Total > 0 {
			share = float64(r.st.Total) / float64(stats.Total) * 100
		}
		if r.st.Total > 0 {
			errPct = float64(r.st.Failures) / float64(r.st.Total) * 100
		}
		out = append(out, widgets.EndpointRow{
			Method:   method,
			Path:     path,
			Count:    r.st.Total,
			SharePct: share,
			RPS:      r.st.RequestsPerSec,
			P95Ms:    r.st.P95LatencyMs,
			P99Ms:    r.st.P99LatencyMs,
			ErrPct:   errPct,
			Errors:   r.st.Failures,
		})
	}
	return out
}

// splitEndpoint splits a "METHOD path" key into ("METHOD", "path"). If no
// space, returns ("", name).
func splitEndpoint(name string) (string, string) {
	for i := 0; i < len(name); i++ {
		if name[i] == ' ' {
			return name[:i], name[i+1:]
		}
	}
	return "", name
}

type tickMsg struct{}

// Init runs on Program start.
func (d *Driver) Init() tea.Cmd {
	return tea.Tick(d.opts.Interval, func(time.Time) tea.Msg { return tickMsg{} })
}

// Update handles ticks, key events, and resize.
func (d *Driver) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tickMsg:
		d.collector.Snapshot()
		snap := BuildSnapshot(d.collector, time.Since(d.started))
		d.mu.Lock()
		d.model, _ = d.model.Update(snap)
		d.mu.Unlock()
		return d, tea.Tick(d.opts.Interval, func(time.Time) tea.Msg { return tickMsg{} })
	case tea.KeyMsg:
		switch m.String() {
		case "q", "esc", "ctrl+c":
			if d.shutdown != nil {
				d.shutdown()
			}
			return d, tea.Quit
		}
	case tea.WindowSizeMsg:
		d.mu.Lock()
		d.model, _ = d.model.Update(m)
		d.mu.Unlock()
	}
	return d, nil
}

// View renders the embedded model.
func (d *Driver) View() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.model.View()
}

// Start launches the bubbletea program in alt-screen mode in a background
// goroutine. Returns immediately.
func (d *Driver) Start() error {
	d.started = time.Now()
	d.program = tea.NewProgram(d, tea.WithAltScreen())
	d.wg.Add(1)
	var startErr error
	ready := make(chan struct{})
	go func() {
		defer d.wg.Done()
		close(ready)
		if _, err := d.program.Run(); err != nil {
			startErr = fmt.Errorf("livedash: %w", err)
		}
	}()
	<-ready
	return startErr
}

// Stop signals the program to quit, waits for it to exit, and returns final
// stats computed against the elapsed run time.
func (d *Driver) Stop() metrics.Stats {
	if d.program != nil {
		d.program.Quit()
	}
	d.wg.Wait()
	d.finalDur = time.Since(d.started)
	return d.collector.Stats(d.finalDur)
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/cli/livedash/ -count=1`
Expected: PASS. If `RecordSuccess`/`RecordFailure` signatures differ, adjust per Step 1 note.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/livedash/
git commit -m "feat(livedash): add bubbletea CLI driver wrapping runview

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 7: Add livedash test for shutdown callback and final stats

**Files:**
- Modify: `internal/cli/livedash/livedash_test.go`

- [ ] **Step 1: Append the failing tests**

Append to `internal/cli/livedash/livedash_test.go` (and add the imports below to the existing import block):

```go
import (
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
)

func TestKeyQInvokesShutdownAndQuits(t *testing.T) {
	c := metrics.NewCollector()
	c.Start()
	var called int32
	d := livedash.New(c, livedash.Opts{Title: "x"}, func() {
		atomic.AddInt32(&called, 1)
	})
	if _, cmd := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}); cmd == nil {
		t.Fatalf("expected tea.Quit cmd, got nil")
	}
	if atomic.LoadInt32(&called) != 1 {
		t.Errorf("shutdown called %d times, want 1", atomic.LoadInt32(&called))
	}
}

func TestStopReturnsFinalStats(t *testing.T) {
	c := metrics.NewCollector()
	c.Start()
	c.RecordSuccess(10*time.Millisecond, "GET", "/", "http", "200")
	c.Snapshot()

	d := livedash.New(c, livedash.Opts{Title: "x", Interval: 10 * time.Millisecond}, func() {})
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(40 * time.Millisecond)
	stats := d.Stop()
	if stats.Total < 1 {
		t.Errorf("expected stats.Total >= 1, got %d", stats.Total)
	}
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/cli/livedash/ -count=1 -run "TestKeyQ|TestStopReturns"`
Expected: PASS. If the bubbletea `tea.KeyMsg` shape differs in your version, switch to whatever literal expresses a `q` keypress (e.g., `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}`).

- [ ] **Step 3: Commit**

```bash
git add internal/cli/livedash/livedash_test.go
git commit -m "test(livedash): cover shutdown callback and final stats

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 8: Wire `internal/cli/run.go` to use `livedash`

**Files:**
- Modify: `internal/cli/run.go`

- [ ] **Step 1: Read current dashboard wiring (lines 73-110 and 160-163) for context**

Run: `sed -n '70,115p;155,170p' internal/cli/run.go`
Note the import block, the `var dash *dashboard.Dashboard` block, and the explicit `dash.Stop()` at the end of the function.

- [ ] **Step 2: Swap imports**

In `internal/cli/run.go`, change:

```go
	"github.com/torosent/crankfire/internal/dashboard"
```

to:

```go
	"github.com/torosent/crankfire/internal/cli/livedash"
	"github.com/torosent/crankfire/internal/tui/runview"
```

If `internal/metrics` isn't already imported in this file, also add:

```go
	"github.com/torosent/crankfire/internal/metrics"
```

- [ ] **Step 3: Replace the dashboard construction block**

Replace the entire block that begins with `var dash *dashboard.Dashboard` and ends with the matching `defer ... dash.Stop()` (lines ~73-109) with:

```go
	var dash *livedash.Driver
	var dashStats metrics.Stats
	if cfg.Dashboard {
		targetURL := cfg.TargetURL
		if targetURL == "" && len(cfg.Endpoints) > 0 {
			targetURL = cfg.Endpoints[0].URL
			if targetURL == "" && cfg.Endpoints[0].Path != "" {
				targetURL = cfg.Endpoints[0].Path
			}
		}
		opts := livedash.Opts{
			Title:       "Crankfire",
			Header:      buildDashHeader(targetURL, *cfg),
			Total:       int64(cfg.Total),
			LoadPattern: buildLoadPattern(cfg.LoadPatterns),
		}
		dash = livedash.New(collector, opts, cancel)
		if err := dash.Start(); err != nil {
			return err
		}
		defer func() {
			if dash != nil {
				dashStats = dash.Stop()
			}
		}()
	}
	_ = dashStats // reserved for future use
```

- [ ] **Step 4: Delete the explicit `dash.Stop()` call later in the function**

Find and delete:

```go
	if dash != nil {
		dash.Stop()
		dash = nil
	}
```

(the `defer` from Step 3 now handles it).

- [ ] **Step 5: Replace `buildDashPatternSteps` with `buildLoadPattern` + `buildDashHeader`**

At the bottom of `internal/cli/run.go`, delete the existing `buildDashPatternSteps` function and add the following two helpers. Note the `config.Config` type and the load-pattern field/constant names — if they don't compile, find the right names with `grep -n "type Config struct\|LoadPatternType\|type LoadPattern struct" internal/config/*.go` and update accordingly.

```go
// buildLoadPattern converts config load patterns into a runview LoadPattern
// suitable for the live dashboard. Returns nil if no patterns are configured.
func buildLoadPattern(patterns []config.LoadPattern) *runview.LoadPattern {
	if len(patterns) == 0 {
		return nil
	}
	var steps []runview.PatternStep
	var names []string
	var offset time.Duration
	for _, p := range patterns {
		switch p.Type {
		case config.LoadPatternTypeStep:
			names = append(names, "step")
			for _, s := range p.Steps {
				if s.Duration <= 0 {
					continue
				}
				steps = append(steps, runview.PatternStep{
					Label:    fmt.Sprintf("%d RPS", s.RPS),
					Duration: s.Duration,
					Start:    offset,
				})
				offset += s.Duration
			}
		case config.LoadPatternTypeRamp:
			if p.Duration <= 0 {
				continue
			}
			names = append(names, "ramp")
			steps = append(steps, runview.PatternStep{
				Label:    fmt.Sprintf("%d→%d RPS", p.FromRPS, p.ToRPS),
				Duration: p.Duration,
				Start:    offset,
			})
			offset += p.Duration
		case config.LoadPatternTypeSpike:
			if p.Duration <= 0 {
				continue
			}
			names = append(names, "spike")
			steps = append(steps, runview.PatternStep{
				Label:    fmt.Sprintf("Spike %d RPS", p.RPS),
				Duration: p.Duration,
				Start:    offset,
			})
			offset += p.Duration
		default:
			if p.Duration <= 0 {
				continue
			}
			names = append(names, string(p.Type))
			steps = append(steps, runview.PatternStep{
				Label:    fmt.Sprintf("%d RPS", p.RPS),
				Duration: p.Duration,
				Start:    offset,
			})
			offset += p.Duration
		}
	}
	if len(steps) == 0 {
		return nil
	}
	return &runview.LoadPattern{
		Name:  strings.Join(names, "+"),
		Total: offset,
		Steps: steps,
	}
}

// buildDashHeader returns the header lines shown above the live dashboard
// progress bar (target URL + key test parameters).
func buildDashHeader(targetURL string, cfg config.Config) []string {
	lines := []string{fmt.Sprintf("Target: %s", targetURL)}
	var parts []string
	if cfg.Protocol != "" && string(cfg.Protocol) != "http" {
		parts = append(parts, fmt.Sprintf("Protocol: %s", cfg.Protocol))
	}
	if cfg.Method != "" && cfg.Method != "GET" {
		parts = append(parts, fmt.Sprintf("Method: %s", cfg.Method))
	}
	if cfg.Concurrency > 0 {
		parts = append(parts, fmt.Sprintf("Workers: %d", cfg.Concurrency))
	}
	if cfg.Rate > 0 {
		parts = append(parts, fmt.Sprintf("Rate: %d/s", cfg.Rate))
	} else {
		parts = append(parts, "Rate: unlimited")
	}
	if cfg.Duration > 0 {
		parts = append(parts, fmt.Sprintf("Duration: %s", cfg.Duration))
	}
	if cfg.Total > 0 {
		parts = append(parts, fmt.Sprintf("Total: %d", cfg.Total))
	}
	if cfg.Timeout > 0 {
		parts = append(parts, fmt.Sprintf("Timeout: %s", cfg.Timeout))
	}
	if cfg.Retries > 0 {
		parts = append(parts, fmt.Sprintf("Retries: %d", cfg.Retries))
	}
	if cfg.ConfigFile != "" {
		parts = append(parts, fmt.Sprintf("Config: %s", cfg.ConfigFile))
	}
	if len(parts) > 0 {
		lines = append(lines, strings.Join(parts, " | "))
	}
	return lines
}
```

- [ ] **Step 6: Build to verify wiring compiles**

Run: `go build ./...`
Expected: PASS. Fix any compile errors (likely: missing imports, wrong field names).

- [ ] **Step 7: Run all tests**

Run: `go test -race ./... -count=1`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/cli/run.go
git commit -m "refactor(cli): route --dashboard through bubbletea livedash driver

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 9: Delete `internal/dashboard/` and run `go mod tidy`

**Files:**
- Delete: `internal/dashboard/`
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Confirm no remaining references**

Run: `grep -rn "internal/dashboard\|gizak/termui" --include="*.go"`
Expected: no output. If anything remains, fix it before deleting.

- [ ] **Step 2: Delete the package**

Run: `git rm -r internal/dashboard/`
Expected: `dashboard.go` and `dashboard_test.go` removed from the index.

- [ ] **Step 3: Run `go mod tidy`**

Run: `go mod tidy`
Expected: `go.mod` and `go.sum` updated; `github.com/gizak/termui/v3` and any termui-only transitives are removed.

- [ ] **Step 4: Verify the build**

Run: `go build ./... && go test -race ./... -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/dashboard go.mod go.sum
git commit -m "chore: remove internal/dashboard and gizak/termui dependency

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 10: Update documentation

**Files:**
- Modify: `docs/architecture/02-architecture-overview.md`
- Modify: `.github/copilot-instructions.md`

- [ ] **Step 1: Update the architecture diagram**

In `docs/architecture/02-architecture-overview.md`:

Change:

```
        termui[gizak/termui]
```

to:

```
        bubbletea[charmbracelet/bubbletea]
```

Change:

```
        dashboard --> termui
```

to:

```
        dashboard --> bubbletea
```

Change:

```
    style termui fill:#10b981
```

to:

```
    style bubbletea fill:#10b981
```

If any prose in the same file mentions termui or `internal/dashboard/`, update it to reference `internal/cli/livedash/` and `internal/tui/runview/`.

- [ ] **Step 2: Update copilot instructions**

In `.github/copilot-instructions.md`, replace the line:

```
- `internal/output/` and `internal/dashboard/` — JSON, HTML report, and live terminal dashboard output.
```

with:

```
- `internal/output/` — JSON and HTML report output.
- `internal/cli/livedash/` — bubbletea-based live terminal dashboard driver, embeds `internal/tui/runview/`.
```

- [ ] **Step 3: Verify no other doc references stale paths**

Run: `grep -rn "internal/dashboard\|gizak/termui" docs/ .github/ README.md 2>&1 | grep -v specs/`
Expected: only the spec doc remains (which is historical and should not be edited).

- [ ] **Step 4: Commit**

```bash
git add docs/architecture/02-architecture-overview.md .github/copilot-instructions.md
git commit -m "docs: update architecture refs after termui removal

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 11: End-to-end smoke

**Files:** none (verification only)

- [ ] **Step 1: Full test run**

Run: `go test -race ./... -count=1`
Expected: PASS across all packages.

- [ ] **Step 2: Verify the binary builds and starts the dashboard cleanly**

Run:
```bash
go build -o build/crankfire ./cmd/crankfire
./build/crankfire --target https://httpbin.org/status/200 --concurrency 2 --duration 2s --dashboard
```
Expected: bubbletea alt-screen renders for ~2 seconds with header, throughput sparkline, latency stats; exits cleanly with terminal restored. (Skip this step in non-interactive environments — it requires a TTY.)

- [ ] **Step 3: Run integration tests**

Run: `go test -v -tags=integration -race -timeout 15m ./cmd/crankfire/ -count=1`
Expected: PASS.

- [ ] **Step 4: Push**

```bash
git push
```

---

## Self-Review Notes

- **Spec coverage:** every section of the spec maps to one or more tasks above (runview extension → Tasks 1, 5; widgets → Tasks 2, 3, 4; livedash driver → Tasks 6, 7; CLI rewiring → Task 8; deletion + tidy → Task 9; docs → Task 10; smoke → Task 11).
- **Backward compat:** Task 5 keeps `runview.Options` and `runview.SnapshotMsg` strictly additive; existing TUI run-screen calls continue to compile and work.
- **No placeholders:** every step contains the exact code, command, or file change required.
- **Field-name caveat:** Task 6 Step 1 and Task 8 Step 5 explicitly flag that collector and config field names must match what the codebase actually exports, with verification commands. The migrations themselves are mechanical translation, not new design.
