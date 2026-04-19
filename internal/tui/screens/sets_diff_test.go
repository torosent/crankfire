package screens_test

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tui/screens"
)

func TestSetsHistorySpaceTogglesMark(t *testing.T) {
	st, _ := store.NewFS(t.TempDir())
	ctx := context.Background()
	setID := "01F8MECHZX3TBDSZ7XR9PFE7H0"
	_ = st.SaveSet(ctx, store.Set{ID: setID, Name: "n", Stages: []store.Stage{{Name: "s"}}})
	for i := 0; i < 3; i++ {
		r, _ := st.CreateSetRun(ctx, setID)
		_ = st.FinalizeSetRun(ctx, r)
	}
	model := screens.NewSetsHistory(ctx, st, setID)
	cmd := model.Init()
	model, _ = model.Update(cmd())
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if !strings.Contains(model.View(), "[1 selected]") {
		t.Errorf("expected [1 selected]:\n%s", model.View())
	}
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if !strings.Contains(model.View(), "[2 selected]") {
		t.Errorf("expected [2 selected]:\n%s", model.View())
	}
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if !strings.Contains(model.View(), "[1 selected]") {
		t.Errorf("expected [1 selected] after untoggle:\n%s", model.View())
	}
}

func TestSetsHistoryDOpensDiffWithTwo(t *testing.T) {
	st, _ := store.NewFS(t.TempDir())
	ctx := context.Background()
	setID := "01F8MECHZX3TBDSZ7XR9PFE7H1"
	_ = st.SaveSet(ctx, store.Set{ID: setID, Name: "n", Stages: []store.Stage{{Name: "s"}}})
	for i := 0; i < 2; i++ {
		r, _ := st.CreateSetRun(ctx, setID)
		_ = st.FinalizeSetRun(ctx, r)
	}
	model := screens.NewSetsHistory(ctx, st, setID)
	cmd := model.Init()
	model, _ = model.Update(cmd())
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if _, ok := model.(*screens.SetsDiff); !ok {
		t.Errorf("d with 2 selected should transition to SetsDiff, got %T", model)
	}
}

func TestSetsDiffViewShowsVerdict(t *testing.T) {
	a := store.SetRun{Status: store.SetRunCompleted, Stages: []store.StageResult{
		{Name: "s", Items: []store.ItemResult{{Name: "api", Status: store.RunStatusCompleted, Summary: store.RunSummary{P95Ms: 100, TotalRequests: 1000, DurationSec: 10}}}},
	}}
	b := store.SetRun{Status: store.SetRunCompleted, Stages: []store.StageResult{
		{Name: "s", Items: []store.ItemResult{{Name: "api", Status: store.RunStatusCompleted, Summary: store.RunSummary{P95Ms: 110, TotalRequests: 1000, DurationSec: 10}}}},
	}}
	d := screens.NewSetsDiff(context.Background(), nil, "set1", a, b)
	v := d.View()
	if !strings.Contains(v, "regressed") {
		t.Errorf("view should show verdict regressed:\n%s", v)
	}
	if !strings.Contains(v, "api") {
		t.Errorf("view should show item:\n%s", v)
	}
}
