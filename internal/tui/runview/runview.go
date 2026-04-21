// Package runview is a reusable bubbletea component that renders a live,
// panelized load-test dashboard from metrics snapshots. It supports summary,
// request rate, latency, endpoint, load-pattern, status-bucket, and protocol
// metrics panels for both the CLI dashboard and the interactive TUI.
package runview

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/tui/widgets"
)

const (
	sparklineCapacity = 120
	defaultWidth      = 80
	defaultHeight     = 24
	splitThreshold    = 80
	panelGap          = 1
	rpsStripLineCount = 3 // fixed line height of the compact RPS strip (meta / value / bar)
)

// Options configures a Model at construction time.
type Options struct {
	Title          string
	Total          int64
	Header         []string
	RequestContext RequestContext
	LoadPattern    *LoadPattern
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
	requestContext  RequestContext
	loadPattern     *LoadPattern
	latest          metrics.DataPoint
	stats           *metrics.Stats
	endpoints       []widgets.EndpointRow
	statusBuckets   map[string]map[string]int
	protocolMetrics map[string]map[string]interface{}
	elapsed         time.Duration
	width           int
	height          int
	latencyRing     []float64
	rpsRing         []float64
}

type panelSpec struct {
	title string
	body  string
}

// New creates a runview Model from Options.
func New(opts Options) Model {
	return Model{
		title:          opts.Title,
		total:          opts.Total,
		header:         opts.Header,
		requestContext: opts.RequestContext,
		loadPattern:    opts.LoadPattern,
		latencyRing:    make([]float64, 0, sparklineCapacity),
		rpsRing:        make([]float64, 0, sparklineCapacity),
		width:          defaultWidth,
		height:         defaultHeight,
	}
}

// RequestContextForTest exposes the request context for white-box tests.
func (m Model) RequestContextForTest() RequestContext { return m.requestContext }

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

		m.latencyRing = append(m.latencyRing, latencySample(v))
		if len(m.latencyRing) > sparklineCapacity {
			m.latencyRing = m.latencyRing[len(m.latencyRing)-sparklineCapacity:]
		}
		m.rpsRing = append(m.rpsRing, rpsSample(v))
		if len(m.rpsRing) > sparklineCapacity {
			m.rpsRing = m.rpsRing[len(m.rpsRing)-sparklineCapacity:]
		}
	case tea.WindowSizeMsg:
		if v.Width > 0 {
			m.width = v.Width
		}
		if v.Height > 0 {
			m.height = v.Height
		}
	}
	return m, nil
}

// SparklineLen returns the current number of samples held in the ring buffer.
func (m Model) SparklineLen() int { return len(m.latencyRing) }

// View renders the dashboard.
func (m Model) View() string {
	width := m.dashboardWidth()
	rowHeights := m.dashboardRowContentHeights()
	sections := make([]string, 0, 7)

	if m.title != "" {
		sections = append(sections, m.title)
	}

	if row0 := m.renderTopRow(width, rowHeights[0]); row0 != "" {
		sections = append(sections, row0)
	}

	if rowHeights[1] >= 0 {
		sections = append(sections, renderPanel("Request Context", m.requestContextBody(), width, rowHeights[1]))
	}

	row2 := renderPanelRow(
		width,
		0.68,
		rowHeights[2],
		panelSpec{title: "Latency Trend", body: m.latencyBody(rowContentWidth(width, 0.68), rowHeights[2])},
		panelSpec{title: "Latency Stats / Health", body: m.latencyStatsBody()},
	)
	if row2 != "" {
		sections = append(sections, row2)
	}

	row3 := m.protocolPanel(width, rowHeights[3])
	if row3 != "" {
		sections = append(sections, row3)
	}

	row4 := renderPanelRow(
		width,
		0.5,
		rowHeights[4],
		panelSpec{title: "Endpoints", body: m.endpointsBody()},
		panelSpec{title: "Status Buckets", body: m.statusBucketsBody()},
	)
	if row4 != "" {
		sections = append(sections, row4)
	}

	return strings.Join(sections, "\n")
}

func (m Model) dashboardWidth() int {
	if m.width <= 0 {
		return defaultWidth
	}
	return m.width
}

func (m Model) dashboardHeight() int {
	if m.height <= 0 {
		return defaultHeight
	}
	return m.height
}

