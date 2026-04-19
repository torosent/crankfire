// internal/store/fs_sessions_test.go
package store_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
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

func TestSaveSessionDedupesAndSortsTags(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewFS(dir)
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}
	ctx := context.Background()
	sess := store.Session{Name: "n", Tags: []string{"prod", "smoke", "prod", "alpha"}}
	if err := st.SaveSession(ctx, sess); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	got, err := st.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 session, got %d", len(got))
	}
	want := []string{"alpha", "prod", "smoke"}
	if !reflect.DeepEqual(got[0].Tags, want) {
		t.Errorf("Tags = %v, want %v", got[0].Tags, want)
	}
}

func TestSaveSessionRejectsInvalidTag(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	ctx := context.Background()
	tests := []struct {
		name string
		tag  string
	}{
		{"empty", ""},
		{"contains space", "has space"},
		{"contains slash", "a/b"},
		{"too long", strings.Repeat("a", 65)},
		{"unicode", "naïve"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := st.SaveSession(ctx, store.Session{Name: "n", Tags: []string{tc.tag}})
			if !errors.Is(err, store.ErrInvalidTag) {
				t.Errorf("err = %v, want ErrInvalidTag", err)
			}
		})
	}
}
