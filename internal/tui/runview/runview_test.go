package runview_test

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/tui/runview"
	"github.com/torosent/crankfire/internal/tui/widgets"
)

// TestMain sets up environment for lipgloss color rendering in tests.
// CLICOLOR_FORCE=1 ensures ANSI color codes are emitted even in non-TTY.
func TestMain(m *testing.M) {
	os.Setenv("CLICOLOR_FORCE", "1")
	os.Setenv("NO_COLOR", "")
	os.Exit(m.Run())
}

func TestRunViewRendersFromSnapshot(t *testing.T) {
	rv := runview.New(runview.Options{Title: "Checkout API smoke", Total: 50000})
	rv1, _ := rv.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	rv2, _ := rv1.Update(runview.SnapshotMsg{Snap: metrics.DataPoint{TotalRequests: 17250, CurrentRPS: 487}})
	out := rv2.View()
	for _, want := range []string{"Checkout API smoke", "17", "487", "Requests / sec", "Latency Stats"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestSparklineGrowsWithSnapshots(t *testing.T) {
	rv := runview.New(runview.Options{Title: "x", Total: 100})
	var cur runview.Model = rv
	for i := 0; i < 130; i++ {
		cur, _ = cur.Update(runview.SnapshotMsg{Snap: metrics.DataPoint{CurrentRPS: float64(i)}})
	}
	if got := cur.SparklineLen(); got != 120 {
		t.Errorf("sparkline ring length got %d want 120", got)
	}
}

func TestRunViewEmptySnapshotDoesNotPanic(t *testing.T) {
	rv := runview.New(runview.Options{Title: "empty", Total: 0})
	out := rv.View()
	if !strings.Contains(out, "empty") {
		t.Errorf("expected title in view, got:\n%s", out)
	}
}

func TestRunViewRendersDedicatedRequestContextRow(t *testing.T) {
	rv := runview.New(runview.Options{
		Title: "Crankfire",
		Total: 500,
		RequestContext: runview.RequestContext{
			RawURL: "https://example.com/run?payloadsizekb=100",
			Params: []runview.ContextParam{
				{Label: "Workers", Value: "5"},
				{Label: "Rate", Value: "5/s"},
			},
			QueryParams: []runview.ContextParam{
				{Label: "payloadsizekb", Value: "100"},
			},
		},
	})
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	rv, _ = rv.Update(runview.SnapshotMsg{Snap: metrics.DataPoint{TotalRequests: 59, SuccessfulRequests: 59, CurrentRPS: 5.97}})

	out := rv.View()
	for _, want := range []string{"Run Summary", "Requests / sec", "Request Context", "https://example.com/run?payloadsizekb=100", "payloadsizekb=100"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRunViewDropsStandaloneMetricsPanel(t *testing.T) {
	rv := runview.New(runview.Options{Title: "x", Total: 1})
	out := rv.View()
	// The old layout had a standalone "Metrics" panel showing Total/Success/Failed counts
	// The new layout splits that info: summary stats in Run Summary, rate in RPS vs Target
	// Only "Protocol Metrics" should remain, not a standalone "Metrics" panel
	if strings.Contains(out, "┌ Metrics ") {
		t.Fatalf("did not expect standalone Metrics panel:\n%s", out)
	}
}

func TestRunViewRendersLegacyDashboardPanelsAndEmptyStates(t *testing.T) {
	rv := runview.New(runview.Options{Title: "Crankfire", Total: 500})
	rv1, _ := rv.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	rv2, _ := rv1.Update(runview.SnapshotMsg{
		Snap: metrics.DataPoint{
			TotalRequests:      172,
			SuccessfulRequests: 172,
			CurrentRPS:         221.38,
		},
		Stats: &metrics.Stats{
			EndpointStats: metrics.EndpointStats{
				Total:         172,
				Successes:     172,
				Failures:      0,
				MinLatencyMs:  67.05,
				MeanLatencyMs: 361.28,
				P50LatencyMs:  239.23,
				P90LatencyMs:  673.28,
				P95LatencyMs:  911.42,
				P99LatencyMs:  1963.01,
			},
		},
		Elapsed: 27*time.Minute + 16*time.Second,
	})

	out := rv2.View()
	for _, want := range []string{
		"Run Summary",
		"Elapsed: 27m16s | Total: 172 | Success Rate: 100.0%",
		"Requests / sec",
		"221.38",
		"Request Context",
		"Latency Trend",
		"Latency Stats / Health",
		"Min:",
		"Mean:",
		"P50:",
		"P90:",
		"P95:",
		"P99:",
		"Protocol Metrics",
		"No protocol-specific metrics",
		"Endpoints",
		"No endpoint data",
		"Status Buckets",
		"No failures",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestRunViewShowsRequestProgressWhenTotalConfigured(t *testing.T) {
	rv := runview.New(runview.Options{Title: "x", Total: 100})
	rv1, _ := rv.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	rv2, _ := rv1.Update(runview.SnapshotMsg{
		Snap: metrics.DataPoint{
			TotalRequests:      40,
			SuccessfulRequests: 38,
		},
	})

	out := rv2.View()
	if !strings.Contains(out, "Progress: 40% (40/100)") {
		t.Errorf("expected request progress line, got:\n%s", out)
	}
}

func TestRunViewLatencyGraphUsesLatencyAcrossSnapshotTypes(t *testing.T) {
	rv := runview.New(runview.Options{Title: "x"})
	rv1, _ := rv.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	rv2, _ := rv1.Update(runview.SnapshotMsg{
		Snap: metrics.DataPoint{
			CurrentRPS:   1,
			P95LatencyMs: 50,
		},
	})
	rv3, _ := rv2.Update(runview.SnapshotMsg{
		Snap: metrics.DataPoint{
			CurrentRPS:   1000,
			P95LatencyMs: 50,
		},
		Stats: &metrics.Stats{
			EndpointStats: metrics.EndpointStats{
				MeanLatencyMs: 50,
				P95LatencyMs:  50,
			},
		},
	})

	out := rv3.View()
	if !strings.Contains(out, "██") {
		t.Errorf("expected latency trend to keep matching latency samples, got:\n%s", out)
	}
}

func TestRunViewUsesTerminalHeightForDashboardLayout(t *testing.T) {
	rv := runview.New(runview.Options{Title: "Crankfire", Total: 500})
	rv1, _ := rv.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	var rv2 runview.Model = rv1
	for i := 0; i < 80; i++ {
		rv2, _ = rv2.Update(runview.SnapshotMsg{
			Snap: metrics.DataPoint{
				P95LatencyMs: float64(100 + i),
			},
		})
	}
	rv3, _ := rv2.Update(runview.SnapshotMsg{
		Snap: metrics.DataPoint{
			TotalRequests:      76,
			SuccessfulRequests: 76,
			CurrentRPS:         3.95,
			P95LatencyMs:       629.76,
		},
		Stats: &metrics.Stats{
			EndpointStats: metrics.EndpointStats{
				Total:         76,
				Successes:     76,
				MinLatencyMs:  105.69,
				MeanLatencyMs: 194.27,
				P50LatencyMs:  157.18,
				P90LatencyMs:  209.41,
				P95LatencyMs:  629.76,
				P99LatencyMs:  631.29,
			},
		},
		Elapsed: 15 * time.Second,
	})

	out := rv3.View()
	if got := len(strings.Split(out, "\n")); got < 35 || got > 40 {
		t.Errorf("expected dashboard to fit within terminal height while using most of it, got %d lines:\n%s", got, out)
	}
	for _, want := range []string{"Protocol Metrics", "Endpoints", "Status Buckets"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected bottom panels to remain visible; missing %q in:\n%s", want, out)
		}
	}
}

func TestRunViewClipsPopulatedPanelsToTerminalHeight(t *testing.T) {
	rv := runview.New(runview.Options{Title: "Crankfire", Total: 500})
	rv1, _ := rv.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	var rv2 runview.Model = rv1
	for i := 0; i < 80; i++ {
		rv2, _ = rv2.Update(runview.SnapshotMsg{
			Snap: metrics.DataPoint{
				P95LatencyMs: float64(100 + i),
			},
		})
	}

	endpoints := make([]widgets.EndpointRow, 0, 10)
	for i := 0; i < 10; i++ {
		endpoints = append(endpoints, widgets.EndpointRow{
			Method:   "POST",
			Path:     "/scenario/" + strings.Repeat("x", i+1),
			Count:    int64(100 - i),
			SharePct: 10,
			RPS:      5.5,
			P95Ms:    200 + float64(i),
			P99Ms:    300 + float64(i),
			ErrPct:   0.1 * float64(i),
			Errors:   int64(i),
		})
	}

	rv3, _ := rv2.Update(runview.SnapshotMsg{
		Snap: metrics.DataPoint{
			TotalRequests:      76,
			SuccessfulRequests: 76,
			CurrentRPS:         3.95,
			P95LatencyMs:       629.76,
		},
		Stats: &metrics.Stats{
			EndpointStats: metrics.EndpointStats{
				Total:         76,
				Successes:     76,
				MinLatencyMs:  105.69,
				MeanLatencyMs: 194.27,
				P50LatencyMs:  157.18,
				P90LatencyMs:  209.41,
				P95LatencyMs:  629.76,
				P99LatencyMs:  631.29,
			},
		},
		ProtocolMetrics: map[string]map[string]interface{}{
			"http": {
				"requests":   int64(76),
				"open_conns": float64(4),
				"bytes":      int64(1024),
				"reuse":      float64(0.75),
			},
		},
		StatusBuckets: map[string]map[string]int{
			"http": {"500": 5, "502": 4, "503": 3},
			"grpc": {"UNAVAILABLE": 2, "INTERNAL": 1},
		},
		Endpoints: endpoints,
		Elapsed:   15 * time.Second,
	})

	out := rv3.View()
	if got := len(strings.Split(out, "\n")); got > 40 {
		t.Errorf("expected populated dashboard to fit terminal height, got %d lines:\n%s", got, out)
	}
	for _, want := range []string{"Protocol Metrics", "Endpoints", "Status Buckets", "http:", "HTTP 500 5", "/scenario/"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

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
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 80, Height: 32})
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
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 80, Height: 32})
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

func TestRunViewLatencyTrendUsesP95Series(t *testing.T) {
	rv := runview.New(runview.Options{Title: "x"})
	rv, _ = rv.Update(runview.SnapshotMsg{Stats: &metrics.Stats{EndpointStats: metrics.EndpointStats{P95LatencyMs: 180}}})
	rv, _ = rv.Update(runview.SnapshotMsg{Stats: &metrics.Stats{EndpointStats: metrics.EndpointStats{P95LatencyMs: 220}}})
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	out := rv.View()
	if !strings.Contains(out, "Latency Trend") {
		t.Fatalf("missing latency trend panel:\n%s", out)
	}
	if !strings.Contains(out, "█") {
		t.Fatalf("expected chart blocks in latency trend:\n%s", out)
	}
	if got := rv.SparklineLen(); got != 2 {
		t.Fatalf("expected 2 latency samples, got %d", got)
	}
}

func TestRunViewRPSPanelShowsConfiguredTarget(t *testing.T) {
	rv := runview.New(runview.Options{
		Title: "x",
		Total: 1,
		RequestContext: runview.RequestContext{
			TargetRPS: 5,
			Params:    []runview.ContextParam{{Label: "Rate", Value: "5/s"}},
		},
	})
	rv, _ = rv.Update(runview.SnapshotMsg{Snap: metrics.DataPoint{CurrentRPS: 6}})
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	out := rv.View()
	for _, want := range []string{"Requests / sec", "6.00", "Target 5.00/s"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRunViewRPSBarUsesFullPanelContentWidth(t *testing.T) {
	rv := runview.New(runview.Options{
		Title:          "x",
		Total:          1,
		RequestContext: runview.RequestContext{TargetRPS: 10},
	})
	rv, _ = rv.Update(runview.SnapshotMsg{Snap: metrics.DataPoint{CurrentRPS: 10}})
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	out := rv.View()

	// In the new layout, the RPS strip is inline with the summary panel.
	// Bar chars only appear in the strip, not in the summary panel.
	topRowEnd := strings.Index(out, "┌ Request Context")
	if topRowEnd == -1 {
		t.Fatalf("missing Request Context row:\n%s", out)
	}
	topRow := out[:topRowEnd]

	var barLine string
	for _, line := range strings.Split(topRow, "\n") {
		if strings.Contains(line, "█") || strings.Contains(line, "░") {
			barLine = line
			break
		}
	}
	if barLine == "" {
		t.Fatalf("could not find RPS bar in output:\n%s", out)
	}

	// Count all bar chars — they are exclusively in the strip portion
	barLen := strings.Count(barLine, "█") + strings.Count(barLine, "░")

	// RPS is in the RIGHT strip of a 0.58 ratio split
	// For 120 width: usable=119, left=int(119*0.58)=69, right=119-69=50
	// Strip content width = rightPanelContentWidth(120, 0.58) = max(1, 50-4) = 46
	expectedWidth := 46
	if barLen != expectedWidth {
		t.Fatalf("RPS bar length %d, expected %d (full strip content width). Bar line: %q", barLen, expectedWidth, barLine)
	}
}

func TestRunViewRPSPanelAppliesSemanticStyling(t *testing.T) {
	// This test verifies that semantic styling is applied to the RPS strip
	// based on current vs target ratios, using actual ANSI color codes.
	// Lipgloss degrades 256-color codes to 16-color codes in test env:
	// okStyle (color 42) → \x1b[92m (bright green)
	// warnStyle (color 220) → \x1b[93m (bright yellow)
	// errStyle (color 196) → \x1b[91m (bright red)

	tests := []struct {
		name        string
		current     float64
		target      float64
		wantColor   string // Expected ANSI sequence (e.g., "\x1b[92m")
		wantNeutral bool   // True if no semantic color should be applied
	}{
		{"below target (green)", 3, 10, "\x1b[92m", false},
		{"at target (green)", 10, 10, "\x1b[92m", false},
		{"slightly over (yellow)", 11, 10, "\x1b[93m", false},
		{"well over (red)", 15, 10, "\x1b[91m", false},
		{"no target (neutral)", 10, 0, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rv := runview.New(runview.Options{
				Title:          "x",
				RequestContext: runview.RequestContext{TargetRPS: tt.target},
			})
			rv, _ = rv.Update(runview.SnapshotMsg{Snap: metrics.DataPoint{CurrentRPS: tt.current}})
			rv, _ = rv.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
			out := rv.View()

			// In the new layout the strip is inline — bar chars only appear in the strip.
			topRowEnd := strings.Index(out, "┌ Request Context")
			if topRowEnd == -1 {
				t.Fatalf("missing Request Context panel")
			}
			topRow := out[:topRowEnd]

			// Find the bar line (bar chars exclusively come from the strip)
			var barLine string
			for _, line := range strings.Split(topRow, "\n") {
				if strings.Contains(line, "█") || strings.Contains(line, "░") {
					barLine = line
					break
				}
			}
			if barLine == "" {
				t.Errorf("expected progress bar in RPS strip")
				return
			}

			// Verify semantic color codes on the bar line
			if tt.wantNeutral {
				// No target: should not have semantic state colors (green/yellow/red)
				semanticCodes := []string{"\x1b[91m", "\x1b[92m", "\x1b[93m"}
				for _, code := range semanticCodes {
					if strings.Contains(barLine, code) {
						t.Errorf("neutral state (no target) should not use semantic color %q, but found it in bar line", code)
					}
				}
			} else {
				// With target: should have the expected semantic color
				if !strings.Contains(barLine, tt.wantColor) {
					t.Errorf("expected ANSI color code %q for %s state, not found in bar line",
						tt.wantColor, tt.name)
				}
			}
		})
	}
}

func TestRunViewUnlimitedRunScalesToLowRPS(t *testing.T) {
	// Verify that unlimited runs (target=0) with low RPS (< 1.0) scale
	// to the actual rolling max, not a hardcoded 1.0 floor
	rv := runview.New(runview.Options{
		Title:          "x",
		RequestContext: runview.RequestContext{TargetRPS: 0}, // unlimited
	})
	// Send a low RPS sample
	rv, _ = rv.Update(runview.SnapshotMsg{Snap: metrics.DataPoint{CurrentRPS: 0.5}})
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	out := rv.View()

	// In the new layout, bar chars only appear in the strip (not in summary panel).
	topRowEnd := strings.Index(out, "┌ Request Context")
	if topRowEnd == -1 {
		t.Fatalf("could not find Request Context row:\n%s", out)
	}
	topRow := out[:topRowEnd]

	var barLine string
	for _, line := range strings.Split(topRow, "\n") {
		if strings.Contains(line, "█") {
			barLine = line
			break
		}
	}
	if barLine == "" {
		t.Fatalf("could not find RPS bar in output:\n%s", out)
	}

	// Count filled vs empty blocks — all bar chars are in the strip
	filled := strings.Count(barLine, "█")
	empty := strings.Count(barLine, "░")
	total := filled + empty

	// When current RPS (0.5) equals the rolling max (0.5), bar should be full
	// Expected: 46 filled blocks (full strip content width)
	expectedWidth := 46
	if total != expectedWidth {
		t.Fatalf("expected total bar width of %d, got %d", expectedWidth, total)
	}
	if filled != expectedWidth {
		t.Fatalf("low-RPS unlimited run should show full bar (current=max=0.5), got %d/%d filled", filled, total)
	}
}

func TestRunViewRPSStripUsesInlineSection(t *testing.T) {
	rv := runview.New(runview.Options{
		Title: "x",
		Total: 1,
		RequestContext: runview.RequestContext{
			TargetRPS: 5,
			Params:    []runview.ContextParam{{Label: "Rate", Value: "5/s"}},
		},
	})
	rv, _ = rv.Update(runview.SnapshotMsg{Snap: metrics.DataPoint{CurrentRPS: 6}})
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	out := rv.View()

	requestContextStart := strings.Index(out, "┌ Request Context")
	if requestContextStart == -1 {
		t.Fatalf("missing Request Context row:\n%s", out)
	}
	topRow := out[:requestContextStart]

	if strings.Contains(topRow, "┌ RPS vs Target") {
		t.Fatalf("top row still renders RPS as a boxed panel:\n%s", topRow)
	}
	for _, want := range []string{"Requests / sec", "6.00", "Target 5.00/s"} {
		if !strings.Contains(topRow, want) {
			t.Fatalf("missing %q in compact RPS strip:\n%s", want, topRow)
		}
	}
	if strings.Count(topRow, "┌") != 1 {
		t.Fatalf("expected only the summary panel to have a top border in row 0:\n%s", topRow)
	}
}

func TestRunViewRPSStripPreservesSemanticStyling(t *testing.T) {
	rv := runview.New(runview.Options{
		Title:          "x",
		RequestContext: runview.RequestContext{TargetRPS: 10},
	})
	rv, _ = rv.Update(runview.SnapshotMsg{Snap: metrics.DataPoint{CurrentRPS: 15}})
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	out := rv.View()

	requestContextStart := strings.Index(out, "┌ Request Context")
	if requestContextStart == -1 {
		t.Fatalf("missing Request Context row:\n%s", out)
	}
	topRow := out[:requestContextStart]

	if !strings.Contains(topRow, "\x1b[91m") {
		t.Fatalf("expected red semantic styling in compact RPS strip:\n%s", topRow)
	}
	if !strings.Contains(topRow, "█") && !strings.Contains(topRow, "░") {
		t.Fatalf("expected target-aware bar in compact RPS strip:\n%s", topRow)
	}
}

func TestRunViewRPSStripKeepsBarWhenTargetLabelWraps(t *testing.T) {
	rv := runview.New(runview.Options{
		Title: "x",
		RequestContext: runview.RequestContext{
			TargetRPS: 1234567890,
		},
	})
	rv, _ = rv.Update(runview.SnapshotMsg{Snap: metrics.DataPoint{CurrentRPS: 15}})
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	out := rv.View()

	requestContextStart := strings.Index(out, "┌ Request Context")
	if requestContextStart == -1 {
		t.Fatalf("missing Request Context row:\n%s", out)
	}
	topRow := out[:requestContextStart]

	if !strings.Contains(topRow, "Requests / sec | Target") {
		t.Fatalf("expected target context label in compact RPS strip:\n%s", topRow)
	}
	if !strings.Contains(topRow, "15.00") {
		t.Fatalf("expected current RPS value to remain visible when target label wraps:\n%s", topRow)
	}
	if !strings.Contains(topRow, "█") && !strings.Contains(topRow, "░") {
		t.Fatalf("expected target-aware bar to remain visible when target label wraps:\n%s", topRow)
	}
}

func TestRunViewFitsShortTerminalAndPreservesRequestContext(t *testing.T) {
	rv := runview.New(runview.Options{
		Title: "x",
		RequestContext: runview.RequestContext{
			RawURL: "https://example.com/run?payloadsizekb=100&orchestrationcount=1",
			Params: []runview.ContextParam{
				{Label: "Workers", Value: "5"},
				{Label: "Rate", Value: "5/s"},
			},
			QueryParams: []runview.ContextParam{
				{Label: "payloadsizekb", Value: "100"},
				{Label: "orchestrationcount", Value: "1"},
			},
		},
	})
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	out := rv.View()

	// Verify output does not exceed height budget
	lines := strings.Count(out, "\n") + 1
	if lines > 24 {
		t.Errorf("output %d lines exceeds height budget of 24:\n%s", lines, out)
	}

	// Verify Request Context panel header is visible
	if !strings.Contains(out, "Request Context") {
		t.Errorf("missing Request Context panel in short terminal:\n%s", out)
	}

	// Verify structured query-param detail lines survive clipping.
	// Each query param appears twice: once in RawURL, once in dedicated detail line.
	// If count < 2, the structured detail line was clipped.
	for _, param := range []string{"payloadsizekb=100", "orchestrationcount=1"} {
		count := strings.Count(out, param)
		if count < 2 {
			t.Errorf("query param %q appears %d times (want >= 2 for structured detail line):\n%s", param, count, out)
		}
	}
}

func TestRunViewNarrowStackedLayoutIsHeightSafeAndPreservesRequestContext(t *testing.T) {
	rv := runview.New(runview.Options{
		Title: "x",
		RequestContext: runview.RequestContext{
			RawURL: "https://example.com/run?payloadsizekb=100&orchestrationcount=1",
			Params: []runview.ContextParam{
				{Label: "Workers", Value: "5"},
				{Label: "Rate", Value: "5/s"},
			},
			QueryParams: []runview.ContextParam{
				{Label: "payloadsizekb", Value: "100"},
				{Label: "orchestrationcount", Value: "1"},
			},
		},
	})
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 72, Height: 32})
	out := rv.View()

	// 1. Verify output does not exceed height budget (critical for stacked layout)
	lines := strings.Count(out, "\n") + 1
	if lines > 32 {
		t.Errorf("narrow stacked output %d lines exceeds height budget of 32:\n%s", lines, out)
	}

	// 2. Verify panels are actually stacked (not side-by-side)
	outLines := strings.Split(out, "\n")
	summaryLine := -1
	rpsLine := -1
	for i, line := range outLines {
		if strings.Contains(line, "Run Summary") {
			summaryLine = i
		}
		if strings.Contains(line, "Requests / sec") {
			rpsLine = i
		}
	}
	if summaryLine == -1 || rpsLine == -1 {
		t.Fatalf("missing panel titles in output:\n%s", out)
	}
	if summaryLine == rpsLine {
		t.Errorf("narrow terminal should stack panels vertically, but titles appear on same line %d", summaryLine)
	}

	// 3. Verify structured Request Context detail survives despite stacked-layout overhead
	// Each query param appears twice: once in RawURL, once in dedicated detail line
	for _, param := range []string{"payloadsizekb=100", "orchestrationcount=1"} {
		count := strings.Count(out, param)
		if count < 2 {
			t.Errorf("query param %q appears %d times (want >= 2 for structured detail line) in narrow stacked layout:\n%s", param, count, out)
		}
	}
}

func TestRunViewNarrowShortStackedLayoutIsHeightSafeAndPreservesRequestContext(t *testing.T) {
	rv := runview.New(runview.Options{
		Title: "x",
		RequestContext: runview.RequestContext{
			RawURL: "https://example.com/run?payloadsizekb=100&orchestrationcount=1",
			Params: []runview.ContextParam{
				{Label: "Workers", Value: "5"},
				{Label: "Rate", Value: "5/s"},
			},
			QueryParams: []runview.ContextParam{
				{Label: "payloadsizekb", Value: "100"},
				{Label: "orchestrationcount", Value: "1"},
			},
		},
	})
	rv = withLatencyTrendData(rv)
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 72, Height: 24})
	out := rv.View()

	// 1. Verify output does not exceed height budget (critical for 72x24)
	lines := strings.Count(out, "\n") + 1
	if lines > 24 {
		t.Errorf("narrow short stacked output %d lines exceeds height budget of 24:\n%s", lines, out)
	}

	// 2. Verify panels are actually stacked (not side-by-side)
	outLines := strings.Split(out, "\n")
	summaryLine := -1
	rpsLine := -1
	for i, line := range outLines {
		if strings.Contains(line, "Run Summary") {
			summaryLine = i
		}
		if strings.Contains(line, "Requests / sec") {
			rpsLine = i
		}
	}
	if summaryLine == -1 || rpsLine == -1 {
		t.Fatalf("missing panel titles in output:\n%s", out)
	}
	if summaryLine == rpsLine {
		t.Errorf("narrow terminal should stack panels vertically, but titles appear on same line %d", summaryLine)
	}

	// 3. Verify structured Request Context detail survives even on narrow short terminal
	// Each query param appears twice: once in RawURL, once in dedicated detail line
	for _, param := range []string{"payloadsizekb=100", "orchestrationcount=1"} {
		count := strings.Count(out, param)
		if count < 2 {
			t.Errorf("query param %q appears %d times (want >= 2 for structured detail line) in narrow short stacked layout:\n%s", param, count, out)
		}
	}
}

