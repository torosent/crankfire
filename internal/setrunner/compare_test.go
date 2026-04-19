package setrunner_test

import (
	"testing"

	"github.com/torosent/crankfire/internal/setrunner"
	"github.com/torosent/crankfire/internal/store"
)

func TestCompareEmpty(t *testing.T) {
	c := setrunner.Compare(nil)
	if len(c.Items) != 0 || len(c.Rows) != 0 {
		t.Errorf("expected empty: %+v", c)
	}
}

func TestCompareOrdersByInputAndPicksWinners(t *testing.T) {
	items := []store.ItemResult{
		{Name: "a", Status: store.RunStatusCompleted, Summary: store.RunSummary{P95Ms: 200, TotalRequests: 100, Errors: 1, DurationSec: 1.0}},
		{Name: "b", Status: store.RunStatusCompleted, Summary: store.RunSummary{P95Ms: 150, TotalRequests: 200, Errors: 10, DurationSec: 1.0}},
	}
	c := setrunner.Compare(items)
	if got, want := c.Items, []string{"a", "b"}; !equal(got, want) {
		t.Errorf("Items order: got %v want %v", got, want)
	}
	row := findRow(c.Rows, "p95")
	if row.Winner != "b" {
		t.Errorf("p95 winner: got %q want b (lower-is-better)", row.Winner)
	}
	row = findRow(c.Rows, "rps")
	if row.Winner != "b" {
		t.Errorf("rps winner: got %q want b (higher-is-better)", row.Winner)
	}
	row = findRow(c.Rows, "error_rate")
	if row.Winner != "a" {
		t.Errorf("error_rate winner: got %q want a", row.Winner)
	}
}

func TestCompareSkipsNonCompletedFromWinner(t *testing.T) {
	items := []store.ItemResult{
		{Name: "ok", Status: store.RunStatusCompleted, Summary: store.RunSummary{P95Ms: 300}},
		{Name: "fail", Status: store.RunStatusFailed, Summary: store.RunSummary{P95Ms: 100}},
	}
	c := setrunner.Compare(items)
	row := findRow(c.Rows, "p95")
	if row.Winner != "ok" {
		t.Errorf("failed item must not win: got %q", row.Winner)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func findRow(rs []setrunner.CompareRow, metric string) setrunner.CompareRow {
	for _, r := range rs {
		if r.Metric == metric {
			return r
		}
	}
	return setrunner.CompareRow{}
}
