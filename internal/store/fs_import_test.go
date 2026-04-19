// internal/store/fs_import_test.go
package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/torosent/crankfire/internal/store"
)

func TestImportSessionFromConfigFile(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.yaml")
	cfg := []byte("target: https://example.com\nprotocol: http\ntotal: 5\n")
	if err := os.WriteFile(cfgPath, cfg, 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := s.ImportSessionFromConfigFile(context.Background(), cfgPath, "imported")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Name != "imported" {
		t.Errorf("got name %q want imported", sess.Name)
	}
	if sess.Config.TargetURL != "https://example.com" {
		t.Errorf("got target %q want https://example.com", sess.Config.TargetURL)
	}
}
