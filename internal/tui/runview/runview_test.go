package runview_test

import (
	"strings"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/tui/runview"
)

func TestRunViewRendersFromSnapshot(t *testing.T) {
	rv := runview.New(runview.Options{Title: "Checkout API smoke", Total: 50000})
	rv2, _ := rv.Update(runview.SnapshotMsg{Snap: metrics.DataPoint{TotalRequests: 17250, CurrentRPS: 487}})
	out := rv2.View()
	for _, want := range []string{"Checkout API smoke", "17", "487", "Throughput", "Latency"} {
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
