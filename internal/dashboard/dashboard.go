package dashboard

import (
	"context"
	"fmt"
	"sync"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"

	"github.com/torosent/crankfire/internal/metrics"
)

// Dashboard renders a live terminal UI for load test metrics.
type Dashboard struct {
	collector *metrics.Collector
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.Mutex

	// Widgets
	grid            *ui.Grid
	latencySparkle  *widgets.SparklineGroup
	latencyPara     *widgets.Paragraph
	rpsGauge        *widgets.Gauge
	errorList       *widgets.List
	summaryPara     *widgets.Paragraph
	metricsPara     *widgets.Paragraph
	latencyHistory  []float64
	rpsHistory      []float64
	errorBreakdown  map[string]int
	lastUpdateTime  time.Time
	startTime       time.Time
}

// New creates a new Dashboard.
func New(collector *metrics.Collector) (*Dashboard, error) {
	if err := ui.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize termui: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	d := &Dashboard{
		collector:      collector,
		ctx:            ctx,
		cancel:         cancel,
		latencyHistory: make([]float64, 0, 100),
		rpsHistory:     make([]float64, 0, 100),
		errorBreakdown: make(map[string]int),
		startTime:      time.Now(),
		lastUpdateTime: time.Now(),
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

	// Error List
	d.errorList = widgets.NewList()
	d.errorList.Title = "Error Breakdown"
	d.errorList.Rows = []string{"No errors"}
	d.errorList.TextStyle = ui.NewStyle(ui.ColorYellow)
	d.errorList.BorderStyle.Fg = ui.ColorCyan

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

}

// setupGrid configures the layout grid.
func (d *Dashboard) setupGrid() {
	termWidth, termHeight := ui.TerminalDimensions()

	d.grid = ui.NewGrid()
	d.grid.SetRect(0, 0, termWidth, termHeight)

	d.grid.Set(
		ui.NewRow(0.16,
			ui.NewCol(1.0, d.summaryPara),
		),
		ui.NewRow(0.20,
			ui.NewCol(0.5, d.rpsGauge),
			ui.NewCol(0.5, d.metricsPara),
		),
		ui.NewRow(0.30,
			ui.NewCol(0.65, d.latencySparkle),
			ui.NewCol(0.35, d.latencyPara),
		),
		ui.NewRow(0.34,
			ui.NewCol(1.0, d.errorList),
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
	ui.Close()
	// Give terminal time to restore
	time.Sleep(100 * time.Millisecond)
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
				return
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
	d.summaryPara.Text = fmt.Sprintf(
		"Elapsed: %s | Total: %d | Success Rate: %.1f%%",
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

	if len(stats.Errors) == 0 {
		d.errorList.Rows = []string{"[No errors](fg:green)"}
	} else {
		rows := make([]string, 0, len(stats.Errors))
		for errType, count := range stats.Errors {
			rows = append(rows, fmt.Sprintf("[%s:](fg:red) %d", errType, count))
		}
		d.errorList.Rows = rows
	}
}

// render draws all widgets to the screen.
func (d *Dashboard) render() {
	d.mu.Lock()
	defer d.mu.Unlock()

	ui.Render(d.grid)
}
