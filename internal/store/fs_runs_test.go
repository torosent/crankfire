// internal/store/fs_runs_test.go
package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/torosent/crankfire/internal/store"
)

func TestRunsLifecycle(t *testing.T) {
	s, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	sess := newSession(t, "runs")
	if err := s.SaveSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	list, _ := s.ListSessions(ctx)
	sid := list[0].ID

	run, err := s.CreateRun(ctx, sid)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != store.RunStatusRunning {
		t.Errorf("got %q want running", run.Status)
	}
	if _, err := os.Stat(run.Dir); err != nil {
		t.Fatalf("run dir missing: %v", err)
	}

	// simulate that the runner wrote result.json + report.html
	for _, f := range []string{"result.json", "report.html"} {
		if err := os.WriteFile(filepath.Join(run.Dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	summary := store.RunSummary{TotalRequests: 100, Errors: 1, P95Ms: 42}
	if err := s.FinalizeRun(ctx, run, summary); err != nil {
		t.Fatal(err)
	}

	runs, err := s.ListRuns(ctx, sid)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("got %d runs want 1", len(runs))
	}
	if runs[0].Status != store.RunStatusCompleted {
		t.Errorf("got status %q want completed", runs[0].Status)
	}
	if runs[0].Summary.TotalRequests != 100 {
		t.Errorf("summary not persisted")
	}
}