// stackedRowCost calculates total rendered lines for a row in stacked mode.
// Row 0 is hybrid: one boxed summary panel (rowHeight+2 for content+borders) plus a
// fixed rpsStripLineCount-line RPS strip.
// Rows 2 and 4 are paired: two panels rendered vertically (2 × (rowHeight+2)).
// Rows 1 and 3 are standalone: one panel (rowHeight+2).
// Returns 0 if the row is skipped (rowHeight < 0).
func stackedRowCost(rowIndex int, rowHeight int) int {
	if rowHeight < 0 {
		return 0 // Skipped row
	}
	if rowIndex == 0 {
		// Row 0: summary panel (h+2 with borders) + RPS strip (rpsStripLineCount lines fixed)
		return (rowHeight + 2) + rpsStripLineCount
	}
	isPaired := (rowIndex == 2 || rowIndex == 4)
	panelTotal := rowHeight + 2 // content + borders
	if isPaired {
		return 2 * panelTotal
	}
	return panelTotal
}

// budgetStackedDashboard allocates row heights for stacked mode using priority-based budgeting.
// Priority order: Summary/RPS (row 0) >= Request Context (row 1) > Latency/Stats (row 2) > Protocol (row 3) > Endpoints/Status (row 4).
// Request Context is allocated based on actual needed lines (reqContextNeeded) to preserve all fields.
// Lower-priority rows are dropped (negative height) when space is tight.
func budgetStackedDashboard(availableHeight, reqContextNeeded int) []int {
	const (
		// Minimum content lines per row (before borders)
		minSummaryContent = 0 // Can collapse to border-only
		// reqContextNeeded is passed as parameter based on actual content
		minLatencyContent = 0 // Can collapse to border-only
		// Protocol and Endpoints can be skipped entirely (negative height)
	)

	// Start with minimum allocation using actual needed Request Context height
	rowHeights := []int{
		minSummaryContent, // Row 0: Summary panel + RPS strip (summary boxed, strip inline)
		reqContextNeeded,  // Row 1: Request Context (standalone) - FIXED PRIORITY
		minLatencyContent, // Row 2: Latency + Stats (paired)
		-1,                // Row 3: Protocol (standalone) - initially skipped
		-1,                // Row 4: Endpoints + Status (paired) - initially skipped
	}

	// Calculate cost of current allocation
	cost := 0
	for i, h := range rowHeights {
		cost += stackedRowCost(i, h)
	}

	// Remaining space to distribute
	remaining := availableHeight - cost
	if remaining < 0 {
		// Baseline allocation is too large for available height
		// Drop Summary and Latency shells to reduce footprint
		rowHeights[0] = -1 // Skip Summary/RPS entirely
		rowHeights[2] = -1 // Skip Latency/Stats entirely

		// Recalculate cost after dropping shells
		cost = stackedRowCost(1, rowHeights[1]) // Only Request Context remains
		remaining = availableHeight - cost

		// If still too large, progressively clip Request Context
		// This handles extremely constrained terminals where even full RC won't fit
		if remaining < 0 {
			// Try to fit at least minimal Request Context (URL + a few fields)
			minimalRC := 5 // URL (2 wrapped) + Method/Protocol + 2 params
			if availableHeight >= stackedRowCost(1, minimalRC) {
				rowHeights[1] = minimalRC
			} else if availableHeight >= 2 {
				rowHeights[1] = max(0, availableHeight-2)
			} else {
				// Truly pathological case: even a panel shell will not fit, so skip it.
				rowHeights[1] = -1
			}
			return rowHeights
		}
	}

	// Priority 1: Restore Summary with at least 1 content line if we have room
	// (even if it was dropped from baseline due to tight space)
	if remaining >= 6 && rowHeights[0] < 0 {
		rowHeights[0] = 1 // summary panel (1 content + 2 borders) + rpsStripLineCount-line strip = 6 total lines
		remaining -= 6
	} else if remaining >= 1 && rowHeights[0] == 0 {
		// Incremental cost: summary panel gains 1 content line; strip height is fixed.
		rowHeights[0] = 1
		remaining -= 1
	}

	// Priority 2: Restore Latency with at least 1 content line if we have room
	if remaining >= 6 && rowHeights[2] < 0 {
		rowHeights[2] = 1 // 2 panels × (1 content + 2 borders) = 6 total lines
		remaining -= 6
	} else if remaining >= 2 && rowHeights[2] == 0 {
		rowHeights[2] = 1
		remaining -= 2
	}

	// Priority 2.5: Give Latency more content before restoring lower-priority panels
	// Latency should reach 3+ content lines before Protocol appears
	for remaining >= 2 && rowHeights[2] > 0 && rowHeights[2] < 3 {
		rowHeights[2]++
		remaining -= 2
	}

	// Priority 3: Add Protocol panel back if we have room (minimum 2 total lines)
	if remaining >= 2 {
		rowHeights[3] = 0 // 0 content + 2 borders = 2 total
		remaining -= 2
	}

	// Priority 4: Add Endpoints/Status panels back if we have room (minimum 4 lines total in stacked)
	if remaining >= 4 {
		rowHeights[4] = 0 // 0 content + 2 borders, x2 for paired = 4 total
		remaining -= 4
	}

	// Priority 5: Distribute remaining space to Summary and Latency proportionally.
	// Latency is a paired row (cost 2 per content line); Summary is a single panel
	// in row 0 (cost 1 per content line, strip height is fixed).
	// A full cycle costs 3 lines: 2 for Latency + 1 for Summary.
	for remaining >= 1 {
		grew := false

		// Grow only rows that are already present; restoring skipped rows requires
		// the explicit full-cost branches above.
		if remaining >= 2 && rowHeights[2] >= 0 {
			rowHeights[2]++
			remaining -= 2
			grew = true
		}
		// Give 1 line to Summary (1 content line; strip height is fixed)
		if remaining >= 1 && rowHeights[0] >= 0 {
			rowHeights[0]++
			remaining--
			grew = true
		}
		if !grew {
			break
		}
		// Stop if we can't afford another full cycle (Latency needs 2)
		if remaining < 3 {
			break
		}
	}

	// Priority 6: If still have space, add to Protocol and Endpoints
	if remaining >= 1 && rowHeights[3] >= 0 {
		rowHeights[3]++
		remaining--
	}
	if remaining >= 2 && rowHeights[4] >= 0 {
		rowHeights[4]++
		remaining -= 2
	}

	// Priority 7: Dump any remaining odd lines into Request Context (can't hurt)
	if remaining > 0 {
		rowHeights[1] += remaining
	}

	return rowHeights
}

