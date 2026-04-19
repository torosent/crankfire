package screens

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/store"
)

type historyLoadedMsg struct {
	runs []store.SetRun
	err  error
}

type SetsHistory struct {
	ctx    context.Context
	store  store.Store
	setID  string
	runs   []store.SetRun
	cursor int
	err    error
}

func NewSetsHistory(ctx context.Context, st store.Store, setID string) tea.Model {
	return &SetsHistory{ctx: ctx, store: st, setID: setID}
}

func (m *SetsHistory) Init() tea.Cmd {
	return func() tea.Msg {
		runs, err := m.store.ListSetRuns(m.ctx, m.setID)
		return historyLoadedMsg{runs: runs, err: err}
	}
}

func (m *SetsHistory) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case historyLoadedMsg:
		m.runs, m.err = msg.runs, msg.err
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return NewSetsDetail(m.ctx, m.store, m.setID), nil
		case "j", "down":
			if m.cursor < len(m.runs)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if len(m.runs) > 0 {
				return NewSetCompare(m.ctx, m.store, m.runs[m.cursor]), nil
			}
		}
	}
	return m, nil
}

func (m *SetsHistory) View() string {
	var b strings.Builder
	b.WriteString("Run History — enter)compare  esc)back\n\n")
	if m.err != nil {
		fmt.Fprintf(&b, "Error: %v\n", m.err)
		return b.String()
	}
	if len(m.runs) == 0 {
		b.WriteString("No runs yet.\n")
		return b.String()
	}
	for i, r := range m.runs {
		marker := "  "
		if i == m.cursor {
			marker = "> "
		}
		fmt.Fprintf(&b, "%s%s  %-10s  %d stage(s)\n", marker, r.StartedAt.Format("2006-01-02 15:04:05"), r.Status, len(r.Stages))
	}
	return b.String()
}
