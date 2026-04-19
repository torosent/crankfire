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
	marks  map[int]bool
	err    error
}

func NewSetsHistory(ctx context.Context, st store.Store, setID string) tea.Model {
	return &SetsHistory{ctx: ctx, store: st, setID: setID, marks: map[int]bool{}}
}

func (m *SetsHistory) Init() tea.Cmd {
	return func() tea.Msg {
		runs, err := m.store.ListSetRuns(m.ctx, m.setID)
		return historyLoadedMsg{runs: runs, err: err}
	}
}

func (m *SetsHistory) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case historyLoadedMsg:
		m.runs, m.err = v.runs, v.err
	case tea.KeyMsg:
		switch v.String() {
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
		case " ":
			if m.marks == nil {
				m.marks = map[int]bool{}
			}
			if m.marks[m.cursor] {
				delete(m.marks, m.cursor)
			} else {
				m.marks[m.cursor] = true
			}
		case "enter":
			if len(m.runs) > 0 {
				return NewSetCompare(m.ctx, m.store, m.runs[m.cursor]), nil
			}
		case "d":
			if len(m.marks) == 2 {
				var picked []store.SetRun
				for i := range m.runs {
					if m.marks[i] {
						picked = append(picked, m.runs[i])
					}
				}
				return NewSetsDiff(m.ctx, m.store, m.setID, picked[0], picked[1]), nil
			}
		}
	}
	return m, nil
}

func (m *SetsHistory) View() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Run History — enter)compare  space)mark  d)diff (need 2)  esc)back   [%d selected]\n\n", len(m.marks))
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
		mark := "  "
		if m.marks[i] {
			mark = "* "
		}
		fmt.Fprintf(&b, "%s%s%s  %-10s  %d stage(s)\n", marker, mark, r.StartedAt.Format("2006-01-02 15:04:05"), r.Status, len(r.Stages))
	}
	return b.String()
}
