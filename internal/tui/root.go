package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/store"
)

type Root struct {
	store store.Store
	stack []tea.Model
}

func NewRoot(s store.Store) Root {
	return Root{store: s, stack: []tea.Model{placeholder{title: "Sessions"}}}
}

func (r Root) Init() tea.Cmd { return r.stack[len(r.stack)-1].Init() }

func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		if k.Type == tea.KeyRunes && len(k.Runes) == 1 && k.Runes[0] == 'q' && len(r.stack) == 1 {
			return r, tea.Quit
		}
	}
	top := r.stack[len(r.stack)-1]
	next, cmd := top.Update(msg)
	r.stack[len(r.stack)-1] = next
	return r, cmd
}

func (r Root) View() string { return r.stack[len(r.stack)-1].View() }

type placeholder struct{ title string }

func (p placeholder) Init() tea.Cmd { return nil }
func (p placeholder) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return p, nil }
func (p placeholder) View() string { return p.title + "\n\n[q] quit" }
