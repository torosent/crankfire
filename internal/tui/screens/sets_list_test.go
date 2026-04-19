package screens_test

import (
	"context"
	"strings"
	"testing"

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
