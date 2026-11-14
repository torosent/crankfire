package store

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Session files hold client secrets, passwords and tokens.
func TestSessionFilesArePrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits")
	}
	dir := t.TempDir()
	st, err := NewFS(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveSession(context.Background(), Session{ID: "s1", Name: "x"}); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]os.FileMode{
		filepath.Join(dir, "sessions"):            0o700,
		filepath.Join(dir, "sessions", "s1.yaml"): 0o600,
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s: mode %v, want %v", path, got, want)
		}
	}
}
