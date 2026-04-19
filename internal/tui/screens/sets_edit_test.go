package screens_test

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tui/screens"
)

func TestSetsEditYAMLModeToggle(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	m := screens.NewSetsEdit(context.Background(), st, store.Set{Name: "x"})
	m.Init()
	if !strings.Contains(m.View(), "Form") {
		t.Errorf("expected Form mode initially:\n%s", m.View())
	}
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	view := model.View()
	if !strings.Contains(view, "YAML") {
		t.Errorf("expected YAML mode after y:\n%s", view)
	}
}

func TestSetsEditSaveValidatesAndPersists(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	sess := store.Session{Name: "base"}
	_ = st.SaveSession(context.Background(), sess)
	list, _ := st.ListSessions(context.Background())
	in := store.Set{
		Name: "smoke",
		Stages: []store.Stage{{Name: "s", Items: []store.SetItem{{Name: "i", SessionID: list[0].ID}}}},
	}
	m := screens.NewSetsEdit(context.Background(), st, in)
	m.Init()
	// Use the test-only API to trigger save.
	if _, cmd := m.(*screens.SetsEdit).Save(); cmd != nil {
		_ = cmd()
	}
	out, _ := st.ListSets(context.Background())
	if len(out) != 1 || out[0].Name != "smoke" {
		t.Errorf("expected smoke saved, got %+v", out)
	}
}
