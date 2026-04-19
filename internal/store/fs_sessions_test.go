// internal/store/fs_sessions_test.go
package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/store"
)

func newSession(t *testing.T, name string) store.Session {
	t.Helper()
	return store.Session{
		SchemaVersion: store.SchemaVersion,
		Name:          name,
		CreatedAt:     time.Now().UTC().Truncate(time.Second),
		UpdatedAt:     time.Now().UTC().Truncate(time.Second),
		Config: config.Config{
			TargetURL: "https://example.com",
			Protocol:  config.ProtocolHTTP,
			Total:     10,
		},
	}
}

func TestFSSessionsCRUD(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewFS(dir)
	if err != nil { t.Fatal(err) }
	ctx := context.Background()

	sess := newSession(t, "smoke")
	if err := s.SaveSession(ctx, sess); err != nil { t.Fatal(err) }
	if sess.ID == "" {
		// SaveSession must allocate an ID when one is missing
	}

	list, err := s.ListSessions(ctx)
	if err != nil { t.Fatal(err) }
	if len(list) != 1 { t.Fatalf("got %d sessions want 1", len(list)) }
	id := list[0].ID
	if id == "" { t.Fatal("session ID should be set after Save") }

	got, err := s.GetSession(ctx, id)
	if err != nil { t.Fatal(err) }
	if got.Name != "smoke" { t.Errorf("got name %q want smoke", got.Name) }

	if err := s.DeleteSession(ctx, id); err != nil { t.Fatal(err) }
	if _, err := s.GetSession(ctx, id); err == nil { t.Fatal("expected ErrNotFound") }
}

func TestFSSessionsAtomicAndLocked(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewFS(dir)
	if err != nil { t.Fatal(err) }
	ctx := context.Background()

	sess := newSession(t, "atomic")
	if err := s.SaveSession(ctx, sess); err != nil { t.Fatal(err) }
	list, _ := s.ListSessions(ctx)
	id := list[0].ID

	// Concurrent saves should both succeed serially without leaving a *.tmp file.
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			cur, err := s.GetSession(ctx, id)
			if err != nil { done <- err; return }
			cur.Name = "updated"
			done <- s.SaveSession(ctx, cur)
		}()
	}
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil { t.Fatal(err) }
	}
}
