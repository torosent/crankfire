package widgets_test

import (
	"strings"
	"testing"

	"github.com/torosent/crankfire/internal/tui/widgets"
)

func TestTrendChartStartsAtLeftWhenHistoryIsShort(t *testing.T) {
	got := widgets.TrendChart([]float64{110, 140, 125}, 8, 4, nil)
	lines := strings.Split(got, "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines want 4", len(lines))
	}
	if lines[len(lines)-1][0] == ' ' && lines[len(lines)-1][1] == ' ' {
		t.Fatalf("expected early history to start on the left:\n%s", got)
	}
	if !strings.Contains(got, "█") {
		t.Fatalf("expected chart blocks to be rendered:\n%s", got)
	}
}

func TestTrendChartDrawsMarkerLines(t *testing.T) {
	got := widgets.TrendChart([]float64{110, 140, 125, 150}, 8, 4, []widgets.Marker{{Label: "SLO", Value: 130, Rune: '─'}})
	if !strings.ContainsRune(got, '─') {
		t.Fatalf("expected marker line in:\n%s", got)
	}
}

func TestTrendChartPositionsMarkersRelativeToSampleRange(t *testing.T) {
	got := widgets.TrendChart([]float64{100, 200}, 4, 4, []widgets.Marker{{Label: "mid", Value: 150, Rune: '─'}})
	lines := strings.Split(got, "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines want 4", len(lines))
	}
	if !strings.ContainsRune(lines[2], '─') {
		t.Fatalf("expected marker on the row for the visible sample range midpoint:\n%s", got)
	}
	if strings.ContainsRune(lines[3], '─') {
		t.Fatalf("expected midpoint marker to render above the bottom row:\n%s", got)
	}
}
