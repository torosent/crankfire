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

func newMemStoreWithSessions(t *testing.T, sessions []store.Session) store.Store {
	t.Helper()
	s, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, sess := range sessions {
		if err := s.SaveSession(context.Background(), sess); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func sendKeysToList(t *testing.T, l screens.List, keys []tea.KeyMsg) screens.List {
	t.Helper()
	var m tea.Model = l
	for _, k := range keys {
		m, _ = m.Update(k)
	}
	return m.(screens.List)
}

func TestListFiltersBySlash(t *testing.T) {
	st := newMemStoreWithSessions(t, []store.Session{
		{ID: "01F8MECHZX3TBDSZ7XR9PFE7M0", Name: "alpha", Tags: []string{"prod"}},
		{ID: "01F8MECHZX3TBDSZ7XR9PFE7M1", Name: "beta", Tags: []string{"staging"}},
		{ID: "01F8MECHZX3TBDSZ7XR9PFE7M2", Name: "gamma", Tags: []string{"prod", "smoke"}},
	})
	l := screens.NewList(st)
	// load
	m, _ := l.Update(screens.LoadedMsg{Sessions: []store.Session{
		{ID: "01F8MECHZX3TBDSZ7XR9PFE7M0", Name: "alpha", Tags: []string{"prod"}},
		{ID: "01F8MECHZX3TBDSZ7XR9PFE7M1", Name: "beta", Tags: []string{"staging"}},
		{ID: "01F8MECHZX3TBDSZ7XR9PFE7M2", Name: "gamma", Tags: []string{"prod", "smoke"}},
	}})
	l = m.(screens.List)
	// /  prod  Enter
	l = sendKeysToList(t, l, []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("/")},
		{Type: tea.KeyRunes, Runes: []rune("p")},
		{Type: tea.KeyRunes, Runes: []rune("r")},
		{Type: tea.KeyRunes, Runes: []rune("o")},
		{Type: tea.KeyRunes, Runes: []rune("d")},
		{Type: tea.KeyEnter},
	})
	v := l.View()
	if !strings.Contains(v, "alpha") || !strings.Contains(v, "gamma") || strings.Contains(v, "beta") {
		t.Errorf("expected only alpha+gamma, got:\n%s", v)
	}
	if !strings.Contains(v, "filter: prod") {
		t.Errorf("expected chip in view:\n%s", v)
	}
}
