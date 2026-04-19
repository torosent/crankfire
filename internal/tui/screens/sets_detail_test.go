package screens_test

import (
	"context"
	"strings"
	"testing"

	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tui/screens"
)

func TestSetsDetailShowsStages(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	sess := store.Session{Name: "base"}
	_ = st.SaveSession(context.Background(), sess)
	sList, _ := st.ListSessions(context.Background())
	_ = st.SaveSet(context.Background(), store.Set{
		Name: "smoke",
		Stages: []store.Stage{
			{Name: "warmup", Items: []store.SetItem{{Name: "warm", SessionID: sList[0].ID}}},
			{Name: "load", Items: []store.SetItem{{Name: "login", SessionID: sList[0].ID}}},
		},
	})
	list, _ := st.ListSets(context.Background())

	m := screens.NewSetsDetail(context.Background(), st, list[0].ID)
	
	// Simulate the async load by calling the Init function directly
	initCmd := m.Init()
	next, _ := m.Update(initCmd())
	m = next.(*screens.SetsDetail)
	
	view := m.View()
	for _, want := range []string{"smoke", "warmup", "load", "R)un", "e)dit", "h)istory"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q\n%s", want, view)
		}
	}
}
