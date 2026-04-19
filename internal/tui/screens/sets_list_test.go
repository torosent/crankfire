package screens_test

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tui/screens"
)

func TestSetsListEmptyShowsHelp(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	m := screens.NewSetsList(context.Background(), st)
	m.Init()
	view := m.View()
	for _, want := range []string{"No sets", "n)ew", "q)uit"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q\n%s", want, view)
		}
	}
}

func TestSetsListRendersSavedSet(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	sess := store.Session{Name: "base"}
	_ = st.SaveSession(context.Background(), sess)
	list, _ := st.ListSessions(context.Background())
	_ = st.SaveSet(context.Background(), store.Set{
		Name: "smoke",
		Stages: []store.Stage{{Name: "s", Items: []store.SetItem{{Name: "i", SessionID: list[0].ID}}}},
	})
	m := screens.NewSetsList(context.Background(), st)
	
	// Simulate the async load by calling the Init function directly
	initCmd := m.Init()
	next, _ := m.Update(initCmd())
	m = next.(*screens.SetsList)
	
	if !strings.Contains(m.View(), "smoke") {
		t.Errorf("view missing 'smoke':\n%s", m.View())
	}
}

func TestSetsListTransitiveTagFilter(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	ctx := context.Background()
	// 2 sessions, one tagged prod
	prodSess := store.Session{ID: "01F8MECHZX3TBDSZ7XR9PFE7M0", Name: "prod-sess", Tags: []string{"prod"}}
	devSess := store.Session{ID: "01F8MECHZX3TBDSZ7XR9PFE7M1", Name: "dev-sess", Tags: []string{"dev"}}
	_ = st.SaveSession(ctx, prodSess)
	_ = st.SaveSession(ctx, devSess)
	// 2 sets: A references prod, B references dev
	_ = st.SaveSet(ctx, store.Set{ID: "01F8MECHZX3TBDSZ7XR9PFE7S0", Name: "set-A", Stages: []store.Stage{{Name: "s", Items: []store.SetItem{{Name: "i", SessionID: prodSess.ID}}}}})
	_ = st.SaveSet(ctx, store.Set{ID: "01F8MECHZX3TBDSZ7XR9PFE7S1", Name: "set-B", Stages: []store.Stage{{Name: "s", Items: []store.SetItem{{Name: "i", SessionID: devSess.ID}}}}})

	m := screens.NewSetsList(ctx, st)
	// drive Init then load
	cmd := m.Init()
	msg := cmd()
	model, _ := m.Update(msg)
	m = model.(*screens.SetsList)
	// /  prod  Enter
	for _, k := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("/")},
		{Type: tea.KeyRunes, Runes: []rune("p")},
		{Type: tea.KeyRunes, Runes: []rune("r")},
		{Type: tea.KeyRunes, Runes: []rune("o")},
		{Type: tea.KeyRunes, Runes: []rune("d")},
		{Type: tea.KeyEnter},
	} {
		model, _ = m.Update(k)
		m = model.(*screens.SetsList)
	}
	v := m.View()
	if !strings.Contains(v, "set-A") || strings.Contains(v, "set-B") {
		t.Errorf("expected only set-A, got:\n%s", v)
	}
}
