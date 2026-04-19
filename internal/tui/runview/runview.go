// Package runview is a reusable bubbletea component that renders a live load
// test dashboard (progress, throughput sparkline, percentile and endpoint
// tables) from metrics snapshots.
package runview

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/tui/widgets"
)

const sparklineCapacity = 120

// Options configures a Model at construction time.
type Options struct {
	Title string
	Total int64
}

// SnapshotMsg delivers a new metrics data point to the runview.
type SnapshotMsg struct {
	Snap      metrics.DataPoint
	Endpoints []widgets.EndpointRow
}

// Model is the runview bubbletea model.
type Model struct {
	title     string
	total     int64
	latest    metrics.DataPoint
	endpoints []widgets.EndpointRow
	ring      []float64
}

// New creates a runview Model from Options.
func New(opts Options) Model {
	return Model{
		title: opts.Title,
		total: opts.Total,
		ring:  make([]float64, 0, sparklineCapacity),
	}
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update handles SnapshotMsg by appending throughput to a rolling ring buffer
// of size sparklineCapacity and recording the latest data point.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch v := msg.(type) {
	case SnapshotMsg:
		m.latest = v.Snap
		if v.Endpoints != nil {
			m.endpoints = v.Endpoints
		}
		m.ring = append(m.ring, v.Snap.CurrentRPS)
		if len(m.ring) > sparklineCapacity {
			m.ring = m.ring[len(m.ring)-sparklineCapacity:]
		}
	}
	return m, nil
}

// SparklineLen returns the current number of samples held in the ring buffer.
func (m Model) SparklineLen() int { return len(m.ring) }

// View renders the dashboard.
func (m Model) View() string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s\n", m.title)

	frac := 0.0
	if m.total > 0 {
		frac = float64(m.latest.TotalRequests) / float64(m.total)
	}
	fmt.Fprintf(&b, "Progress %s %d/%d\n",
		widgets.Progress(30, frac), m.latest.TotalRequests, m.total)

	fmt.Fprintf(&b, "Throughput %s %.0f rps\n",
		widgets.Sparkline(m.ring, 30), m.latest.CurrentRPS)

	b.WriteString("Latency (ms)\n")
	b.WriteString(widgets.PercentileTable(map[string]float64{
		"p50": m.latest.P50LatencyMs,
		"p95": m.latest.P95LatencyMs,
		"p99": m.latest.P99LatencyMs,
	}))

	fmt.Fprintf(&b, "Errors %d / Total %d\n", m.latest.Errors, m.latest.TotalRequests)

	if len(m.endpoints) > 0 {
		b.WriteString("Endpoints\n")
		b.WriteString(widgets.EndpointTable(m.endpoints, 10))
	}

	b.WriteString("\n[q] quit  [p] pause")
	return b.String()
}
