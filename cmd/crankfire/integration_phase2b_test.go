//go:build integration

package main_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/cli"
	"github.com/torosent/crankfire/internal/setrunner"
	"github.com/torosent/crankfire/internal/store"
)

func TestPhase2bEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dir := t.TempDir()
	t.Setenv("CRANKFIRE_DATA_DIR", dir)
	st, err := store.NewFS(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// Tagged sessions
	if err := st.SaveSession(ctx, store.Session{Name: "prod-sess", Tags: []string{"prod"}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveSession(ctx, store.Session{Name: "dev-sess", Tags: []string{"dev"}}); err != nil {
		t.Fatal(err)
	}
	sessions, _ := st.ListSessions(ctx)
	var prodID string
	for _, s := range sessions {
		if s.Name == "prod-sess" {
			prodID = s.ID
		}
	}
	if prodID == "" {
		t.Fatal("missing prod session")
	}

	// Template
	tplBody := []byte("template: true\nname: api-{{ .Env }}\ndescription: e2e\nstages:\n- name: smoke\n  items:\n  - name: api\n    session_id: " + prodID + "\n")
	if err := st.SaveTemplate(ctx, "api-baseline", tplBody); err != nil {
		t.Fatal(err)
	}

	// Materialize
	var stdout, stderr strings.Builder
	code := cli.RunSet(ctx, st, []string{"new", "--from-template", "api-baseline", "--param", "Env=prod"}, &stdout, &stderr)
	if code != cli.ExitOK {
		t.Fatalf("set new code=%d stderr=%s", code, stderr.String())
	}
	newSetID := strings.TrimSpace(stdout.String())
	if newSetID == "" {
		t.Fatal("set new produced no ID")
	}

	// Schedule
	set, _ := st.GetSet(ctx, newSetID)
	set.Schedule = "@every 200ms"
	if err := st.SaveSet(ctx, set); err != nil {
		t.Fatalf("save set: %v", err)
	}

	// Daemon
	daemonCtx, daemonCancel := context.WithCancel(ctx)
	var stdoutD, stderrD strings.Builder
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = cli.RunDaemon(daemonCtx, st, dir, nil, &stdoutD, &stderrD)
	}()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		runs, _ := st.ListSetRuns(ctx, newSetID)
		if len(runs) >= 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Manual second run
	runner := setrunner.New(st, cli.NewSetBuilder())
	if _, err := runner.Run(ctx, newSetID, nil); err != nil {
		t.Logf("manual run err (acceptable): %v", err)
	}

	runs, _ := st.ListSetRuns(ctx, newSetID)
	if len(runs) < 2 {
		daemonCancel()
		wg.Wait()
		t.Fatalf("want >=2 runs, got %d (daemon stdout: %s)", len(runs), stdoutD.String())
	}
	idA := filepath.Base(runs[0].Dir)
	idB := filepath.Base(runs[1].Dir)
	var diffOut, diffErr strings.Builder
	if code := cli.RunSet(ctx, st, []string{"diff", idA, idB}, &diffOut, &diffErr); code != cli.ExitOK {
		t.Errorf("set diff code=%d stderr=%s", code, diffErr.String())
	}
	if !strings.Contains(diffOut.String(), "verdict:") {
		t.Errorf("diff output missing verdict:\n%s", diffOut.String())
	}

	daemonCancel()
	wg.Wait()
	if !strings.Contains(stdoutD.String(), `"event":"stopped"`) {
		t.Errorf("daemon did not log stopped: %s", stdoutD.String())
	}
}
