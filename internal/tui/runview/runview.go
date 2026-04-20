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
