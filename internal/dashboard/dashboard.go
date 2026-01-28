package dashboard

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"

	"github.com/torosent/crankfire/internal/metrics"
)

// TestConfig holds load test configuration parameters for display.
type TestConfig struct {
	TargetURL   string        // Full target URL
	Concurrency int           // Number of concurrent workers
	Duration    time.Duration // Test duration (0 = unlimited)
	Total       int           // Total requests to execute (0 = unlimited)
	Rate        int           // Requests per second (0 = unlimited)
	Timeout     time.Duration // Request timeout
	Retries     int           // Number of retries
	Protocol    string        // Protocol (http, websocket, sse, grpc)
	Method      string        // HTTP method
	ConfigFile  string        // Path to config file if used
}

// Dashboard renders a live terminal UI for load test metrics.
type Dashboard struct {
	collector    *metrics.Collector
	ctx          context.Context
	cancel       context.CancelFunc
	shutdownFunc func()
	wg           sync.WaitGroup
	mu           sync.Mutex

	// Widgets
	grid           *ui.Grid
	latencySparkle *widgets.SparklineGroup
	latencyPara    *widgets.Paragraph
	rpsGauge       *widgets.Gauge
	errorList      *widgets.List
	endpointList   *widgets.List
	summaryPara    *widgets.Paragraph
	metricsPara    *widgets.Paragraph
	protocolPara   *widgets.Paragraph
	latencyHistory []float64
	rpsHistory     []float64
	lastUpdateTime time.Time
	startTime      time.Time
	targetURL      string
	testDuration   time.Duration
	testConfig     TestConfig
}

// New creates a new Dashboard.
func New(collector *metrics.Collector, cfg TestConfig, shutdownFunc func()) (*Dashboard, error) {
	if err := ui.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize termui: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	d := &Dashboard{
		collector:      collector,
		ctx:            ctx,
		cancel:         cancel,
		shutdownFunc:   shutdownFunc,
		latencyHistory: make([]float64, 0, 100),
		rpsHistory:     make([]float64, 0, 100),
		startTime:      time.Now(),
		lastUpdateTime: time.Now(),
		targetURL:      cfg.TargetURL,
		testConfig:     cfg,
	}

	d.initWidgets()
	d.setupGrid()

	return d, nil
}

// initWidgets initializes all dashboard widgets.
func (d *Dashboard) initWidgets() {
	// Latency Sparkline
	sparkline := widgets.NewSparkline()
	sparkline.Title = "Latency (ms)"
	sparkline.LineColor = ui.ColorGreen
	sparkline.Data = []float64{0}

	d.latencySparkle = widgets.NewSparklineGroup(sparkline)
	d.latencySparkle.Title = "Real-time Latency"
	d.latencySparkle.BorderStyle.Fg = ui.ColorCyan

	// Latency Metrics Paragraph
	d.latencyPara = widgets.NewParagraph()
	d.latencyPara.Title = "Latency Stats"
	d.latencyPara.Text = "Min: 0ms\nMean: 0ms\nP50: 0ms\nP90: 0ms\nP99: 0ms"
	d.latencyPara.BorderStyle.Fg = ui.ColorCyan

	// RPS Gauge
	d.rpsGauge = widgets.NewGauge()
	d.rpsGauge.Title = "Requests Per Second"
	d.rpsGauge.Percent = 0
	d.rpsGauge.BarColor = ui.ColorBlue
	d.rpsGauge.BorderStyle.Fg = ui.ColorCyan
	d.rpsGauge.LabelStyle = ui.NewStyle(ui.ColorWhite)

	// Status Bucket List
	d.errorList = widgets.NewList()
	d.errorList.Title = "Status Buckets"
	d.errorList.Rows = []string{"No failures"}
	d.errorList.TextStyle = ui.NewStyle(ui.ColorYellow)
	d.errorList.BorderStyle.Fg = ui.ColorCyan

	// Endpoint List
	d.endpointList = widgets.NewList()
	d.endpointList.Title = "Endpoints"
	d.endpointList.Rows = []string{"Awaiting data"}
	d.endpointList.TextStyle = ui.NewStyle(ui.ColorCyan)
	d.endpointList.BorderStyle.Fg = ui.ColorCyan

	// Summary Paragraph
	d.summaryPara = widgets.NewParagraph()
	d.summaryPara.Title = "Test Summary"
	d.summaryPara.Text = "Initializing..."
	d.summaryPara.BorderStyle.Fg = ui.ColorCyan

	// Metrics Paragraph (plain text summary)
	d.metricsPara = widgets.NewParagraph()
	d.metricsPara.Title = "Metrics"
	d.metricsPara.Text = "Waiting for data..."
	d.metricsPara.BorderStyle.Fg = ui.ColorCyan

	// Protocol Metrics Paragraph
	d.protocolPara = widgets.NewParagraph()
	d.protocolPara.Title = "Protocol Metrics"
	d.protocolPara.Text = "No protocol data"
	d.protocolPara.TextStyle = ui.NewStyle(ui.ColorGreen)
	d.protocolPara.BorderStyle.Fg = ui.ColorCyan

}