func TestRunViewMinimalStackedLayoutIsHeightSafeAndPreservesRequestContext(t *testing.T) {
	rv := runview.New(runview.Options{
		Title: "x",
		RequestContext: runview.RequestContext{
			RawURL: "https://api.example.com/v2/test?apiKey=abc123&userId=user456",
			Params: []runview.ContextParam{
				{Label: "Workers", Value: "10"},
				{Label: "Duration", Value: "30s"},
			},
			QueryParams: []runview.ContextParam{
				{Label: "apiKey", Value: "abc123"},
				{Label: "userId", Value: "user456"},
			},
		},
	})
	const width, height = 72, 20
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: width, Height: height})
	out := rv.View()

	lines := strings.Count(out, "\n") + 1
	if lines > height {
		t.Errorf("minimal stacked output %d lines exceeds height budget of %d:\n%s", lines, height, out)
	}

	for _, param := range []string{"apiKey=abc123", "userId=user456"} {
		count := strings.Count(out, param)
		if count < 2 {
			t.Errorf("query param %q appears %d times (want >= 2 for structured detail line) in minimal stacked layout:\n%s", param, count, out)
		}
	}
}

// TestStackedBoundaryHeights verifies responsive layout across boundary heights in narrow stacked mode.
// Each test ensures: (1) output lines <= height budget, (2) stacked layout detected, (3) query params preserved (count >= 2).
// TestStackedRowZeroBudgetUpgradeOnOneLine verifies that row 0 (summary panel + RPS
// strip) is upgraded from shell-only (h=0) to one content line when exactly one line
// of remaining height is available after baseline allocation.
//
// Arithmetic (width=72, stacked mode):
//
//	stackedRowCost(0, 0) = (0+2)+rpsStripLineCount = 5
//	stackedRowCost(1, 3) = 3+2 = 5   (3-line RC: URL + Params + QueryParams)
//	stackedRowCost(2, 0) = 4
//	baseline = 5+5+4 = 14, availableHeight = 15 → remaining = 1
//
// With the old `remaining >= 2` guard the upgrade was skipped (remaining=1 < 2).
// With the corrected `remaining >= 1` guard the upgrade fires and rowHeights[0] = 1.
func TestStackedRowZeroBudgetUpgradeOnOneLine(t *testing.T) {
	rv := runview.New(runview.Options{
		Title: "x",
		RequestContext: runview.RequestContext{
			RawURL:      "https://example.com/api?id=1",
			Params:      []runview.ContextParam{{Label: "Workers", Value: "5"}},
			QueryParams: []runview.ContextParam{{Label: "id", Value: "1"}},
		},
	})
	rv, _ = rv.Update(runview.SnapshotMsg{Snap: metrics.DataPoint{TotalRequests: 42, CurrentRPS: 3}})
	// height=16: title(1) + availableHeight(15); baseline cost=14 so remaining=1
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 72, Height: 16})
	out := rv.View()

	lines := strings.Count(out, "\n") + 1
	if lines > 16 {
		t.Errorf("output %d lines exceeds height budget of 16:\n%s", lines, out)
	}

	// Run Summary should have at least one content line (h=1), not just borders.
	summarySection := extractSection(out, "Run Summary")
	if summarySection == "" {
		t.Fatalf("Run Summary panel not found:\n%s", out)
	}
	if panelContentLines(summarySection) < 1 {
		t.Errorf("expected Run Summary to have >= 1 content line (budget upgrade fired), got shell-only:\n%s", summarySection)
	}
}

