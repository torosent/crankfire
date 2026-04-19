package screens

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/torosent/crankfire/internal/store"
)

type setsLoadedMsg struct {
	sets []store.Set
	err  error
}

type SetsList struct {
	ctx    context.Context
	store  store.Store
	sets   []store.Set
	cursor int
	err    error
	width  int
	height int
}

func NewSetsList(ctx context.Context, st store.Store) *SetsList {
	return &SetsList{ctx: ctx, store: st}
}

func (m *SetsList) Init() tea.Cmd {
	return m.load()
}

func (m *SetsList) load() tea.Cmd {
	return func() tea.Msg {
		s, err := m.store.ListSets(m.ctx)
		return setsLoadedMsg{sets: s, err: err}
	}
}

func (m *SetsList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case setsLoadedMsg:
		m.sets, m.err = msg.sets, msg.err
		if m.cursor >= len(m.sets) {
			m.cursor = 0
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.sets)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "n":
			return m, func() tea.Msg {
				return PushMsg{NewSetsEdit(m.ctx, m.store, store.Set{})}
			}
		case "enter":
			if len(m.sets) > 0 {
				return m, func() tea.Msg {
					return PushMsg{NewSetsDetail(m.ctx, m.store, m.sets[m.cursor].ID)}
				}
			}
		case "d":
			if len(m.sets) > 0 {
				_ = m.store.DeleteSet(m.ctx, m.sets[m.cursor].ID)
				return m, m.load()
			}
		case "r":
			return m, m.load()
		}
	}
	return m, nil
}

func (m *SetsList) View() string {
	var b strings.Builder
	b.WriteString("Sets — n)ew  enter)open  d)elete  r)efresh  q)uit\n\n")
	if m.err != nil {
		fmt.Fprintf(&b, "Error: %v\n", m.err)
		return b.String()
	}
	if len(m.sets) == 0 {
		b.WriteString("No sets yet. Press n to create one.\n")
		return b.String()
	}
	for i, s := range m.sets {
		marker := "  "
		if i == m.cursor {
			marker = "> "
		}
		fmt.Fprintf(&b, "%s%-26s  %-30s  %d stage(s)\n", marker, s.ID, s.Name, len(s.Stages))
	}
	return b.String()
}