func (m Model) dashboardRowContentHeights() []int {
	height := m.dashboardHeight()
	if m.title != "" {
		height--
	}

	// When width < splitThreshold, panels stack vertically instead of side-by-side.
	// Use principled budgeting to ensure Request Context detail survives at all heights.
	stackedMode := m.width < splitThreshold
	if stackedMode {
		reqContextNeeded := m.requestContextNeededHeight(m.width)
		return budgetStackedDashboard(height, reqContextNeeded)
	}

	// General case: use existing responsive priority logic for side-by-side layout
	totals := scaledHeights(height, []int{12, 14, 22, 12, 20}, 3)

	// Apply responsive priority for short terminals in side-by-side mode
	if height <= 26 {
		// Zero-sum reprioritization for short terminals
		// Priority order: summary/RPS > Request Context > Latency > Protocol > Endpoints/Status

		// For very short terminals (<=22), use aggressive reductions to preserve Request Context
		if height <= 22 {
			// Reduce Protocol Metrics to bare minimum (1 line content = 3 total with borders)
			if totals[3] > 3 {
				protocolReduction := totals[3] - 3
				totals[3] = 3
				totals[1] += protocolReduction
			}

			// Reduce Endpoints/Status to bare minimum (1 line content = 3 total with borders)
			if totals[4] > 3 {
				bottomReduction := totals[4] - 3
				totals[4] = 3
				totals[1] += bottomReduction
			}

			// Also reduce Latency Trend to give more room to Request Context structured detail
			minLatency := 4
			if totals[2] > minLatency {
				latencyReduction := totals[2] - minLatency
				totals[2] = minLatency
				totals[1] += latencyReduction
			}
		} else if height <= 24 {
			// height 23-24: aggressive but allow slightly more for lower panels
			// Reduce Protocol Metrics to minimum
			if totals[3] > 3 {
				protocolReduction := totals[3] - 3
				totals[3] = 3
				totals[1] += protocolReduction
			}

			// Reduce Endpoints/Status to minimum viable (2 lines content = 4 total)
			if totals[4] > 4 {
				bottomReduction := totals[4] - 4
				totals[4] = 4
				totals[1] += bottomReduction
			}
		} else {
			// height 25-26: gentler reprioritization
			// Reduce bottom row by 4, give to Request Context (+2) and Latency (+2)
			reduction := 4
			if totals[4] < reduction+4 {
				reduction = max(0, totals[4]-4)
			}
			totals[4] -= reduction

			if reduction >= 4 {
				totals[1] += 2
				totals[2] += 2
			} else if reduction >= 2 {
				totals[1] += 1
				totals[2] += 1
			}
		}
	}
	rowHeights := make([]int, len(totals))
	for i, total := range totals {
		rowHeights[i] = max(1, total-2)
	}
	return rowHeights
}

