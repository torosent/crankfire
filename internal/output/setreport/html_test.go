package setreport_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/output/setreport"
	"github.com/torosent/crankfire/internal/store"
)

var update = flag.Bool("update", false, "update golden files")

func TestRenderHTMLGolden(t *testing.T) {
	run := store.SetRun{
		SetID:               "01HX",
		SetName:             "auth-regression",
		StartedAt:           time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC),
		EndedAt:             time.Date(2026, 4, 19, 10, 5, 0, 0, time.UTC),
		Status:              store.SetRunCompleted,
		AllThresholdsPassed: true,
		Stages: []store.StageResult{
			{
				Name: "warmup",
				Items: []store.ItemResult{
					{Name: "warm", SessionID: "s1", Status: store.RunStatusCompleted,
						Summary: store.RunSummary{P50Ms: 80, P95Ms: 200, P99Ms: 400, TotalRequests: 50, Errors: 0, DurationSec: 1}},
				},
			},
			{
				Name: "load",
				Items: []store.ItemResult{
					{Name: "login", SessionID: "s1", Status: store.RunStatusCompleted,
						Summary: store.RunSummary{P50Ms: 100, P95Ms: 250, P99Ms: 500, TotalRequests: 100, Errors: 0, DurationSec: 1}},
					{Name: "search", SessionID: "s2", Status: store.RunStatusCompleted,
						Summary: store.RunSummary{P50Ms: 150, P95Ms: 300, P99Ms: 600, TotalRequests: 80, Errors: 2, DurationSec: 1}},
				},
			},
		},
		Thresholds: []store.ThresholdResult{
			{Threshold: store.Threshold{Metric: "p95", Op: "lt", Value: 500, Scope: "aggregate"}, Actual: 300, Passed: true},
		},
	}
	got, err := setreport.Render(run)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	goldenPath := filepath.Join("testdata", "expected.html")
	if *update {
		_ = os.MkdirAll("testdata", 0o755)
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with -update first time): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("HTML mismatch — re-run with -update if intentional.\n--- diff start ---\n%s\n--- diff end ---", diff(string(want), string(got)))
	}
}

func TestRenderHTMLContainsCompareTable(t *testing.T) {
	run := store.SetRun{
		Stages: []store.StageResult{{Name: "s", Items: []store.ItemResult{
			{Name: "a", Status: store.RunStatusCompleted, Summary: store.RunSummary{P95Ms: 100}},
			{Name: "b", Status: store.RunStatusCompleted, Summary: store.RunSummary{P95Ms: 200}},
		}}},
	}
	out, err := setreport.Render(run)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"compare-table", "p95", "winner-a"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q in HTML", want)
		}
	}
}

func diff(a, b string) string {
	if len(a) > 400 {
		a = a[:400]
	}
	if len(b) > 400 {
		b = b[:400]
	}
	return "want:\n" + a + "\n\ngot:\n" + b
}
