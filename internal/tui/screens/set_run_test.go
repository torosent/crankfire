package screens_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/setrunner"
	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tui/screens"
)

// fakeBuilder returns an immediate error so the runner goroutine terminates
// quickly without requiring real protocol wiring. Tests use IngestForTest to
// drive rendering independently of the goroutine.
type fakeBuilder struct{}

func (fakeBuilder) Build(_ context.Context, _ config.Config, _ string) (setrunner.ItemRun, error) {
	return setrunner.ItemRun{}, fmt.Errorf("fake: build not available in tests")
}

func TestSetRunRendersStageAndItem(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)

	sess := store.Session{Name: "base"}
	_ = st.SaveSession(context.Background(), sess)
	sList, _ := st.ListSessions(context.Background())
	_ = st.SaveSet(context.Background(), store.Set{
		Name: "smoke",
		Stages: []store.Stage{{
			Name:  "s1",
			Items: []store.SetItem{{Name: "i", SessionID: sList[0].ID}},
		}},
	})
	list, _ := st.ListSets(context.Background())

	m := screens.NewSetRun(context.Background(), st, fakeBuilder{}, list[0].ID)
	m.Init()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	sr := m.(*screens.SetRun)
	t.Cleanup(sr.WaitForTest)
	sr.IngestForTest(setrunner.Event{Kind: setrunner.EventSetStarted})
	sr.IngestForTest(setrunner.Event{Kind: setrunner.EventStageStarted, Stage: "s1"})
	sr.IngestForTest(setrunner.Event{Kind: setrunner.EventItemStarted, Stage: "s1", Item: "i"})

	view := m.View()
	for _, want := range []string{"smoke", "s1", "i"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q\n%s", want, view)
		}
	}
}

func TestSetRunQCancelsAndNavigatesBack(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	_ = st.SaveSession(context.Background(), store.Session{Name: "base"})
	sList, _ := st.ListSessions(context.Background())
	_ = st.SaveSet(context.Background(), store.Set{
		Name: "myset",
		Stages: []store.Stage{{
			Name:  "s1",
			Items: []store.SetItem{{Name: "x", SessionID: sList[0].ID}},
		}},
	})
	list, _ := st.ListSets(context.Background())
	if len(list) == 0 {
		t.Fatal("expected at least one set")
	}

	m := screens.NewSetRun(context.Background(), st, fakeBuilder{}, list[0].ID)
	m.Init()
	t.Cleanup(func() { m.(*screens.SetRun).WaitForTest() })
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	// After q we should be on a different screen (SetsDetail).
	if _, ok := next.(*screens.SetRun); ok {
		t.Error("expected navigation away from SetRun after q, still on SetRun")
	}
}