// setupGrid configures the layout grid.
func (d *Dashboard) setupGrid() {
	termWidth, termHeight := ui.TerminalDimensions()

	d.grid = ui.NewGrid()
	d.grid.SetRect(0, 0, termWidth, termHeight)

	d.grid.Set(
		ui.NewRow(0.14,
			ui.NewCol(1.0, d.summaryPara),
		),
		ui.NewRow(0.18,
			ui.NewCol(0.5, d.rpsGauge),
			ui.NewCol(0.5, d.metricsPara),
		),
		ui.NewRow(0.26,
			ui.NewCol(0.65, d.latencySparkle),
			ui.NewCol(0.35, d.latencyPara),
		),
		ui.NewRow(0.14,
			ui.NewCol(1.0, d.protocolPara),
		),
		ui.NewRow(0.28,
			ui.NewCol(0.5, d.endpointList),
			ui.NewCol(0.5, d.errorList),
		),
	)
}

// Start begins the dashboard update loop.
func (d *Dashboard) Start() {
	d.wg.Add(1)
	go d.run()
}

// Stop stops the dashboard and cleans up.
func (d *Dashboard) Stop() {
	d.cancel()
	d.wg.Wait()
	d.testDuration = time.Since(d.startTime)
	ui.Close()
	// Give terminal time to restore
	time.Sleep(100 * time.Millisecond)
}

// GetFinalStats returns the final statistics after the dashboard has stopped.
func (d *Dashboard) GetFinalStats() metrics.Stats {
	return d.collector.Stats(d.testDuration)
}

// run is the main dashboard update loop.
func (d *Dashboard) run() {
	defer d.wg.Done()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	uiEvents := ui.PollEvents()

	d.render()

	for {
		select {
		case <-d.ctx.Done():
			// Drain any remaining events
			for len(uiEvents) > 0 {
				<-uiEvents
			}
			return
		case e := <-uiEvents:
			// Check if context is done to avoid blocking
			select {
			case <-d.ctx.Done():
				return
			default:
			}

			switch e.ID {
			case "q", "<C-c>":
				if d.shutdownFunc != nil {
					d.shutdownFunc()
				}
				// Do not return here; wait for Stop() to cancel context
			case "<Resize>":
				payload := e.Payload.(ui.Resize)
				d.grid.SetRect(0, 0, payload.Width, payload.Height)
				ui.Clear()
				d.render()
			}
		case <-ticker.C:
			d.update()
			d.render()
		}
	}
}

