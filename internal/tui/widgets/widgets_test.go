// internal/tui/widgets/widgets_test.go
package widgets_test

import (
	"strings"
	"testing"

	"github.com/torosent/crankfire/internal/tui/widgets"
)

func TestProgressBar(t *testing.T) {
	got := widgets.Progress(40, 0.25)
	// 25% of 40 cells = 10 filled
	if !strings.HasPrefix(got, strings.Repeat("█", 10)) {
		t.Errorf("got %q, want 10 filled blocks at start", got)
	}
}

func TestSparkline(t *testing.T) {
	samples := []float64{0, 1, 2, 3, 4, 5, 6, 7}
	got := widgets.Sparkline(samples, 8)
	if len([]rune(got)) != 8 {
		t.Errorf("got %d runes want 8", len([]rune(got)))
	}
}

func TestSparklineEmpty(t *testing.T) {
	if got := widgets.Sparkline(nil, 8); got != strings.Repeat(" ", 8) {
		t.Errorf("got %q want 8 spaces", got)
	}
}

func TestPercentileTable(t *testing.T) {
	got := widgets.PercentileTable(map[string]float64{"p50": 10, "p95": 99})
	if !strings.Contains(got, "p50") || !strings.Contains(got, "99") {
		t.Errorf("missing fields:\n%s", got)
	}
}

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