// TestStackedRowZeroPriority5SummaryGrowth verifies the Priority 5 distribution path
// where Summary should receive the stranded 1-line remainder after Latency consumes 2.
//
// Budget trace at 72×27 with a 3-line RC (URL + 1 param + 1 query param):
//
//	title(1) + available(26); baseline cost = stackedRowCost(0,0)+stackedRowCost(1,3)+stackedRowCost(2,0)
//	= 5+5+4 = 14; remaining = 12.
//	P1 (row0==0,>=1): rowHeights[0]=1, remaining=11
//	P2 (row2==0,>=2): rowHeights[2]=1, remaining=9
//	P2.5 (grow Latency to 3): rowHeights[2]=3, remaining=5
//	P3 (Protocol,>=2): rowHeights[3]=0, remaining=3
//	P5 entry: remaining=3; Latency gets 2 → remaining=1; Summary gets 1 → remaining=0.
//	→ rowHeights[0]=2.
//
// Without the fix (old `remaining >= 2` guard) Summary's P5 branch is skipped and stays at 1.
func TestStackedRowZeroPriority5SummaryGrowth(t *testing.T) {
	rv := runview.New(runview.Options{
		Title: "x",
		RequestContext: runview.RequestContext{
			RawURL:      "https://example.com/api?id=1",
			Params:      []runview.ContextParam{{Label: "Workers", Value: "5"}},
			QueryParams: []runview.ContextParam{{Label: "id", Value: "1"}},
		},
	})
	rv, _ = rv.Update(runview.SnapshotMsg{Snap: metrics.DataPoint{TotalRequests: 100, CurrentRPS: 5}})
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 72, Height: 27})
	out := rv.View()

	lines := strings.Count(out, "\n") + 1
	if lines > 27 {
		t.Fatalf("output %d lines exceeds height budget of 27:\n%s", lines, out)
	}

	// Summary should have ≥ 2 inner lines (Priority 1 + Priority 5 each contribute 1).
	// summaryBody may produce blank content lines, so count all │-bordered lines, not just
	// non-empty ones.
	summarySection := extractSection(out, "Run Summary")
	if summarySection == "" {
		t.Fatalf("Run Summary panel not found:\n%s", out)
	}
	innerLines := 0
	for _, line := range strings.Split(summarySection, "\n") {
		if strings.HasPrefix(line, "│") {
			innerLines++
		}
	}
	if innerLines < 2 {
		t.Errorf("expected Run Summary >= 2 inner lines (P5 distributes stranded line), got %d:\n%s",
			innerLines, summarySection)
	}
}

