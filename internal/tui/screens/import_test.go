package screens_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tui/screens"
)

func TestImportFromFile(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "c.yaml")
	os.WriteFile(cfg, []byte("target: https://example.com\nprotocol: http\ntotal: 5\n"), 0o644)
	s, _ := store.NewFS(t.TempDir())
	m := screens.NewImport(s)
	var cur tea.Model = m
	cur, _ = cur.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(cfg)})
	cur, _ = cur.Update(tea.KeyMsg{Type: tea.KeyTab})
	cur, _ = cur.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("imported")})
	cur, _ = cur.Update(tea.KeyMsg{Type: tea.KeyEnter})
	list, _ := s.ListSessions(context.Background())
	if len(list) != 1 || list[0].Name != "imported" {
		t.Fatalf("expected 1 imported session, got %#v", list)
	}
}
