package screens

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/torosent/crankfire/internal/setrunner"
	"github.com/torosent/crankfire/internal/store"
)

type SetCompare struct {
	ctx     context.Context
	store   store.Store
	run     store.SetRun
	compare setrunner.Comparison
}

func NewSetCompare(ctx context.Context, st store.Store, run store.SetRun) tea.Model {
	var items []store.ItemResult
	for _, s := range run.Stages {
		items = append(items, s.Items...)
	}
	return &SetCompare{ctx: ctx, store: st, run: run, compare: setrunner.Compare(items)}
}

func (m *SetCompare) Init() tea.Cmd { return nil }

func (m *SetCompare) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc", "q":
			return NewSetsDetail(m.ctx, m.store, m.run.SetID), nil
		}
	}
	return m, nil
}

func (m *SetCompare) View() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Compare — %s [%s]   esc)back\n\n", m.run.SetName, m.run.Status)

	colW := 12
	fmt.Fprintf(&b, "%-12s", "metric")
	for _, name := range m.compare.Items {
		fmt.Fprintf(&b, " %*s", colW, name)
	}
	b.WriteString("\n")
	b.WriteString(strings.Repeat("-", 12+(colW+1)*len(m.compare.Items)) + "\n")
	for _, row := range m.compare.Rows {
		fmt.Fprintf(&b, "%-12s", row.Metric)
		for _, name := range m.compare.Items {
			v := row.Values[name]
			if name == row.Winner {
				fmt.Fprintf(&b, " %*s", colW, fmt.Sprintf("★%.2f", v))
			} else {
				fmt.Fprintf(&b, " %*.2f", colW, v)
			}
		}
		b.WriteString("\n")
	}
	if len(m.run.Thresholds) > 0 {
		b.WriteString("\nThresholds:\n")
		for _, th := range m.run.Thresholds {
			mark := "✓"
			if !th.Passed {
				mark = "✗"
			}
			fmt.Fprintf(&b, "  %s %s %s %v actual=%.3f [%s]\n", mark, th.Metric, th.Op, th.Value, th.Actual, th.Scope)
		}
	}
	return b.String()
}