// TestStackedDroppedRowsStayDroppedWhenRestoreCostCannotFit covers the overflow path
// where baseline stacked allocation drops row 0 and row 2 to -1. With width=72 and a
// 3-line Request Context, height=6 leaves only 1 line after the Request Context shell,
// which is not enough to restore either dropped row at full cost.
func TestStackedDroppedRowsStayDroppedWhenRestoreCostCannotFit(t *testing.T) {
	rv := runview.New(runview.Options{
		RequestContext: runview.RequestContext{
			RawURL:      "https://example.com/api?id=1",
			Params:      []runview.ContextParam{{Label: "Workers", Value: "5"}},
			QueryParams: []runview.ContextParam{{Label: "id", Value: "1"}},
		},
	})
	rv, _ = rv.Update(runview.SnapshotMsg{Snap: metrics.DataPoint{TotalRequests: 42, CurrentRPS: 3}})
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 72, Height: 6})
	out := rv.View()

	lines := strings.Count(out, "\n") + 1
	if lines > 6 {
		t.Fatalf("output %d lines exceeds height budget of 6:\n%s", lines, out)
	}
	if strings.Contains(out, "Run Summary") {
		t.Errorf("expected Run Summary to stay dropped when restore cost cannot fit:\n%s", out)
	}
	if strings.Contains(out, "Latency Trend") || strings.Contains(out, "Latency Stats / Health") {
		t.Errorf("expected latency panels to stay dropped when restore cost cannot fit:\n%s", out)
	}
}

