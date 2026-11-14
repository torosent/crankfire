package cli_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/torosent/crankfire/internal/cli"
	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/store"
)

// --threshold and --override are for one run; they must not be written back
// into the saved set (they used to pile up, and tokens landed on disk).
func TestSetRunFlagsDoNotChangeSavedSet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	ctx := context.Background()
	st, _ := store.NewFS(t.TempDir())
	if err := st.SaveSession(ctx, store.Session{ID: "sess", Name: "s", Config: config.Config{
		TargetURL: srv.URL, Total: 1, Concurrency: 1,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveSet(ctx, store.Set{ID: "set", Name: "n", Stages: []store.Stage{{
		Name: "s", Items: []store.SetItem{{Name: "api", SessionID: "sess"}},
	}}}); err != nil {
		t.Fatal(err)
	}

	var out, errBuf bytes.Buffer
	code := cli.RunSet(ctx, st, []string{
		"run", "--threshold", "p95:lt:60000", "--override", "api.total_requests=2", "set",
	}, &out, &errBuf)
	if code != cli.ExitOK {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("p95 lt 60000")) {
		t.Errorf("the --threshold flag was not applied to the run:\n%s", out.String())
	}

	saved, err := st.GetSet(ctx, "set")
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Thresholds) != 0 || saved.Stages[0].Items[0].Overrides.TotalRequests != nil {
		t.Errorf("run flags were saved into the set: %+v", saved)
	}
}

// A hand-written set file is identified by its file name; the id: field
// inside may be missing.
func TestSetRunUsesFileNameAsID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	ctx := context.Background()
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	if err := st.SaveSession(ctx, store.Session{ID: "sess", Name: "s", Config: config.Config{
		TargetURL: srv.URL, Total: 1, Concurrency: 1,
	}}); err != nil {
		t.Fatal(err)
	}
	setYAML := "name: nightly\nstages:\n  - name: s\n    items:\n      - name: api\n        session_id: sess\n"
	if err := os.WriteFile(filepath.Join(dir, "sets", "nightly.yaml"), []byte(setYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errBuf bytes.Buffer
	if code := cli.RunSet(ctx, st, []string{"run", "nightly"}, &out, &errBuf); code != cli.ExitOK {
		t.Fatalf("code=%d stderr=%s", code, errBuf.String())
	}
	runs, err := st.ListSetRuns(ctx, "nightly")
	if err != nil || len(runs) != 1 {
		t.Fatalf("expected one run stored under the set's file name, got %d (err=%v)", len(runs), err)
	}
}