func (m Model) summaryBody() string {
	var lines []string
	if !m.requestContext.HasContent() {
		lines = append(lines, m.header...)
	}
	lines = append(lines, fmt.Sprintf(
		"Elapsed: %s | Total: %d | Success Rate: %.1f%%",
		m.elapsedDisplay(),
		m.totalRequests(),
		m.successRate(),
	))
	if progress := m.requestProgressLine(); progress != "" {
		lines = append(lines, progress)
	}
	if progress := m.patternProgressLine(); progress != "" {
		lines = append(lines, progress)
	}
	return strings.Join(lines, "\n")
}

func (m Model) rpsStripBody(contentWidth int) string {
	target := m.requestContext.TargetRPS
	scaleMax := target
	if scaleMax <= 0 {
		scaleMax = m.maxRPSSeen()
		if m.currentRPS() > scaleMax {
			scaleMax = m.currentRPS()
		}
	}

	current := m.currentRPS()
	var style lipgloss.Style
	if target > 0 {
		ratio := current / target
		switch {
		case ratio <= 1.0:
			style = okStyle
		case ratio <= 1.2:
			style = warnStyle
		default:
			style = errStyle
		}
	} else {
		style = lipgloss.NewStyle()
	}

	meta := "Requests / sec | Rolling max scale"
	if target > 0 {
		meta = fmt.Sprintf("Requests / sec | Target %.2f/s", target)
	}
	value := style.Render(fmt.Sprintf("%.2f", current))
	bar := style.Render(widgets.TargetBar(contentWidth, current, target, scaleMax))
	return strings.Join([]string{meta, value, bar}, "\n")
}

func (m Model) requestContextBody() string {
	lines := []string{m.requestContext.RawURL}
	if m.requestContext.Method != "" || m.requestContext.Protocol != "" {
		lines = append(lines, fmt.Sprintf("Method: %s | Protocol: %s", m.requestContext.Method, m.requestContext.Protocol))
	}

	// Compact execution params onto fewer lines
	if len(m.requestContext.Params) > 0 {
		var paramParts []string
		for _, param := range m.requestContext.Params {
			paramParts = append(paramParts, fmt.Sprintf("%s: %s", param.Label, param.Value))
		}
		// Join with " | " and let wrapping handle line breaks
		lines = append(lines, strings.Join(paramParts, " | "))
	}

	// Compact query params onto fewer lines
	if len(m.requestContext.QueryParams) > 0 {
		var qpParts []string
		for _, param := range m.requestContext.QueryParams {
			qpParts = append(qpParts, fmt.Sprintf("%s=%s", param.Label, param.Value))
		}
		// Join with " | " and let wrapping handle line breaks
		lines = append(lines, strings.Join(qpParts, " | "))
	}

	return strings.Join(lines, "\n")
}

// requestContextNeededHeight calculates the actual lines needed for Request Context
// after text wrapping, based on the panel's content width.
func (m Model) requestContextNeededHeight(width int) int {
	contentWidth := panelContentWidth(width)
	body := m.requestContextBody()
	wrappedLines := wrapBody(body, contentWidth)
	return len(wrappedLines)
}

func (m Model) latencyBody(contentWidth, contentHeight int) string {
	markers := []widgets.Marker{
		{Label: "P50", Value: m.currentP50(), Rune: '·'},
		{Label: "P95", Value: m.currentP95(), Rune: '─'},
	}
	if slo := m.requestContext.LatencySLOMs; slo > 0 {
		markers = append(markers, widgets.Marker{Label: "SLO", Value: slo, Rune: '═'})
	}
	return widgets.TrendChart(m.latencyRing, max(1, contentWidth), max(1, contentHeight), markers)
}

func (m Model) currentP50() float64 {
	if m.stats != nil && m.stats.P50LatencyMs > 0 {
		return m.stats.P50LatencyMs
	}
	return m.latest.P50LatencyMs
}

