package screens

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/torosent/crankfire/internal/setrunner"
	"github.com/torosent/crankfire/internal/store"
)

type SetsDiff struct {
	ctx    context.Context
	store  store.Store
	setID  string
	result setrunner.DiffResult
}

func NewSetsDiff(ctx context.Context, st store.Store, setID string, a, b store.SetRun) *SetsDiff {
	return &SetsDiff{ctx: ctx, store: st, setID: setID, result: setrunner.Diff(a, b)}
}

func (m *SetsDiff) Init() tea.Cmd { return nil }

func (m *SetsDiff) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc", "q":
			return NewSetsHistory(m.ctx, m.store, m.setID), nil
		}
	}
	return m, nil
}

func (m *SetsDiff) View() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Diff — %s vs %s   verdict: %s   esc)back\n\n",
		m.result.A.StartedAt.Format("2006-01-02 15:04:05"),
		m.result.B.StartedAt.Format("2006-01-02 15:04:05"),
		m.result.OverallVerdict)
	fmt.Fprintf(&b, "%-20s  %-10s  %10s  %10s  %10s  %10s  %10s\n",
		"item", "stage", "dP50ms", "dP95ms", "dP99ms", "dErrRate", "dRPS")
	for _, row := range m.result.Rows {
		extra := ""
		if !row.APresent {
			extra = " (B-only)"
		} else if !row.BPresent {
			extra = " (A-only)"
		}
		fmt.Fprintf(&b, "%-20s  %-10s  %+10.2f  %+10.2f  %+10.2f  %+10.4f  %+10.2f%s\n",
			row.ItemName, row.Stage,
			row.P50DeltaMs, row.P95DeltaMs, row.P99DeltaMs,
			row.ErrRateDelta, row.RPSDelta, extra)
	}
	return b.String()
}
