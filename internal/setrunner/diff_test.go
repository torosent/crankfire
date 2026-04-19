// internal/setrunner/diff_test.go
package setrunner_test

import (
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/setrunner"
	"github.com/torosent/crankfire/internal/store"
)

func mkRun(items ...store.ItemResult) store.SetRun {
	return store.SetRun{
		StartedAt: time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC),
		Status:    store.SetRunCompleted,
		Stages:    []store.StageResult{{Name: "s", Items: items}},
	}
}

func mkItem(name string, p95 float64, errs, total int64, dur float64) store.ItemResult {
	return store.ItemResult{
		Name:    name,
		Status:  store.RunStatusCompleted,
		Summary: store.RunSummary{P95Ms: p95, Errors: errs, TotalRequests: total, DurationSec: dur},
	}
}

func TestDiffUnchanged(t *testing.T) {
	a := mkRun(mkItem("api", 100, 0, 1000, 10))
	b := mkRun(mkItem("api", 100, 0, 1000, 10))
	got := setrunner.Diff(a, b)
	if got.OverallVerdict != "unchanged" {
		t.Errorf("verdict = %q, want unchanged", got.OverallVerdict)
	}
}

func TestDiffRegression(t *testing.T) {
	a := mkRun(mkItem("api", 100, 0, 1000, 10))
	b := mkRun(mkItem("api", 110, 0, 1000, 10)) // P95 +10%
	got := setrunner.Diff(a, b)
	if got.OverallVerdict != "regressed" {
		t.Errorf("verdict = %q, want regressed", got.OverallVerdict)
	}
	if got.Rows[0].P95DeltaMs != 10 {
		t.Errorf("P95DeltaMs = %v, want 10", got.Rows[0].P95DeltaMs)
	}
}

func TestDiffImprovement(t *testing.T) {
	a := mkRun(mkItem("api", 100, 50, 1000, 10))
	b := mkRun(mkItem("api", 90, 0, 1000, 10))
	got := setrunner.Diff(a, b)
	if got.OverallVerdict != "improved" {
		t.Errorf("verdict = %q, want improved", got.OverallVerdict)
	}
}

func TestDiffMixed(t *testing.T) {
	a := mkRun(mkItem("api", 100, 0, 1000, 10), mkItem("auth", 50, 0, 500, 10))
	b := mkRun(mkItem("api", 110, 0, 1000, 10), mkItem("auth", 40, 0, 500, 10))
	got := setrunner.Diff(a, b)
	if got.OverallVerdict != "mixed" {
		t.Errorf("verdict = %q, want mixed", got.OverallVerdict)
	}
}

func TestDiffMissingItemsMarkedAbsent(t *testing.T) {
	a := mkRun(mkItem("api", 100, 0, 1000, 10))
	b := mkRun(mkItem("auth", 100, 0, 1000, 10))
	got := setrunner.Diff(a, b)
	if len(got.Rows) != 2 {
		t.Fatalf("rows=%d, want 2 (union)", len(got.Rows))
	}
	for _, r := range got.Rows {
		if r.ItemName == "api" && (!r.APresent || r.BPresent) {
			t.Errorf("api: APresent=%v BPresent=%v", r.APresent, r.BPresent)
		}
		if r.ItemName == "auth" && (r.APresent || !r.BPresent) {
			t.Errorf("auth: APresent=%v BPresent=%v", r.APresent, r.BPresent)
		}
	}
}
