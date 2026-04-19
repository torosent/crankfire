// internal/scheduler/manager_test.go
package scheduler_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/scheduler"
	"github.com/torosent/crankfire/internal/store"
)

// fakeStore for Reload tests.
type fakeStore struct {
	sets []store.Set
}

func (f *fakeStore) ListSets(ctx context.Context) ([]store.Set, error) { return f.sets, nil }

// Stub the rest of the Store interface (only ListSets is used here).
// Add minimal no-op implementations matching the current interface.
// (This block will need updating when Store changes — keep it terse.)
func (f *fakeStore) ListSessions(ctx context.Context) ([]store.Session, error) { return nil, nil }
func (f *fakeStore) GetSession(ctx context.Context, id string) (store.Session, error) {
	return store.Session{}, store.ErrNotFound
}
func (f *fakeStore) SaveSession(ctx context.Context, s store.Session) error { return nil }
func (f *fakeStore) DeleteSession(ctx context.Context, id string) error     { return nil }
func (f *fakeStore) ImportSessionFromConfigFile(ctx context.Context, path, name string) (store.Session, error) {
	return store.Session{}, nil
}
func (f *fakeStore) ListRuns(ctx context.Context, sessionID string) ([]store.Run, error) {
	return nil, nil
}
func (f *fakeStore) CreateRun(ctx context.Context, sessionID string) (store.Run, error) {
	return store.Run{}, nil
}
func (f *fakeStore) FinalizeRun(ctx context.Context, run store.Run, summary store.RunSummary) error {
	return nil
}
func (f *fakeStore) GetSet(ctx context.Context, id string) (store.Set, error) {
	for _, s := range f.sets {
		if s.ID == id {
			return s, nil
		}
	}
	return store.Set{}, store.ErrNotFound
}
func (f *fakeStore) SaveSet(ctx context.Context, s store.Set) error { return nil }
func (f *fakeStore) DeleteSet(ctx context.Context, id string) error { return nil }
func (f *fakeStore) ListSetRuns(ctx context.Context, setID string) ([]store.SetRun, error) {
	return nil, nil
}
func (f *fakeStore) CreateSetRun(ctx context.Context, setID string) (store.SetRun, error) {
	return store.SetRun{}, nil
}
func (f *fakeStore) FinalizeSetRun(ctx context.Context, run store.SetRun) error { return nil }
func (f *fakeStore) ListTemplates(ctx context.Context) ([]string, error)         { return nil, nil }
func (f *fakeStore) GetTemplate(ctx context.Context, id string) ([]byte, error) {
	return nil, store.ErrNotFound
}
func (f *fakeStore) SaveTemplate(ctx context.Context, id string, body []byte) error { return nil }
func (f *fakeStore) DeleteTemplate(ctx context.Context, id string) error            { return nil }

func TestManagerAddOrReplaceFires(t *testing.T) {
	var fires int32
	done := make(chan struct{}, 1)
	mgr := scheduler.New(func(ctx context.Context, setID string) {
		if atomic.AddInt32(&fires, 1) == 1 {
			select {
			case done <- struct{}{}:
			default:
			}
		}
	})
	if err := mgr.AddOrReplace("set1", "@every 100ms"); err != nil {
		t.Fatalf("AddOrReplace: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go mgr.Run(ctx)
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("timeout waiting for first fire")
	}
	if atomic.LoadInt32(&fires) < 1 {
		t.Errorf("fires = %d, want >=1", fires)
	}
}

func TestManagerRemoveStopsFiring(t *testing.T) {
	var fires int32
	mgr := scheduler.New(func(ctx context.Context, setID string) {
		atomic.AddInt32(&fires, 1)
	})
	if err := mgr.AddOrReplace("set1", "@every 50ms"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go mgr.Run(ctx)
	time.Sleep(150 * time.Millisecond)
	mgr.Remove("set1")
	before := atomic.LoadInt32(&fires)
	time.Sleep(200 * time.Millisecond)
	after := atomic.LoadInt32(&fires)
	// Allow up to one extra fire that was already in flight.
	if after-before > 1 {
		t.Errorf("after Remove, fires went from %d to %d", before, after)
	}
}

func TestManagerInvalidExprError(t *testing.T) {
	mgr := scheduler.New(func(ctx context.Context, setID string) {})
	if err := mgr.AddOrReplace("set1", "not-a-cron"); err == nil {
		t.Error("expected error for invalid expression")
	}
}

func TestManagerReloadAddsRemovesReplaces(t *testing.T) {
	mgr := scheduler.New(func(ctx context.Context, setID string) {})
	st := &fakeStore{sets: []store.Set{
		{ID: "a", Schedule: "@daily"},
		{ID: "b", Schedule: "@hourly"},
		{ID: "c", Schedule: ""}, // no schedule, should be ignored
	}}
	ctx := context.Background()
	if err := mgr.Reload(ctx, st); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if got := mgr.EntryCount(); got != 2 {
		t.Errorf("EntryCount = %d, want 2", got)
	}
	// Replace b's schedule, drop a, keep c (still no schedule).
	st.sets = []store.Set{
		{ID: "b", Schedule: "@every 5m"},
		{ID: "c", Schedule: ""},
	}
	if err := mgr.Reload(ctx, st); err != nil {
		t.Fatal(err)
	}
	if got := mgr.EntryCount(); got != 1 {
		t.Errorf("after reload EntryCount = %d, want 1", got)
	}
}

func TestManagerSkipsOverlappingFires(t *testing.T) {
	var fires int32
	var inflight sync.WaitGroup
	gate := make(chan struct{})
	mgr := scheduler.New(func(ctx context.Context, setID string) {
		atomic.AddInt32(&fires, 1)
		inflight.Add(1)
		defer inflight.Done()
		<-gate
	})
	_ = mgr.AddOrReplace("set1", "@every 50ms")
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	go mgr.Run(ctx)
	time.Sleep(500 * time.Millisecond) // multiple ticks; only one should fire
	close(gate)
	inflight.Wait()
	if got := atomic.LoadInt32(&fires); got > 2 {
		t.Errorf("overlapping policy: fires = %d, expected <=2", got)
	}
}
