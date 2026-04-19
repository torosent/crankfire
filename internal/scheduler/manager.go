// Package scheduler manages cron entries for sets and fires runs via
// a caller-supplied callback. Wraps github.com/robfig/cron/v3.
package scheduler

import (
	"context"
	"fmt"
	"sync"

	cron "github.com/robfig/cron/v3"

	"github.com/torosent/crankfire/internal/store"
)

// SetLister is the slice of store.Store the Manager actually needs.
// Defined narrowly so tests can fake just one method.
type SetLister interface {
	ListSets(ctx context.Context) ([]store.Set, error)
}

// FireFunc is invoked when a scheduled set is due.
// The ctx is a child of the Manager's Run ctx; if it's already cancelled
// the Manager skips the fire entirely.
type FireFunc func(ctx context.Context, setID string)

// entryRec pairs a cron EntryID with the original expression string so
// Reload can diff without relying on cron.Schedule implementing fmt.Stringer.
type entryRec struct {
	id   cron.EntryID
	expr string
}

// Manager wraps cron.Cron with per-set replace/remove bookkeeping
// and a single-flight policy: a set never has two concurrent fires.
type Manager struct {
	mu       sync.Mutex
	cron     *cron.Cron
	entries  map[string]entryRec // setID -> entry
	parser   cron.Parser
	onFire   FireFunc
	inFlight sync.Map // setID -> struct{}
	runCtx   context.Context
}

// New constructs a Manager. The cron is created but not started.
func New(onFire FireFunc) *Manager {
	return &Manager{
		cron:    cron.New(),
		entries: make(map[string]entryRec),
		parser:  cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor),
		onFire:  onFire,
	}
}

// AddOrReplace registers (or replaces) the schedule for setID.
// Returns an error if expr is not a valid cron expression.
func (m *Manager) AddOrReplace(setID, expr string) error {
	if _, err := m.parser.Parse(expr); err != nil {
		return fmt.Errorf("parse cron %q: %w", expr, err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if rec, ok := m.entries[setID]; ok {
		m.cron.Remove(rec.id)
	}
	id, err := m.cron.AddFunc(expr, func() { m.fire(setID) })
	if err != nil {
		return fmt.Errorf("add cron: %w", err)
	}
	m.entries[setID] = entryRec{id: id, expr: expr}
	return nil
}

// Remove cancels the schedule for setID. No-op if not registered.
func (m *Manager) Remove(setID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rec, ok := m.entries[setID]; ok {
		m.cron.Remove(rec.id)
		delete(m.entries, setID)
	}
}

// Reload diffs the desired set schedules (from the store) against the
// current entries: adds new, removes deleted, replaces changed.
func (m *Manager) Reload(ctx context.Context, st SetLister) error {
	sets, err := st.ListSets(ctx)
	if err != nil {
		return fmt.Errorf("list sets: %w", err)
	}
	desired := make(map[string]string, len(sets))
	for _, s := range sets {
		if s.Schedule != "" {
			desired[s.ID] = s.Schedule
		}
	}
	// Snapshot current expressions so we can diff outside the lock.
	m.mu.Lock()
	current := make(map[string]string, len(m.entries))
	for setID, rec := range m.entries {
		current[setID] = rec.expr
	}
	m.mu.Unlock()
	// Add or update.
	for setID, expr := range desired {
		if cur, ok := current[setID]; ok && cur == expr {
			continue
		}
		if err := m.AddOrReplace(setID, expr); err != nil {
			return fmt.Errorf("reload %s: %w", setID, err)
		}
	}
	// Remove gone.
	for setID := range current {
		if _, ok := desired[setID]; !ok {
			m.Remove(setID)
		}
	}
	return nil
}

// EntryCount returns the number of currently-scheduled entries.
// Useful in tests.
func (m *Manager) EntryCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries)
}

// Run starts the scheduler and blocks until ctx is done.
// On exit, it stops the cron and waits for the stop context to drain.
func (m *Manager) Run(ctx context.Context) {
	m.mu.Lock()
	m.runCtx = ctx
	m.mu.Unlock()
	m.cron.Start()
	<-ctx.Done()
	stopCtx := m.cron.Stop()
	<-stopCtx.Done()
}

// fire is invoked by cron at scheduled times. Skips the fire if the
// set is already in-flight (single-flight per set).
func (m *Manager) fire(setID string) {
	if _, busy := m.inFlight.LoadOrStore(setID, struct{}{}); busy {
		return
	}
	defer m.inFlight.Delete(setID)
	m.mu.Lock()
	ctx := m.runCtx
	m.mu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	m.onFire(ctx, setID)
}
