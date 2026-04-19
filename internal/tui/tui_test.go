package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tui"
)

func TestRootRendersPlaceholder(t *testing.T) {
	s, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := tui.NewRoot(s)
	view := m.View()
	if !strings.Contains(view, "Sessions") {
		t.Errorf("View did not contain %q:\n%s", "Sessions", view)
	}
	// Quitting on q should issue tea.Quit
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("expected quit cmd on q")
	}
}