// TestStackedMinimalRequestContextClampsToHeight covers the ultra-short fallback where
// even the reduced Request Context shell must still respect the available height.
func TestStackedMinimalRequestContextClampsToHeight(t *testing.T) {
	rv := runview.New(runview.Options{
		RequestContext: runview.RequestContext{
			RawURL:      "https://example.com/api?id=1",
			Params:      []runview.ContextParam{{Label: "Workers", Value: "5"}},
			QueryParams: []runview.ContextParam{{Label: "id", Value: "1"}},
		},
	})
	rv, _ = rv.Update(runview.SnapshotMsg{Snap: metrics.DataPoint{TotalRequests: 42, CurrentRPS: 3}})
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 72, Height: 4})
	out := rv.View()

	lines := strings.Count(out, "\n") + 1
	if lines > 4 {
		t.Fatalf("output %d lines exceeds height budget of 4:\n%s", lines, out)
	}
	if extractSection(out, "Request Context") == "" {
		t.Fatalf("Request Context should still render in ultra-short fallback:\n%s", out)
	}
}

// TestStackedRequestContextCanBeSkippedWhenHeightCannotFitBorders covers the
// pathological case where the terminal is too short to fit even a 2-line panel shell.
func TestStackedRequestContextCanBeSkippedWhenHeightCannotFitBorders(t *testing.T) {
	rv := runview.New(runview.Options{
		RequestContext: runview.RequestContext{
			RawURL:      "https://example.com/api?id=1",
			Params:      []runview.ContextParam{{Label: "Workers", Value: "5"}},
			QueryParams: []runview.ContextParam{{Label: "id", Value: "1"}},
		},
	})
	rv, _ = rv.Update(runview.SnapshotMsg{Snap: metrics.DataPoint{TotalRequests: 42, CurrentRPS: 3}})
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 72, Height: 1})
	out := rv.View()

	if out != "" {
		t.Fatalf("expected empty render when panel borders cannot fit within height 1:\n%s", out)
	}
}