func (m Model) currentP95() float64 {
	if m.stats != nil && m.stats.P95LatencyMs > 0 {
		return m.stats.P95LatencyMs
	}
	return m.latest.P95LatencyMs
}

func (m Model) latencyStatsBody() string {
	minLatency := 0.0
	meanLatency := 0.0
	p50Latency := m.latest.P50LatencyMs
	p90Latency := 0.0
	p95Latency := m.latest.P95LatencyMs
	p99Latency := m.latest.P99LatencyMs
	if m.stats != nil {
		minLatency = m.stats.MinLatencyMs
		meanLatency = m.stats.MeanLatencyMs
		p50Latency = m.stats.P50LatencyMs
		p90Latency = m.stats.P90LatencyMs
		p95Latency = m.stats.P95LatencyMs
		p99Latency = m.stats.P99LatencyMs
	}
	return fmt.Sprintf(
		"Min:  %.2fms\nMean: %.2fms\nP50:  %.2fms\nP90:  %.2fms\nP95:  %.2fms\nP99:  %.2fms",
		minLatency,
		meanLatency,
		p50Latency,
		p90Latency,
		p95Latency,
		p99Latency,
	)
}

func (m Model) protocolPanel(width, minContentHeight int) string {
	// Skip this panel entirely if minContentHeight is negative (extreme height constraints)
	if minContentHeight < 0 {
		return ""
	}

	title := "Protocol Metrics"
	body := "No protocol-specific metrics"
	if m.loadPattern != nil {
		title = "Load Pattern: " + m.loadPattern.Name
		body = m.loadPatternBody()
	} else if block := strings.TrimRight(widgets.ProtocolMetricsBlock(m.protocolMetrics, 4), "\n"); block != "" {
		body = block
	}
	return renderPanel(title, body, width, minContentHeight)
}

func (m Model) loadPatternBody() string {
	if m.loadPattern == nil {
		return ""
	}
	rendered := RenderLoadPatternStrip(m.loadPattern, m.elapsed, max(10, min(60, m.dashboardWidth()-4)))
	prefix := "Load Pattern: " + m.loadPattern.Name + "\n"
	return strings.TrimPrefix(rendered, prefix)
}

func (m Model) endpointsBody() string {
	if len(m.endpoints) == 0 {
		return "No endpoint data"
	}
	return strings.TrimRight(widgets.EndpointTable(m.endpoints, 10), "\n")
}

func (m Model) statusBucketsBody() string {
	rows := strings.TrimRight(widgets.StatusBucketsTable(m.statusBuckets, 10), "\n")
	if rows == "" {
		return "No failures"
	}
	return rows
}

func (m Model) elapsedDisplay() time.Duration {
	if m.elapsed > 0 {
		return m.elapsed.Round(time.Second)
	}
	if m.stats != nil && m.stats.Duration > 0 {
		return m.stats.Duration.Round(time.Second)
	}
	return 0
}

func (m Model) totalRequests() int64 {
	if m.stats != nil && m.stats.Total > 0 {
		return m.stats.Total
	}
	return m.latest.TotalRequests
}

func (m Model) successfulRequests() int64 {
	if m.stats != nil {
		return m.stats.Successes
	}
	return m.latest.SuccessfulRequests
}

func (m Model) failedRequests() int64 {
	if m.stats != nil {
		return m.stats.Failures
	}
	return m.latest.Errors
}

func (m Model) currentRPS() float64 {
	if m.latest.CurrentRPS > 0 {
		return m.latest.CurrentRPS
	}
	if m.stats != nil {
		return m.stats.RequestsPerSec
	}
	return 0
}

func (m Model) maxRPSSeen() float64 {
	maxSeen := 0.0
	for _, sample := range m.rpsRing {
		if sample > maxSeen {
			maxSeen = sample
		}
	}
	return maxSeen
}

func (m Model) successRate() float64 {
	total := m.totalRequests()
	if total == 0 {
		return 0
	}
	return float64(m.successfulRequests()) / float64(total) * 100
}

func (m Model) requestProgressLine() string {
	if m.total <= 0 {
		return ""
	}

	pct := float64(m.totalRequests()) / float64(m.total) * 100
	if pct > 100 {
		pct = 100
	}
	return fmt.Sprintf("Progress: %.0f%% (%d/%d)", pct, m.totalRequests(), m.total)
}

