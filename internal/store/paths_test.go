// internal/store/paths_test.go
package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/torosent/crankfire/internal/store"
)

func TestResolveDataDir(t *testing.T) {
	t.Run("explicit overrides env and home", func(t *testing.T) {
		t.Setenv("CRANKFIRE_DATA_DIR", "/from/env")
		got, err := store.ResolveDataDir("/explicit")
		if err != nil { t.Fatal(err) }
		if got != "/explicit" { t.Errorf("got %q want %q", got, "/explicit") }
	})

	t.Run("env used when explicit empty", func(t *testing.T) {
		t.Setenv("CRANKFIRE_DATA_DIR", "/from/env")
		got, err := store.ResolveDataDir("")
		if err != nil { t.Fatal(err) }
		if got != "/from/env" { t.Errorf("got %q want %q", got, "/from/env") }
	})

	t.Run("default to home crankfire when nothing set", func(t *testing.T) {
		t.Setenv("CRANKFIRE_DATA_DIR", "")
		home, _ := os.UserHomeDir()
		got, err := store.ResolveDataDir("")
		if err != nil { t.Fatal(err) }
		want := filepath.Join(home, ".crankfire")
		if got != want { t.Errorf("got %q want %q", got, want) }
	})
}
