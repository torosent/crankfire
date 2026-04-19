package screens

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/store"
)

type LoadedMsg struct {
	Sessions []store.Session
	Err      error
}

type List struct {
	store    store.Store
	sessions []store.Session
	cursor   int
	err      error
}

func NewList(s store.Store) List {
	return List{store: s}
}

func (l List) Init() tea.Cmd {
	return func() tea.Msg {
		ss, err := l.store.ListSessions(context.Background())
		return LoadedMsg{Sessions: ss, Err: err}
	}
}

func (l List) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case LoadedMsg:
		l.sessions = m.Sessions
		l.err = m.Err
	case tea.KeyMsg:
		switch m.String() {
		case "q", "ctrl+c":
			return l, tea.Quit
		case "up", "k":
			if l.cursor > 0 {
				l.cursor--
			}
		case "down", "j":
			if l.cursor < len(l.sessions)-1 {
				l.cursor++
			}
		case "enter":
			if len(l.sessions) > 0 {
				return l, func() tea.Msg {
					return PushMsg{NewDetail(l.store, l.sessions[l.cursor])}
				}
			}
		case "n":
			return l, func() tea.Msg {
				return PushMsg{NewEdit(l.store, store.Session{})}
			}
		case "e":
			if len(l.sessions) > 0 {
				sel := l.sessions[l.cursor]
				return l, func() tea.Msg {
					return PushMsg{NewEdit(l.store, sel)}
				}
			}
		case "i":
			return l, func() tea.Msg {
				return PushMsg{NewImport(l.store)}
			}
		case "r":
			if len(l.sessions) > 0 {
				sel := l.sessions[l.cursor]
				return l, func() tea.Msg {
					return PushMsg{NewRun(l.store, sel)}
				}
			}
		case "d":
			if len(l.sessions) > 0 {
				id := l.sessions[l.cursor].ID
				onYes := func() {
					ctx := context.Background()
					_ = l.store.DeleteSession(ctx, id)
				}
				confirm := NewConfirm("Delete session?", onYes)
				return l, func() tea.Msg {
					return PushMsg{confirm}
				}
			}
		}
	}
	return l, nil
}

func (l List) View() string {
	var b strings.Builder
	b.WriteString("Sessions\n\n")
	if l.err != nil {
		fmt.Fprintf(&b, "error: %v\n", l.err)
	}
	if len(l.sessions) == 0 {
		b.WriteString("(no sessions yet — press n to create or i to import)\n")
	}
	for i, s := range l.sessions {
		prefix := "  "
		if i == l.cursor {
			prefix = "> "
		}
		fmt.Fprintf(&b, "%s%s\n", prefix, s.Name)
	}
	b.WriteString("\n[n] new  [e] edit  [d] delete  [i] import  [r] run  [h] history  [Enter] details  [q] quit\n")
	return b.String()
}
