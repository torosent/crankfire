package setrunner_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/setrunner"
	"github.com/torosent/crankfire/internal/store"
)

// newStoreWithSession creates a temp-dir FS store with one session saved.
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

type fakeBuilder struct {
	calls  atomic.Int32
	failOn string // item name to fail on
	delay  time.Duration
}

func (f *fakeBuilder) Build(_ context.Context, _ config.Config, itemName string) (setrunner.ItemRun, error) {
	f.calls.Add(1)
	if itemName == f.failOn {
		return setrunner.ItemRun{}, errors.New("synthetic build failure")
	}
	return setrunner.ItemRun{
		Run: func(_ context.Context) (store.RunSummary, error) {
			if f.delay > 0 {
				time.Sleep(f.delay)
			}
			return store.RunSummary{P95Ms: 100}, nil
		},
		Snapshot: func() setrunner.MetricSnapshot { return setrunner.MetricSnapshot{P95: 100, RPS: 50} },
		Cleanup:  func() {},
	}, nil
}

func TestRunnerHappyPath(t *testing.T) {
	st, sess := newStoreWithSession(t)
	in := store.Set{
		Name: "smoke",
		Stages: []store.Stage{{
			Name:  "s1",
			Items: []store.SetItem{{Name: "a", SessionID: sess.ID}, {Name: "b", SessionID: sess.ID}},
		}},
		Thresholds: []store.Threshold{{Metric: "p95", Op: "lt", Value: 500, Scope: "aggregate"}},
	}
	if err := st.SaveSet(context.Background(), in); err != nil {
		t.Fatalf("SaveSet: %v", err)
	}
	list, _ := st.ListSets(context.Background())
	r := setrunner.New(st, &fakeBuilder{})
	run, err := r.Run(context.Background(), list[0].ID, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if run.Status != store.SetRunCompleted {
		t.Errorf("status: %s", run.Status)
	}
	if !run.AllThresholdsPassed {
		t.Errorf("expected thresholds to pass: %+v", run.Thresholds)
	}
	if len(run.Stages) != 1 || len(run.Stages[0].Items) != 2 {
		t.Fatalf("stages: %+v", run.Stages)
	}
	for _, it := range run.Stages[0].Items {
		if it.Status != store.RunStatusCompleted {
			t.Errorf("item %s status: %s", it.Name, it.Status)
		}
	}
}

func TestRunnerStageAbortStopsSubsequentStages(t *testing.T) {
	st, sess := newStoreWithSession(t)
	in := store.Set{
		Name: "abort",
		Stages: []store.Stage{
			{Name: "first", OnFailure: store.OnFailureAbort, Items: []store.SetItem{
				{Name: "fail-here", SessionID: sess.ID},
			}},
			{Name: "second", Items: []store.SetItem{{Name: "never", SessionID: sess.ID}}},
		},
	}
	_ = st.SaveSet(context.Background(), in)
	list, _ := st.ListSets(context.Background())
	r := setrunner.New(st, &fakeBuilder{failOn: "fail-here"})
	run, err := r.Run(context.Background(), list[0].ID, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if run.Status != store.SetRunFailed {
		t.Errorf("status: got %s want failed", run.Status)
	}
	if len(run.Stages) != 1 {
		t.Errorf("expected 1 stage executed, got %d", len(run.Stages))
	}
}

func TestRunnerStageContinueRunsNextStage(t *testing.T) {
	st, sess := newStoreWithSession(t)
	in := store.Set{
		Name: "continue",
		Stages: []store.Stage{
			{Name: "first", OnFailure: store.OnFailureContinue, Items: []store.SetItem{
				{Name: "fail-here", SessionID: sess.ID},
			}},
			{Name: "second", Items: []store.SetItem{{Name: "ok", SessionID: sess.ID}}},
		},
	}
	_ = st.SaveSet(context.Background(), in)
	list, _ := st.ListSets(context.Background())
	r := setrunner.New(st, &fakeBuilder{failOn: "fail-here"})
	run, _ := r.Run(context.Background(), list[0].ID, nil)
	if len(run.Stages) != 2 {
		t.Errorf("expected 2 stages, got %d", len(run.Stages))
	}
}

func TestRunnerCancelStopsCleanly(t *testing.T) {
	st, sess := newStoreWithSession(t)
	in := store.Set{
		Name: "cancel",
		Stages: []store.Stage{{
			Name:  "s",
			Items: []store.SetItem{{Name: "long", SessionID: sess.ID}},
		}},
	}
	_ = st.SaveSet(context.Background(), in)
	list, _ := st.ListSets(context.Background())
	r := setrunner.New(st, &fakeBuilder{delay: 200 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(10 * time.Millisecond); cancel() }()
	run, _ := r.Run(ctx, list[0].ID, nil)
	if run.Status != store.SetRunCancelled {
		t.Errorf("status: got %s want cancelled", run.Status)
	}
}

func TestRunnerEmitsEvents(t *testing.T) {
	st, sess := newStoreWithSession(t)
	in := store.Set{
		Name: "events",
		Stages: []store.Stage{{Name: "s", Items: []store.SetItem{{Name: "i", SessionID: sess.ID}}}},
	}
	_ = st.SaveSet(context.Background(), in)
	list, _ := st.ListSets(context.Background())
	events := make(chan setrunner.Event, 16)
	r := setrunner.New(st, &fakeBuilder{})
	if _, err := r.Run(context.Background(), list[0].ID, events); err != nil {
		t.Fatalf("Run: %v", err)
	}
	close(events)
	var kinds []setrunner.EventKind
	for e := range events {
		kinds = append(kinds, e.Kind)
	}
	if len(kinds) == 0 {
		t.Fatalf("no events emitted")
	}
	first, last := kinds[0], kinds[len(kinds)-1]
	if first != setrunner.EventSetStarted {
		t.Errorf("first event: %s", first)
	}
	if last != setrunner.EventSetEnded {
		t.Errorf("last event: %s", last)
	}
	_ = fmt.Sprint
}
