package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/store"
)

func newStoreWithSession(t *testing.T) (store.Store, store.Session) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.NewFS(dir)
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}
	sess := store.Session{
		Name:   "base",
		Config: config.Config{Protocol: "http", TargetURL: "https://example.test"},
	}
	if err := st.SaveSession(context.Background(), sess); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	list, err := st.ListSessions(context.Background())
	if err != nil || len(list) == 0 {
		t.Fatalf("expected one session, err=%v len=%d", err, len(list))
	}
	return st, list[0]
}

func TestSaveAndGetSetRoundTrip(t *testing.T) {
	st, sess := newStoreWithSession(t)
	ctx := context.Background()
	in := store.Set{
		Name:        "auth-regression",
		Description: "smoke + load",
		Stages: []store.Stage{
			{
				Name: "warmup",
				Items: []store.SetItem{
					{Name: "warm", SessionID: sess.ID},
				},
			},
		},
		Thresholds: []store.Threshold{
			{Metric: "p95", Op: "lt", Value: 500, Scope: "aggregate"},
		},
	}
	if err := st.SaveSet(ctx, in); err != nil {
		t.Fatalf("SaveSet: %v", err)
	}
	list, err := st.ListSets(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListSets: err=%v len=%d", err, len(list))
	}
	got, err := st.GetSet(ctx, list[0].ID)
	if err != nil {
		t.Fatalf("GetSet: %v", err)
	}
	if got.Name != in.Name || len(got.Stages) != 1 {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if got.SchemaVersion != store.SchemaVersion {
		t.Errorf("schema_version: got %d want %d", got.SchemaVersion, store.SchemaVersion)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Errorf("timestamps not populated")
	}
}

func TestSaveSetRejectsUnknownSession(t *testing.T) {
	st, _ := newStoreWithSession(t)
	in := store.Set{
		Name: "x",
		Stages: []store.Stage{
			{Name: "s1", Items: []store.SetItem{{Name: "a", SessionID: "01HW_NOT_REAL"}}},
		},
	}
	err := st.SaveSet(context.Background(), in)
	if !errors.Is(err, store.ErrInvalidSet) {
		t.Errorf("got %v, want ErrInvalidSet", err)
	}
}

func TestSaveSetRejectsEmpty(t *testing.T) {
	st, _ := newStoreWithSession(t)
	cases := []struct {
		name string
		in   store.Set
	}{
		{"no name", store.Set{Stages: []store.Stage{{Name: "s", Items: []store.SetItem{{Name: "i", SessionID: "x"}}}}}},
		{"no stages", store.Set{Name: "n"}},
		{"empty stage", store.Set{Name: "n", Stages: []store.Stage{{Name: "s"}}}},
		{"duplicate item name", store.Set{
			Name: "n",
			Stages: []store.Stage{{
				Name: "s",
				Items: []store.SetItem{
					{Name: "dup", SessionID: "x"},
					{Name: "dup", SessionID: "y"},
				},
			}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := st.SaveSet(context.Background(), tc.in); !errors.Is(err, store.ErrInvalidSet) {
				t.Errorf("got %v, want ErrInvalidSet", err)
			}
		})
	}
}

func TestSaveSetRejectsPathTraversalIDs(t *testing.T) {
	st, sess := newStoreWithSession(t)
	bad := []string{"../etc", "..", ".", "a/b", "a\\b", "a\x00b", "a\nb"}
	for _, id := range bad {
		t.Run(id, func(t *testing.T) {
			in := store.Set{
				ID:   id,
				Name: "x",
				Stages: []store.Stage{{
					Name: "s", Items: []store.SetItem{{Name: "i", SessionID: sess.ID}},
				}},
			}
			err := st.SaveSet(context.Background(), in)
			if err == nil {
				t.Fatalf("expected error for id=%q", id)
			}
		})
	}
}

func TestDeleteSetRemovesFile(t *testing.T) {
	st, sess := newStoreWithSession(t)
	in := store.Set{
		Name: "x",
		Stages: []store.Stage{{
			Name: "s", Items: []store.SetItem{{Name: "i", SessionID: sess.ID}},
		}},
	}
	if err := st.SaveSet(context.Background(), in); err != nil {
		t.Fatalf("SaveSet: %v", err)
	}
	list, _ := st.ListSets(context.Background())
	if err := st.DeleteSet(context.Background(), list[0].ID); err != nil {
		t.Fatalf("DeleteSet: %v", err)
	}
	out, _ := st.ListSets(context.Background())
	if len(out) != 0 {
		t.Errorf("expected 0 sets, got %d", len(out))
	}
}

func TestSetRunLifecycle(t *testing.T) {
	st, sess := newStoreWithSession(t)
	ctx := context.Background()
	in := store.Set{
		Name: "x",
		Stages: []store.Stage{{
			Name: "s", Items: []store.SetItem{{Name: "i", SessionID: sess.ID}},
		}},
	}
	if err := st.SaveSet(ctx, in); err != nil {
		t.Fatalf("SaveSet: %v", err)
	}
	list, _ := st.ListSets(ctx)
	setID := list[0].ID

	run, err := st.CreateSetRun(ctx, setID)
	if err != nil {
		t.Fatalf("CreateSetRun: %v", err)
	}
	if run.Dir == "" || run.Status != store.SetRunRunning {
		t.Errorf("bad created run: %+v", run)
	}

	run.Status = store.SetRunCompleted
	run.EndedAt = time.Now().UTC()
	run.AllThresholdsPassed = true
	if err := st.FinalizeSetRun(ctx, run); err != nil {
		t.Fatalf("FinalizeSetRun: %v", err)
	}
	runs, err := st.ListSetRuns(ctx, setID)
	if err != nil || len(runs) != 1 {
		t.Fatalf("ListSetRuns: err=%v len=%d", err, len(runs))
	}
	if runs[0].Status != store.SetRunCompleted {
		t.Errorf("status: got %q want completed", runs[0].Status)
	}
	if !filepath.IsAbs(runs[0].Dir) {
		t.Errorf("Dir not absolute: %q", runs[0].Dir)
	}
}