func (m Model) patternProgressLine() string {
	if m.loadPattern == nil || m.loadPattern.Total <= 0 {
		return ""
	}

	pct := float64(m.elapsed) / float64(m.loadPattern.Total) * 100
	if pct > 100 {
		pct = 100
	}
	return fmt.Sprintf("Pattern Progress: %.0f%%", pct)
}

func latencySample(msg SnapshotMsg) float64 {
	switch {
	case msg.Stats != nil && msg.Stats.P95LatencyMs > 0:
		return msg.Stats.P95LatencyMs
	case msg.Snap.P95LatencyMs > 0:
		return msg.Snap.P95LatencyMs
	case msg.Stats != nil && msg.Stats.MeanLatencyMs > 0:
		return msg.Stats.MeanLatencyMs
	case msg.Snap.P50LatencyMs > 0:
		return msg.Snap.P50LatencyMs
	default:
		return 0
	}
}

func rpsSample(msg SnapshotMsg) float64 {
	if msg.Snap.CurrentRPS > 0 {
		return msg.Snap.CurrentRPS
	}
	if msg.Stats != nil {
		return msg.Stats.RequestsPerSec
	}
	return 0
}

func renderPanelRow(totalWidth int, ratio float64, minContentHeight int, left, right panelSpec) string {
	// Skip this row entirely if minContentHeight is negative (extreme height constraints)
	if minContentHeight < 0 {
		return ""
	}

	leftWidth, rightWidth, split := rowWidths(totalWidth, ratio)
	if !split {
		return renderPanel(left.title, left.body, totalWidth, minContentHeight) + "\n" + renderPanel(right.title, right.body, totalWidth, minContentHeight)
	}

	contentHeight := max(0, minContentHeight)
	return joinBlocks(
		strings.Repeat(" ", panelGap),
		renderPanel(left.title, left.body, leftWidth, contentHeight),
		renderPanel(right.title, right.body, rightWidth, contentHeight),
	)
}

func (m Model) renderTopRow(totalWidth, minSummaryHeight int) string {
	if minSummaryHeight < 0 {
		return ""
	}
	leftWidth, rightWidth, split := rowWidths(totalWidth, 0.58)
	// stripContentWidth is narrower than rightWidth by the panel inner-padding amount so
	// the bar/content aligns consistently; the strip itself fills the full rightWidth column.
	stripContentWidth := rightPanelContentWidth(totalWidth, 0.58)
	strip := renderRPSStrip(rightWidth, m.rpsStripBody(stripContentWidth))
	if !split {
		return renderPanel("Run Summary", m.summaryBody(), totalWidth, minSummaryHeight) + "\n" + strip
	}
	summary := renderPanel("Run Summary", m.summaryBody(), leftWidth, max(0, minSummaryHeight))
	return joinBlocks(strings.Repeat(" ", panelGap), summary, strip)
}

func renderRPSStrip(width int, body string) string {
	contentWidth := max(1, width)
	parts := strings.Split(body, "\n")
	lines := make([]string, 0, rpsStripLineCount)
	for i := 0; i < rpsStripLineCount; i++ {
		line := ""
		if i < len(parts) {
			wrapped := fitPanelLines(wrapBody(parts[i], contentWidth), contentWidth, 1)
			if len(wrapped) > 0 {
				line = wrapped[0]
			}
		}
		lines = append(lines, padRight(line, contentWidth))
	}
	return strings.Join(lines, "\n")
}

func rowWidths(totalWidth int, ratio float64) (int, int, bool) {
	if totalWidth < splitThreshold {
		return totalWidth, totalWidth, false
	}
	usable := totalWidth - panelGap
	leftWidth := int(float64(usable) * ratio)
	rightWidth := usable - leftWidth
	if leftWidth < 20 || rightWidth < 20 {
		return totalWidth, totalWidth, false
	}
	return leftWidth, rightWidth, true
}

func rowContentWidth(totalWidth int, ratio float64) int {
	leftWidth, _, split := rowWidths(totalWidth, ratio)
	if !split {
		return panelContentWidth(totalWidth)
	}
	return panelContentWidth(leftWidth)
}

func rightPanelContentWidth(totalWidth int, ratio float64) int {
	_, rightWidth, split := rowWidths(totalWidth, ratio)
	if !split {
		return panelContentWidth(totalWidth)
	}
	return panelContentWidth(rightWidth)
}