// TestStackedDroppedRowsDoNotHangPriority5 covers the short-height state where row 0 and
// row 2 have been dropped, Protocol has been restored, and Priority 5 must exit instead
// of spinning forever with remaining=3 and no eligible row to grow.
func TestStackedDroppedRowsDoNotHangPriority5(t *testing.T) {
	rv := runview.New(runview.Options{
		RequestContext: runview.RequestContext{
			RawURL:      "https://example.com/api?id=1",
			Params:      []runview.ContextParam{{Label: "Workers", Value: "5"}},
			QueryParams: []runview.ContextParam{{Label: "id", Value: "1"}},
		},
	})
	rv, _ = rv.Update(runview.SnapshotMsg{Snap: metrics.DataPoint{TotalRequests: 42, CurrentRPS: 3}})
	// No title: availableHeight=10. Baseline drops row 0 + row 2, then Protocol restores
	// with remaining=3 before Priority 5 runs.
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 72, Height: 10})

	done := make(chan string, 1)
	go func() {
		done <- rv.View()
	}()

	select {
	case out := <-done:
		lines := strings.Count(out, "\n") + 1
		if lines > 10 {
			t.Fatalf("output %d lines exceeds height budget of 10:\n%s", lines, out)
		}
		if !strings.Contains(out, "Protocol Metrics") {
			t.Fatalf("expected Protocol Metrics to be restored in this short-height case:\n%s", out)
		}
		if strings.Contains(out, "Run Summary") || strings.Contains(out, "Latency Trend") {
			t.Fatalf("expected dropped top rows to remain absent in this short-height case:\n%s", out)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("View hung in Priority 5 with dropped rows and remaining=3")
	}
}

