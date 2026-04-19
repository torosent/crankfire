package screens_test

import (
	"context"
	"strings"
	"testing"

	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tui/screens"
)

func TestSetCompareRendersTableWithWinners(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	run := store.SetRun{
		SetName: "smoke",
		Stages: []store.StageResult{{Name: "s", Items: []store.ItemResult{
			{Name: "a", Status: store.RunStatusCompleted, Summary: store.RunSummary{P95Ms: 200, TotalRequests: 100, DurationSec: 1}},
			{Name: "b", Status: store.RunStatusCompleted, Summary: store.RunSummary{P95Ms: 100, TotalRequests: 200, DurationSec: 1}},
		}}},
	}
	m := screens.NewSetCompare(context.Background(), st, run)
	m.Init()
	view := m.View()
	for _, want := range []string{"a", "b", "p95", "rps"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q\n%s", want, view)
		}
	}
	if !strings.Contains(view, "★") {
		t.Errorf("expected winner marker:\n%s", view)
	}
}
