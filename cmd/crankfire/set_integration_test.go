//go:build integration

package main_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/torosent/crankfire/internal/cli"
	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/store"
)

// TestSetRunEndToEnd exercises the entire set run CLI path:
// creates a session and set in the fs store, invokes cli.RunSet directly,
// and asserts the SetRun is persisted with expected status and results.
func TestSetRunEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dir := t.TempDir()
	s, err := store.NewFS(dir)
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}

	ctx := context.Background()

	// Create and save a session
	sess := store.Session{
		Name: "e2e-session",
		Config: config.Config{
			TargetURL:   srv.URL,
			Protocol:    config.ProtocolHTTP,
			Total:       5,
			Concurrency: 1,
		},
	}
	if err := s.SaveSession(ctx, sess); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	// Get the created session ID
	sessions, err := s.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	sessionID := sessions[0].ID

	// Create and save a set with one stage and one item pointing to the session
	set := store.Set{
		SchemaVersion: store.SchemaVersion,
		Name:          "e2e-set",
		Stages: []store.Stage{
			{
				Name: "single",
				Items: []store.SetItem{
					{
						Name:      "hit",
						SessionID: sessionID,
					},
				},
			},
		},
		Thresholds: []store.Threshold{
			{
				Metric: "error_rate",
				Op:     "lt",
				Value:  0.5,
				Scope:  "aggregate",
			},
		},
	}
	if err := s.SaveSet(ctx, set); err != nil {
		t.Fatalf("SaveSet: %v", err)
	}

	// Get the created set ID
	sets, err := s.ListSets(ctx)
	if err != nil {
		t.Fatalf("ListSets: %v", err)
	}
	if len(sets) != 1 {
		t.Fatalf("expected 1 set, got %d", len(sets))
	}
	setID := sets[0].ID

	// Invoke cli.RunSet directly
	exitCode := cli.RunSet(ctx, s, []string{"run", setID}, os.Stdout, os.Stderr)
	if exitCode != 0 {
		t.Fatalf("cli.RunSet returned exit code %d, expected 0", exitCode)
	}

	// Verify SetRun was created and persisted
	setRuns, err := s.ListSetRuns(ctx, setID)
	if err != nil {
		t.Fatalf("ListSetRuns: %v", err)
	}
	if len(setRuns) != 1 {
		t.Fatalf("expected 1 set run, got %d", len(setRuns))
	}

	setRun := setRuns[0]
	if setRun.Status != store.SetRunCompleted {
		t.Errorf("set run status = %q, want %q", setRun.Status, store.SetRunCompleted)
	}

	// Verify stages and items were executed
	if len(setRun.Stages) != 1 {
		t.Errorf("expected 1 stage, got %d", len(setRun.Stages))
	}
	if len(setRun.Stages[0].Items) != 1 {
		t.Errorf("expected 1 item in stage, got %d", len(setRun.Stages[0].Items))
	}

	item := setRun.Stages[0].Items[0]
	if item.Status != store.RunStatusCompleted {
		t.Errorf("item status = %q, want %q", item.Status, store.RunStatusCompleted)
	}

	// Verify summary was populated
	if item.Summary.TotalRequests != 5 {
		t.Errorf("total requests = %d, want 5", item.Summary.TotalRequests)
	}
	if item.Summary.Errors != 0 {
		t.Errorf("errors = %d, want 0", item.Summary.Errors)
	}
	if item.Summary.DurationSec <= 0 {
		t.Errorf("duration_sec = %f, want > 0", item.Summary.DurationSec)
	}
	if item.Summary.P50Ms <= 0 {
		t.Errorf("p50_ms = %f, want > 0", item.Summary.P50Ms)
	}
}
