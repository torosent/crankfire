package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tui/screens"
)

type Root struct {
	store store.Store
	stack []tea.Model
}

func NewRoot(s store.Store) Root {
	return Root{store: s, stack: []tea.Model{screens.NewList(s)}}
}

func (r Root) Init() tea.Cmd { return r.stack[len(r.stack)-1].Init() }

func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		if k.Type == tea.KeyRunes && len(k.Runes) == 1 && k.Runes[0] == 'q' && len(r.stack) == 1 {
			return r, tea.Quit
		}
	}

	switch m := msg.(type) {
	case screens.PushMsg:
		r.stack = append(r.stack, m.Model)
		return r, r.stack[len(r.stack)-1].Init()
	case screens.PopMsg:
		if len(r.stack) > 1 {
			r.stack = r.stack[:len(r.stack)-1]
		}
		return r, r.stack[len(r.stack)-1].Init()
	}

	top := r.stack[len(r.stack)-1]
	next, cmd := top.Update(msg)
	r.stack[len(r.stack)-1] = next
	return r, cmd
}

func (r Root) View() string { return r.stack[len(r.stack)-1].View() }
