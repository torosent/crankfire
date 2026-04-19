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
	bar    FilterBar
	// sessionTagsCache caches sessionID → tags so the transitive
	// filter doesn't re-hit the store on every render. Invalidated
	// only when the bar changes.
	sessionTagsCache map[string][]string
}

func NewSetsList(ctx context.Context, st store.Store) *SetsList {
	return &SetsList{ctx: ctx, store: st, bar: NewFilterBar()}
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

// filtered returns the sets slice after applying the active filter.
// A set matches when any of its items references a session whose tags
// satisfy the matcher. Sessions are resolved lazily and cached.
func (m *SetsList) filtered() []store.Set {
	if !m.bar.IsActive() {
		return m.sets
	}
	if m.sessionTagsCache == nil {
		m.sessionTagsCache = map[string][]string{}
	}
	out := make([]store.Set, 0, len(m.sets))
	for _, set := range m.sets {
		if m.setMatches(set) {
			out = append(out, set)
		}
	}
	return out
}

func (m *SetsList) setMatches(set store.Set) bool {
	for _, stage := range set.Stages {
		for _, item := range stage.Items {
			tags, ok := m.sessionTagsCache[item.SessionID]
			if !ok {
				sess, err := m.store.GetSession(m.ctx, item.SessionID)
				if err == nil {
					tags = sess.Tags
				}
				m.sessionTagsCache[item.SessionID] = tags
			}
			if m.bar.Matcher().Matches(tags) {
				return true
			}
		}
	}
	return false
}

func (m *SetsList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case setsLoadedMsg:
		m.sets, m.err = msg.sets, msg.err
		m.sessionTagsCache = nil // re-build on next filter
		if m.cursor >= len(m.sets) {
			m.cursor = 0
		}
	case tea.KeyMsg:
		var cmd tea.Cmd
		var handled bool
		m.bar, handled, cmd = m.bar.Update(msg)
		if handled {
			// Filter changed → invalidate cache + clamp cursor
			m.sessionTagsCache = nil
			if n := len(m.filtered()); m.cursor >= n {
				if n == 0 {
					m.cursor = 0
				} else {
					m.cursor = n - 1
				}
			}
			return m, cmd
		}
		switch msg.String() {
		case "q", "esc":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.filtered())-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "t":
			return NewTemplatePicker(m.ctx, m.store), nil
		case "n":
			return m, func() tea.Msg {
				return PushMsg{NewSetsEdit(m.ctx, m.store, store.Set{})}
			}
		case "enter":
			rows := m.filtered()
			if len(rows) > 0 {
				id := rows[m.cursor].ID
				return m, func() tea.Msg {
					return PushMsg{NewSetsDetail(m.ctx, m.store, id)}
				}
			}
		case "d":
			rows := m.filtered()
			if len(rows) > 0 {
				_ = m.store.DeleteSet(m.ctx, rows[m.cursor].ID)
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
	b.WriteString("Sets — n)ew  t)emplate  enter)open  d)elete  r)efresh  /)filter  q)uit\n")
	if bv := m.bar.View(); bv != "" {
		b.WriteString(bv + "\n")
	}
	b.WriteString("\n")
	if m.err != nil {
		fmt.Fprintf(&b, "Error: %v\n", m.err)
		return b.String()
	}
	rows := m.filtered()
	if len(rows) == 0 {
		if m.bar.IsActive() {
			b.WriteString("(no sets match filter)\n")
		} else {
			b.WriteString("No sets yet. Press n to create one.\n")
		}
		return b.String()
	}
	for i, s := range rows {
		marker := "  "
		if i == m.cursor {
			marker = "> "
		}
		fmt.Fprintf(&b, "%s%-26s  %-30s  %d stage(s)\n", marker, s.ID, s.Name, len(s.Stages))
	}
	return b.String()
}
