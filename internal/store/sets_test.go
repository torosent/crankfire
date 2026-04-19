package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/store"
)

func TestSetTypeZeroValuesAreUsable(t *testing.T) {
	s := store.Set{
		SchemaVersion: store.SchemaVersion,
		ID:            "01HXSET1",
		Name:          "smoke",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Stages: []store.Stage{
			{Name: "warmup", Items: []store.SetItem{{Name: "warm", SessionID: "01HW"}}},
		},
	}
	if got, want := len(s.Stages), 1; got != want {
		t.Fatalf("stages: got %d want %d", got, want)
	}
	if got, want := s.Stages[0].Items[0].Name, "warm"; got != want {
		t.Errorf("item name: got %q want %q", got, want)
	}
}

func TestNewFSCreatesSetsDirs(t *testing.T) {
	dir := t.TempDir()
	if _, err := store.NewFS(dir); err != nil {
		t.Fatalf("NewFS: %v", err)
	}
	for _, sub := range []string{"sets", filepath.Join("runs", "sets")} {
		p := filepath.Join(dir, sub)
		fi, err := os.Stat(p)
		if err != nil {
			t.Errorf("missing dir %s: %v", p, err)
			continue
		}
		if !fi.IsDir() {
			t.Errorf("%s: expected dir", p)
		}
	}
}
