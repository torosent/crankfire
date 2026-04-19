package store_test

import (
"context"
"errors"
"os"
"path/filepath"
"strings"
"testing"

"github.com/torosent/crankfire/internal/config"
"github.com/torosent/crankfire/internal/store"
)

func TestSaveSessionRejectsPathTraversalIDs(t *testing.T) {
dir := t.TempDir()
st, err := store.NewFS(dir)
if err != nil {
t.Fatalf("NewFS: %v", err)
}

cases := []string{
"../../../etc/passwd",
"..",
".",
"foo/bar",
"foo\\bar",
"a/../b",
"foo\x00bar",
}

for _, id := range cases {
t.Run(id, func(t *testing.T) {
sess := store.Session{
ID:     id,
Name:   "x",
Config: config.Config{TargetURL: "http://x"},
}
err := st.SaveSession(context.Background(), sess)
if err == nil {
t.Fatalf("expected error for id %q, got nil", id)
}
if !errors.Is(err, store.ErrInvalidSession) {
t.Fatalf("expected ErrInvalidSession for id %q, got %v", id, err)
}
})
}

// And confirm no escape file was created.
bad := filepath.Join(dir, "..", "etc", "passwd.yaml")
if _, err := os.Stat(bad); err == nil {
t.Fatalf("path traversal succeeded; file at %s exists", bad)
}
// And nothing landed in the sessions dir.
entries, _ := os.ReadDir(filepath.Join(dir, "sessions"))
for _, e := range entries {
if strings.Contains(e.Name(), "passwd") {
t.Fatalf("unexpected file in sessions dir: %s", e.Name())
}
}
}

func TestSaveSessionEmptyIDStillGeneratesULID(t *testing.T) {
dir := t.TempDir()
st, _ := store.NewFS(dir)
sess := store.Session{Name: "x", Config: config.Config{TargetURL: "http://x"}}
if err := st.SaveSession(context.Background(), sess); err != nil {
t.Fatalf("SaveSession with empty id: %v", err)
}
got, _ := st.ListSessions(context.Background())
if len(got) != 1 || got[0].ID == "" {
t.Fatalf("expected exactly one session with generated id, got %+v", got)
}
}