func TestStackedBoundaryHeights(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		desc   string
	}{
		{"72x20", 72, 20, "minimal stacked height with panel skipping"},
		{"72x21", 72, 21, "boundary above minimal"},
		{"72x24", 72, 24, "short stacked height"},
		{"72x29", 72, 29, "boundary between short and medium"},
		{"72x31", 72, 31, "medium stacked height"},
		{"72x33", 72, 33, "boundary between medium and tall"},
		{"72x35", 72, 35, "tall stacked height"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rv := runview.New(runview.Options{
				Title: "x",
				RequestContext: runview.RequestContext{
					RawURL: "https://api.example.com/v1/users?apiKey=abc123&format=json",
					Params: []runview.ContextParam{
						{Label: "Workers", Value: "10"},
						{Label: "Rate", Value: "100/s"},
					},
					QueryParams: []runview.ContextParam{
						{Label: "apiKey", Value: "abc123"},
						{Label: "format", Value: "json"},
					},
				},
			})
			rv, _ = rv.Update(tea.WindowSizeMsg{Width: tt.width, Height: tt.height})
			out := rv.View()

			// Verify output lines <= height budget
			lines := strings.Count(out, "\n") + 1
			if lines > tt.height {
				t.Errorf("%s: output has %d lines, exceeds height %d:\n%s", tt.desc, lines, tt.height, out)
			}

			// Verify stacked layout (panels appear vertically)
			if !strings.Contains(out, "Run Summary") || !strings.Contains(out, "Requests / sec") {
				t.Errorf("%s: missing stacked panels (Run Summary or Requests / sec not found)", tt.desc)
			}

			// Verify query params survive (occurrence count >= 2: once in URL, once as detail line)
			for _, param := range []string{"apiKey=abc123", "format=json"} {
				count := strings.Count(out, param)
				if count < 2 {
					t.Errorf("%s: query param %q appears %d times (want >= 2):\n%s", tt.desc, param, count, out)
				}
			}
		})
	}
}

// TestStackedFullRequestContextPayload verifies that all Request Context fields
// survive rendering at common constrained screen sizes (72x24). A realistic payload
// includes: long URL with multiple query params, Method+Protocol, and 6 config params.
// This test should ensure all fields are visible without clipping lower-priority panels.
func TestStackedFullRequestContextPayload(t *testing.T) {
	rv := runview.New(runview.Options{
		Title: "x",
		RequestContext: runview.RequestContext{
			RawURL:   "https://api.example.com/v2/users/search?apiKey=abc123def456&format=json&limit=100&offset=0",
			Method:   "POST",
			Protocol: "HTTP/2",
			Params: []runview.ContextParam{
				{Label: "Workers", Value: "10"},
				{Label: "Rate", Value: "100/s"},
				{Label: "Duration", Value: "5m"},
				{Label: "Timeout", Value: "30s"},
				{Label: "Retries", Value: "3"},
				{Label: "Config", Value: "prod.yml"},
			},
			QueryParams: []runview.ContextParam{
				{Label: "apiKey", Value: "abc123def456"},
				{Label: "format", Value: "json"},
				{Label: "limit", Value: "100"},
				{Label: "offset", Value: "0"},
			},
		},
	})
	rv = withLatencyTrendData(rv)
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 72, Height: 24})
	out := rv.View()

	// Verify output lines <= height budget
	lines := strings.Count(out, "\n") + 1
	if lines > 24 {
		t.Errorf("output has %d lines, exceeds height 24:\n%s", lines, out)
	}

	// Extract Request Context section
	rcSection := extractSection(out, "Request Context")
	if rcSection == "" {
		t.Fatalf("Request Context panel not found in output:\n%s", out)
	}

	// All config params must be present (check values and labels separately for wrapping resilience)
	requiredValues := []string{
		"10",       // Workers
		"100/s",    // Rate
		"5m",       // Duration
		"30s",      // Timeout
		"3",        // Retries
		"prod.yml", // Config
	}
	for _, val := range requiredValues {
		if !strings.Contains(rcSection, val) {
			t.Errorf("Request Context missing value %q:\n%s", val, rcSection)
		}
	}
	requiredLabels := []string{"Workers:", "Rate:", "Duration:", "Timeout:", "Retries:", "Config:"}
	for _, label := range requiredLabels {
		if !strings.Contains(rcSection, label) {
			t.Errorf("Request Context missing label %q:\n%s", label, rcSection)
		}
	}

	// All query params must be present as detail lines
	requiredQueryParams := []string{
		"apiKey=abc123def456",
		"format=json",
		"limit=100",
		"offset=0",
	}
	for _, qp := range requiredQueryParams {
		if !strings.Contains(rcSection, qp) {
			t.Errorf("Request Context missing query param %q:\n%s", qp, rcSection)
		}
	}

	// Verify top panels have meaningful content (not shell-only)
	summarySection := extractSection(out, "Run Summary")
	if summarySection == "" {
		t.Error("Run Summary panel should be present at 72x24")
	} else if panelContentLines(summarySection) == 0 {
		t.Errorf("Run Summary should have content lines, found shell-only:\n%s", summarySection)
	}

	if !strings.Contains(out, "Requests / sec") {
		t.Error("RPS strip should be present at 72x24")
	}
	if !strings.Contains(out, "█") && !strings.Contains(out, "░") {
		t.Error("RPS strip at 72x24 should render a bar (█ or ░)")
	}

	latencySection := extractSection(out, "Latency Trend")
	if latencySection == "" {
		t.Error("Latency Trend panel should be present at 72x24")
	} else if panelContentLines(latencySection) == 0 {
		t.Errorf("Latency Trend should have content lines, found shell-only:\n%s", latencySection)
	}

	// Verify Protocol and Endpoints are absent (lower priority)
	if strings.Contains(out, "Protocol Metrics") {
		t.Error("Protocol Metrics panel should be absent at 72x24 with full Request Context")
	}
	if strings.Contains(out, "Endpoints") {
		t.Error("Endpoints panel should be absent at 72x24 with full Request Context")
	}
}