// update refreshes all widget data from the collector.
func (d *Dashboard) update() {
	d.mu.Lock()
	defer d.mu.Unlock()

	elapsed := time.Since(d.startTime)
	stats := d.collector.Stats(elapsed)

	// Update latency history for sparkline
	if stats.MeanLatency > 0 {
		latencyMs := stats.MeanLatencyMs
		d.latencyHistory = append(d.latencyHistory, latencyMs)
		if len(d.latencyHistory) > 100 {
			d.latencyHistory = d.latencyHistory[1:]
		}
		d.latencySparkle.Sparklines[0].Data = d.latencyHistory
		// Update sparkline title with current latency values
		d.latencySparkle.Title = fmt.Sprintf(
			"Real-time Latency | Current: %.2fms | Min: %.2fms | Max: %.2fms",
			latencyMs,
			stats.MinLatencyMs,
			stats.MaxLatencyMs,
		)
	}

	currentRPS := stats.RequestsPerSec
	maxRPS := 100.0
	if currentRPS > maxRPS {
		maxRPS = currentRPS
	}
	rpsPercent := int((currentRPS / maxRPS) * 100)
	if rpsPercent > 100 {
		rpsPercent = 100
	}
	d.rpsGauge.Percent = rpsPercent
	d.rpsGauge.Label = fmt.Sprintf("%.1f RPS", currentRPS)

	successRate := 0.0
	if stats.Total > 0 {
		successRate = (float64(stats.Successes) / float64(stats.Total)) * 100
	}

	// Build test parameters line
	params := d.formatTestParams()

	d.summaryPara.Text = fmt.Sprintf(
		"Target: %s\n%s\nElapsed: %s | Total: %d | Success Rate: %.1f%%",
		d.targetURL,
		params,
		elapsed.Round(time.Second),
		stats.Total,
		successRate,
	)

	d.metricsPara.Text = fmt.Sprintf(
		"Total Requests:    %d\nSuccessful:        %d\nFailed:            %d\nCurrent RPS:       %.2f\nSuccess Rate:      %.1f%%\nMin Latency:       %.2fms\nMean Latency:      %.2fms\nP50/P90/P99:       %.2f / %.2f / %.2f ms",
		stats.Total,
		stats.Successes,
		stats.Failures,
		currentRPS,
		successRate,
		stats.MinLatencyMs,
		stats.MeanLatencyMs,
		stats.P50LatencyMs,
		stats.P90LatencyMs,
		stats.P99LatencyMs,
	)

	d.latencyPara.Text = fmt.Sprintf(
		"Min:  %.2fms\nMean: %.2fms\nP50:  %.2fms\nP90:  %.2fms\nP99:  %.2fms",
		stats.MinLatencyMs,
		stats.MeanLatencyMs,
		stats.P50LatencyMs,
		stats.P90LatencyMs,
		stats.P99LatencyMs,
	)

	d.errorList.Rows = formatStatusListRows(stats.StatusBuckets)

	d.updateEndpointList(stats)
	d.updateProtocolMetrics(stats)
}

// render draws all widgets to the screen.
func (d *Dashboard) render() {
	d.mu.Lock()
	defer d.mu.Unlock()

	ui.Render(d.grid)
}

func (d *Dashboard) updateEndpointList(stats metrics.Stats) {
	if len(stats.Endpoints) == 0 {
		d.endpointList.Rows = []string{"[No endpoint data](fg:green)"}
		return
	}
	type endpointRow struct {
		name string
		stat metrics.EndpointStats
	}
	rows := make([]endpointRow, 0, len(stats.Endpoints))
	for name, stat := range stats.Endpoints {
		rows = append(rows, endpointRow{name: name, stat: stat})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].stat.Total == rows[j].stat.Total {
			return rows[i].name < rows[j].name
		}
		return rows[i].stat.Total > rows[j].stat.Total
	})
	formatted := make([]string, 0, len(rows))
	for _, entry := range rows {
		share := 0.0
		if stats.Total > 0 {
			share = (float64(entry.stat.Total) / float64(stats.Total)) * 100
		}
		statusSummary := summarizeStatusBuckets(entry.stat.StatusBuckets, 2)
		if statusSummary == "" {
			statusSummary = "Status n/a"
		} else {
			statusSummary = "Status " + statusSummary
		}
		formatted = append(formatted, fmt.Sprintf("[%s](fg:cyan) | %5.1f%% | RPS %5.1f | P99 %5.1fms | Err %d | %s",
			entry.name,
			share,
			entry.stat.RequestsPerSec,
			entry.stat.P99LatencyMs,
			entry.stat.Failures,
			statusSummary,
		))
	}
	d.endpointList.Rows = formatted
}

