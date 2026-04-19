package screens

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/cli"
	"github.com/torosent/crankfire/internal/store"
)

type setsDetailLoadedMsg struct {
	set  store.Set
	runs []store.SetRun
	err  error
}

type SetsDetail struct {
	ctx   context.Context
	store store.Store
	id    string
	set   store.Set
	runs  []store.SetRun
	err   error
}

func NewSetsDetail(ctx context.Context, st store.Store, id string) tea.Model {
	return &SetsDetail{ctx: ctx, store: st, id: id}
}

func (m *SetsDetail) Init() tea.Cmd { return m.load() }

func (m *SetsDetail) load() tea.Cmd {
	return func() tea.Msg {
		s, err := m.store.GetSet(m.ctx, m.id)
		if err != nil {
			return setsDetailLoadedMsg{err: err}
		}
		runs, _ := m.store.ListSetRuns(m.ctx, m.id)
		return setsDetailLoadedMsg{set: s, runs: runs}
	}
}

func (m *SetsDetail) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case setsDetailLoadedMsg:
		m.set, m.runs, m.err = msg.set, msg.runs, msg.err
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return NewSetsList(m.ctx, m.store), nil
		case "e":
			return NewSetsEdit(m.ctx, m.store, m.set), nil
		case "R":
			return NewSetRun(m.ctx, m.store, cli.NewSetBuilder(), m.id), nil
		case "h":
			return NewSetsHistory(m.ctx, m.store, m.id), nil
		}
	}
	return m, nil
}

func (m *SetsDetail) View() string {
	var b strings.Builder
	b.WriteString("Set Detail — e)dit  R)un  h)istory  esc)back\n\n")
	if m.err != nil {
		fmt.Fprintf(&b, "Error: %v\n", m.err)
		return b.String()
	}
	fmt.Fprintf(&b, "Name: %s\nID: %s\nDescription: %s\n\nStages:\n", m.set.Name, m.set.ID, m.set.Description)
	for i, st := range m.set.Stages {
		fmt.Fprintf(&b, "  %d. %s (on_failure=%s, %d items)\n", i+1, st.Name, defaultStr(string(st.OnFailure), "abort"), len(st.Items))
		for _, it := range st.Items {
			fmt.Fprintf(&b, "       - %s -> session %s\n", it.Name, it.SessionID)
		}
	}
	if len(m.set.Thresholds) > 0 {
		b.WriteString("\nThresholds:\n")
		for _, t := range m.set.Thresholds {
			fmt.Fprintf(&b, "  - %s %s %v [%s]\n", t.Metric, t.Op, t.Value, defaultStr(t.Scope, "aggregate"))
		}
	}
	if len(m.runs) > 0 {
		b.WriteString("\nRecent runs:\n")
		for i, r := range m.runs {
			if i >= 5 {
				break
			}
			fmt.Fprintf(&b, "  %s — %s\n", r.StartedAt.Format("2006-01-02 15:04"), r.Status)
		}
	}
	return b.String()
}

func defaultStr(v, d string) string {
	if v == "" {
		return d
	}
	return v
}


