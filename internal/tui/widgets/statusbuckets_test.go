package widgets_test

import (
	"strings"
	"testing"

	"github.com/torosent/crankfire/internal/tui/widgets"
)

func TestStatusBucketsTableRendersTopRows(t *testing.T) {
	buckets := map[string]map[string]int{
		"http": {"500": 12, "503": 4},
		"grpc": {"UNAVAILABLE": 3},
	}
	out := widgets.StatusBucketsTable(buckets, 10)
	for _, want := range []string{"HTTP 500 12", "HTTP 503 4", "GRPC UNAVAILABLE 3"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestStatusBucketsTableEmptyReturnsEmpty(t *testing.T) {
	if got := widgets.StatusBucketsTable(nil, 10); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestStatusBucketsTableLimitsRows(t *testing.T) {
	buckets := map[string]map[string]int{
		"http": {"500": 5, "501": 4, "502": 3, "503": 2, "504": 1},
	}
	out := widgets.StatusBucketsTable(buckets, 2)
	if got := strings.Count(out, "\n"); got > 2 {
		t.Errorf("expected at most 2 rows, got %d:\n%s", got, out)
	}
}