func (d *Dashboard) updateProtocolMetrics(stats metrics.Stats) {
	if len(stats.ProtocolMetrics) == 0 {
		d.protocolPara.Text = "[No protocol-specific metrics](fg:green)"
		return
	}

	// Sort protocols alphabetically
	protocols := make([]string, 0, len(stats.ProtocolMetrics))
	for protocol := range stats.ProtocolMetrics {
		protocols = append(protocols, protocol)
	}
	sort.Strings(protocols)

	// Build formatted output
	lines := make([]string, 0)
	for _, protocol := range protocols {
		metrics := stats.ProtocolMetrics[protocol]
		lines = append(lines, fmt.Sprintf("[%s:](fg:cyan,mod:bold)", protocol))

		// Sort metric keys
		keys := make([]string, 0, len(metrics))
		for key := range metrics {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		// Format metrics - limit to 4 per protocol to fit on screen
		count := 0
		for _, key := range keys {
			if count >= 4 {
				break
			}
			value := metrics[key]
			lines = append(lines, fmt.Sprintf("  [%s:](fg:white) [%v](fg:yellow)", key, formatMetricValue(value)))
			count++
		}
	}

	d.protocolPara.Text = joinLines(lines)
}

func formatMetricValue(value interface{}) string {
	switch v := value.(type) {
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		if v > 1000 {
			return fmt.Sprintf("%.0f", v)
		}
		return fmt.Sprintf("%.2f", v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	result := lines[0]
	for i := 1; i < len(lines); i++ {
		result += "\n" + lines[i]
	}
	return result
}

func formatStatusListRows(buckets map[string]map[string]int) []string {
	rows := metrics.FlattenStatusBuckets(buckets)
	if len(rows) == 0 {
		return []string{"[No failures](fg:green)"}
	}
	maxRows := len(rows)
	if maxRows > 10 {
		maxRows = 10
	}
	formatted := make([]string, 0, maxRows)
	for i := 0; i < maxRows; i++ {
		row := rows[i]
		formatted = append(formatted, fmt.Sprintf("[%s %s](fg:red) %d", strings.ToUpper(row.Protocol), row.Code, row.Count))
	}
	return formatted
}

func summarizeStatusBuckets(buckets map[string]map[string]int, limit int) string {
	rows := metrics.FlattenStatusBuckets(buckets)
	if len(rows) == 0 {
		return ""
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		parts = append(parts, fmt.Sprintf("%s %s x%d", strings.ToUpper(row.Protocol), row.Code, row.Count))
	}
	return strings.Join(parts, ", ")
}

// formatTestParams formats the test configuration parameters for display.
func (d *Dashboard) formatTestParams() string {
	var parts []string

	// Protocol (only show if non-default)
	if d.testConfig.Protocol != "" && d.testConfig.Protocol != "http" {
		parts = append(parts, fmt.Sprintf("Protocol: %s", d.testConfig.Protocol))
	}

	// Method (for HTTP)
	if d.testConfig.Method != "" && d.testConfig.Method != "GET" {
		parts = append(parts, fmt.Sprintf("Method: %s", d.testConfig.Method))
	}

	// Concurrency
	if d.testConfig.Concurrency > 0 {
		parts = append(parts, fmt.Sprintf("Workers: %d", d.testConfig.Concurrency))
	}

	// Rate
	if d.testConfig.Rate > 0 {
		parts = append(parts, fmt.Sprintf("Rate: %d/s", d.testConfig.Rate))
	} else {
		parts = append(parts, "Rate: unlimited")
	}

	// Duration
	if d.testConfig.Duration > 0 {
		parts = append(parts, fmt.Sprintf("Duration: %s", d.testConfig.Duration))
	}

	// Total
	if d.testConfig.Total > 0 {
		parts = append(parts, fmt.Sprintf("Total: %d", d.testConfig.Total))
	}

	// Timeout
	if d.testConfig.Timeout > 0 {
		parts = append(parts, fmt.Sprintf("Timeout: %s", d.testConfig.Timeout))
	}

	// Retries (only show if set)
	if d.testConfig.Retries > 0 {
		parts = append(parts, fmt.Sprintf("Retries: %d", d.testConfig.Retries))
	}

	// Config file (only show if used)
	if d.testConfig.ConfigFile != "" {
		parts = append(parts, fmt.Sprintf("Config: %s", d.testConfig.ConfigFile))
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " | ")
}