func panelContentHeight(body string, width int) int {
	return len(wrapBody(body, panelContentWidth(width)))
}

func panelContentWidth(width int) int {
	return max(1, width-4)
}

func renderPanel(title, body string, width, minContentHeight int) string {
	if width < 6 {
		width = 6
	}
	if minContentHeight < 0 {
		minContentHeight = 0
	}
	innerWidth := width - 2
	contentWidth := panelContentWidth(width)
	lines := fitPanelLines(wrapBody(body, contentWidth), contentWidth, minContentHeight)
	if len(lines) < minContentHeight {
		for len(lines) < minContentHeight {
			lines = append(lines, "")
		}
	}

	titleText := " " + title + " "
	titleWidth := lipgloss.Width(titleText)
	if titleWidth > innerWidth {
		titleText = truncate(titleText, innerWidth)
		titleWidth = lipgloss.Width(titleText)
	}

	var b strings.Builder
	b.WriteString("┌")
	b.WriteString(titleText)
	b.WriteString(strings.Repeat("─", max(0, innerWidth-titleWidth)))
	b.WriteString("┐\n")
	for _, line := range lines {
		b.WriteString("│ ")
		b.WriteString(padRight(line, contentWidth))
		b.WriteString(" │\n")
	}
	b.WriteString("└")
	b.WriteString(strings.Repeat("─", innerWidth))
	b.WriteString("┘")
	return b.String()
}

func wrapBody(body string, width int) []string {
	if width <= 0 {
		return []string{""}
	}
	body = strings.TrimRight(body, "\n")
	if body == "" {
		return []string{""}
	}
	rendered := lipgloss.NewStyle().Width(width).Render(body)
	lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func fitPanelLines(lines []string, width, maxLines int) []string {
	if maxLines <= 0 {
		return nil
	}
	if len(lines) <= maxLines {
		return lines
	}
	clipped := append([]string{}, lines[:maxLines]...)
	clipped[maxLines-1] = ellipsize(clipped[maxLines-1], width)
	return clipped
}

func padRight(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) >= width {
		return truncate(s, width)
	}
	return s + strings.Repeat(" ", width-lipgloss.Width(s))
}

func ellipsize(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if width == 1 {
		return "…"
	}
	return padRight(truncate(s, width-1), width-1) + "…"
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		next := b.String() + string(r)
		if lipgloss.Width(next) > width {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

func joinBlocks(gap string, blocks ...string) string {
	type blockInfo struct {
		lines []string
		width int
	}

	info := make([]blockInfo, 0, len(blocks))
	maxLines := 0
	for _, block := range blocks {
		lines := strings.Split(block, "\n")
		blockWidth := 0
		for _, line := range lines {
			blockWidth = max(blockWidth, lipgloss.Width(line))
		}
		info = append(info, blockInfo{lines: lines, width: blockWidth})
		maxLines = max(maxLines, len(lines))
	}

	var b strings.Builder
	for i := 0; i < maxLines; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		for j, block := range info {
			if j > 0 {
				b.WriteString(gap)
			}
			if i < len(block.lines) {
				b.WriteString(padRight(block.lines[i], block.width))
				continue
			}
			b.WriteString(strings.Repeat(" ", block.width))
		}
	}
	return b.String()
}

func scaledHeights(total int, weights []int, minHeight int) []int {
	out := make([]int, len(weights))
	if len(weights) == 0 {
		return out
	}
	if minHeight < 0 {
		minHeight = 0
	}

	minTotal := len(weights) * minHeight
	if total <= minTotal {
		for i := range out {
			out[i] = minHeight
		}
		return out
	}

	for i := range out {
		out[i] = minHeight
	}

	type remainder struct {
		idx int
		rem int
	}

	remaining := total - minTotal
	weightSum := 0
	for _, weight := range weights {
		weightSum += weight
	}

	remainders := make([]remainder, 0, len(weights))
	used := 0
	for i, weight := range weights {
		extra := remaining * weight / weightSum
		out[i] += extra
		used += extra
		remainders = append(remainders, remainder{
			idx: i,
			rem: remaining * weight % weightSum,
		})
	}

	for remainingCells := remaining - used; remainingCells > 0; remainingCells-- {
		best := 0
		for i := 1; i < len(remainders); i++ {
			if remainders[i].rem > remainders[best].rem {
				best = i
			}
		}
		out[remainders[best].idx]++
		remainders[best].rem = -1
	}

	return out
}
