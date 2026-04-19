package screens_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tui/screens"
)

func TestDetailShowsSession(t *testing.T) {
	sess := store.Session{ID: "abc", Name: "smoke", Description: "quick"}
	m := screens.NewDetail(nil, sess)
	view := m.View()
	for _, want := range []string{"smoke", "quick", "abc"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q:\n%s", want, view)
		}
	}
}

func TestConfirmYesEmitsConfirmed(t *testing.T) {
	var got bool
	m := screens.NewConfirm("Delete?", func() { got = true })
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if !got {
		t.Error("expected callback invocation on y")
	}
}