// TestStackedPriorityDegradationOrder verifies that lower-priority panels disappear
// before higher-priority ones lose content. With compacted Request Context, at 72x24
// we now have room for Protocol to appear. Endpoints/Status should still be absent.
func TestStackedPriorityDegradationOrder(t *testing.T) {
	rv := runview.New(runview.Options{
		Title: "x",
		RequestContext: runview.RequestContext{
			RawURL: "https://example.com/api/resource?id=123&detailed=true",
			Params: []runview.ContextParam{
				{Label: "Workers", Value: "5"},
			},
			QueryParams: []runview.ContextParam{
				{Label: "id", Value: "123"},
				{Label: "detailed", Value: "true"},
			},
		},
	})
	rv, _ = rv.Update(tea.WindowSizeMsg{Width: 72, Height: 24})
	out := rv.View()

	lines := strings.Split(out, "\n")
	actualLines := len(lines)
	if actualLines > 24 {
		t.Errorf("output %d lines exceeds height budget 24", actualLines)
	}

	// Top-row assertions: verify compact inline RPS strip
	requestContextStart := strings.Index(out, "┌ Request Context")
	if requestContextStart == -1 {
		t.Fatalf("missing Request Context panel")
	}
	topRow := out[:requestContextStart]

	// Must contain Run Summary and Requests / sec labels
	if !strings.Contains(topRow, "Run Summary") {
		t.Errorf("top row missing 'Run Summary' label:\n%s", topRow)
	}
	if !strings.Contains(topRow, "Requests / sec") {
		t.Errorf("top row missing 'Requests / sec' label:\n%s", topRow)
	}

	// Must contain bar/value content (█ or ░)
	if !strings.Contains(topRow, "█") && !strings.Contains(topRow, "░") {
		t.Errorf("top row missing RPS bar content (█ or ░):\n%s", topRow)
	}

	// Must NOT contain old boxed RPS panel
	if strings.Contains(topRow, "┌ RPS vs Target") {
		t.Errorf("top row still has old boxed RPS panel:\n%s", topRow)
	}

	// Query params should survive (2+ occurrences)
	for _, param := range []string{"id=123", "detailed=true"} {
		count := strings.Count(out, param)
		if count < 2 {
			t.Errorf("query param %q appears %d times (want >= 2)", param, count)
		}
	}

	// Latency should retain meaningful content even if the chart panel collapses
	// before the stats panel at this height.
	latencyTrendSection := extractSection(out, "Latency Trend")
	latencyStatsSection := extractSection(out, "Latency Stats / Health")
	if panelContentLines(latencyTrendSection) == 0 && panelContentLines(latencyStatsSection) == 0 {
		t.Errorf("latency content disappeared entirely at 72x24:\n%s", out)
	}

	// Protocol panel can now appear (compacted RC frees space)
	// This is expected behavior - compaction allows more panels

	// Endpoints and Status panels should still be ABSENT (lowest priority)
	endpointsSection := extractSection(out, "Endpoints")
	if endpointsSection != "" {
		t.Error("Endpoints panel should be absent at 72x24 (lowest priority)")
	}

	statusSection := extractSection(out, "Status")
	if statusSection != "" {
		t.Error("Status panel should be absent at 72x24 (lowest priority)")
	}
}

// TestStackedVeryShortHeights verifies that extremely short heights don't overflow.
func TestStackedVeryShortHeights(t *testing.T) {
	tests := []struct {
		height int
	}{
		{14},
		{15},
	}

	for _, tt := range tests {
		t.Run(string(rune('0'+tt.height/10))+string(rune('0'+tt.height%10)), func(t *testing.T) {
			rv := runview.New(runview.Options{
				Title: "x",
				RequestContext: runview.RequestContext{
					RawURL: "https://example.com/api/resource?id=123&detailed=true",
					Params: []runview.ContextParam{
						{Label: "Workers", Value: "5"},
					},
					QueryParams: []runview.ContextParam{
						{Label: "id", Value: "123"},
						{Label: "detailed", Value: "true"},
					},
				},
			})
			rv, _ = rv.Update(tea.WindowSizeMsg{Width: 72, Height: tt.height})
			out := rv.View()

			lines := strings.Split(out, "\n")
			actualLines := len(lines)
			if actualLines > tt.height {
				t.Errorf("output %d lines exceeds height budget %d:\n%s", actualLines, tt.height, out)
			}

			// Query params should still survive if Request Context is visible
			for _, param := range []string{"id=123", "detailed=true"} {
				count := strings.Count(out, param)
				if count < 2 {
					t.Errorf("query param %q appears %d times (want >= 2)", param, count)
				}
			}
		})
	}
}

// extractSection finds a panel section by title in the output
func extractSection(output, title string) string {
	lines := strings.Split(output, "\n")
	var section []string
	inSection := false
	for _, line := range lines {
		// Check for panel title in border (┌ for top border or │ for title line)
		if strings.Contains(line, "┌ "+title+" ") || strings.Contains(line, "┌ "+title) ||
			strings.Contains(line, "│ "+title+" ") || strings.Contains(line, "│"+title+" ") {
			inSection = true
		}
		if inSection {
			section = append(section, line)
			if strings.HasPrefix(strings.TrimSpace(line), "└") || strings.HasPrefix(strings.TrimSpace(line), "┘") {
				break
			}
		}
	}
	return strings.Join(section, "\n")
}

func panelContentLines(section string) int {
	if section == "" {
		return 0
	}

	count := 0
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "│ ") || !strings.HasSuffix(line, " │") {
			continue
		}

		inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "│ "), " │"))
		if inner != "" {
			count++
		}
	}

	return count
}

func withLatencyTrendData(rv runview.Model) runview.Model {
	samples := []struct {
		rps float64
		p50 float64
		p95 float64
	}{
		{92, 80, 110},
		{96, 85, 120},
	}
	for _, sample := range samples {
		rv, _ = rv.Update(runview.SnapshotMsg{
			Snap: metrics.DataPoint{
				TotalRequests:      48,
				SuccessfulRequests: 48,
				CurrentRPS:         sample.rps,
				P50LatencyMs:       sample.p50,
				P95LatencyMs:       sample.p95,
			},
			Stats: &metrics.Stats{
				EndpointStats: metrics.EndpointStats{
					Total:          48,
					Successes:      48,
					RequestsPerSec: sample.rps,
					MinLatencyMs:   75,
					MeanLatencyMs:  90,
					P50LatencyMs:   sample.p50,
					P95LatencyMs:   sample.p95,
					MaxLatencyMs:   140,
				},
			},
		})
	}
	return rv
}
