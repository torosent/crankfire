package screens_test

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tui/screens"
)

func setupStore(t *testing.T, names ...string) store.Store {
	t.Helper()
	s, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		sess := store.Session{Name: n}
		if err := s.SaveSession(context.Background(), sess); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestListShowsSessions(t *testing.T) {
	s := setupStore(t, "alpha", "beta")
	m := screens.NewList(s)
	
	// Simulate the async load by calling the Init function directly
	initCmd := m.Init()
	loadedMsg := initCmd().(screens.LoadedMsg)
	
	next, _ := m.Update(loadedMsg)
	m = next.(screens.List)
	view := m.View()
	for _, want := range []string{"alpha", "beta"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}

func TestListQuitKey(t *testing.T) {
	s := setupStore(t)
	m := screens.NewList(s)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("expected non-nil cmd for quit")
	}
}
