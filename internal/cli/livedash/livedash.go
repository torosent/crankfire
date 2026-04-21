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
	Title          string
	Header         []string
	RequestContext runview.RequestContext
	Total          int64
	LoadPattern    *runview.LoadPattern
	Interval       time.Duration
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
			Title:          opts.Title,
			Total:          opts.Total,
			Header:         opts.Header,
			RequestContext: opts.RequestContext,
			LoadPattern:    opts.LoadPattern,
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
// goroutine. It blocks briefly to surface fast startup failures (e.g. no TTY
// available) before returning.
func (d *Driver) Start() error {
	d.started = time.Now()
	d.program = tea.NewProgram(d, tea.WithAltScreen())
	errCh := make(chan error, 1)
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		if _, err := d.program.Run(); err != nil {
			errCh <- fmt.Errorf("livedash: %w", err)
			return
		}
		errCh <- nil
	}()
	// Give the program a moment to fail fast on startup errors (e.g. /dev/tty
	// missing). If it's still running after the grace period, assume success.
	select {
	case err := <-errCh:
		return err
	case <-time.After(100 * time.Millisecond):
		return nil
	}
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
