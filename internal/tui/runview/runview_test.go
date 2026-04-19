package runview_test

import (
	"strings"
	"testing"

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
