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
	bar      FilterBar
}

func NewList(s store.Store) List {
	return List{store: s, bar: NewFilterBar()}
}

// filtered returns the sessions slice after applying the active filter.
func (l List) filtered() []store.Session {
	if !l.bar.IsActive() {
		return l.sessions
	}
	out := make([]store.Session, 0, len(l.sessions))
	for _, s := range l.sessions {
		if l.bar.Matcher().Matches(s.Tags) {
			out = append(out, s)
		}
	}
	return out
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
		// Filter bar gets first crack at every key.
		var cmd tea.Cmd
		var handled bool
		l.bar, handled, cmd = l.bar.Update(m)
		if handled {
			// Cursor may now point past the end of filtered slice.
			if n := len(l.filtered()); l.cursor >= n && n > 0 {
				l.cursor = n - 1
			} else if n == 0 {
				l.cursor = 0
			}
			return l, cmd
		}
		switch m.String() {
		case "q", "ctrl+c":
			return l, tea.Quit
		case "up", "k":
			if l.cursor > 0 {
				l.cursor--
			}
		case "down", "j":
			if l.cursor < len(l.filtered())-1 {
				l.cursor++
			}
		case "enter":
			rows := l.filtered()
			if len(rows) > 0 {
				sel := rows[l.cursor]
				return l, func() tea.Msg {
					return PushMsg{NewDetail(l.store, sel)}
				}
			}
		case "n":
			return l, func() tea.Msg {
				return PushMsg{NewEdit(l.store, store.Session{})}
			}
		case "e":
			rows := l.filtered()
			if len(rows) > 0 {
				sel := rows[l.cursor]
				return l, func() tea.Msg {
					return PushMsg{NewEdit(l.store, sel)}
				}
			}
		case "i":
			return l, func() tea.Msg {
				return PushMsg{NewImport(l.store)}
			}
		case "r":
			rows := l.filtered()
			if len(rows) > 0 {
				sel := rows[l.cursor]
				return l, func() tea.Msg {
					return PushMsg{NewRun(l.store, sel)}
				}
			}
		case "h":
			rows := l.filtered()
			if len(rows) > 0 {
				id := rows[l.cursor].ID
				return l, func() tea.Msg {
					return PushMsg{NewHistory(l.store, id)}
				}
			}
		case "d":
			rows := l.filtered()
			if len(rows) > 0 {
				id := rows[l.cursor].ID
				onYes := func() {
					ctx := context.Background()
					_ = l.store.DeleteSession(ctx, id)
				}
				confirm := NewConfirm("Delete session?", onYes)
				return l, func() tea.Msg {
					return PushMsg{confirm}
				}
			}
		case "s":
			return l, func() tea.Msg {
				return PushMsg{NewSetsList(context.Background(), l.store)}
			}
		}
	}
	return l, nil
}

func (l List) View() string {
	var b strings.Builder
	b.WriteString("Sessions\n\n")
	if barView := l.bar.View(); barView != "" {
		b.WriteString(barView + "\n")
	}
	if l.err != nil {
		fmt.Fprintf(&b, "error: %v\n", l.err)
	}
	rows := l.filtered()
	if len(rows) == 0 {
		b.WriteString("(no sessions yet — press n to create or i to import)\n")
	}
	for i, s := range rows {
		prefix := "  "
		if i == l.cursor {
			prefix = "> "
		}
		fmt.Fprintf(&b, "%s%s\n", prefix, s.Name)
	}
	b.WriteString("\n[n] new  [e] edit  [d] delete  [i] import  [r] run  [h] history  [s] sets  [Enter] details  [/] filter  [q] quit\n")
	return b.String()
}
