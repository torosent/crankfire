package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/torosent/crankfire/internal/tagfilter"
)

// FilterBar is a slash-search filter widget reused by list screens.
//
// State machine:
//
//	inactive --[/]--> prompting --[Enter+valid]--> active
//	                      |--[Esc]--> inactive (or active if there was a prior expr)
//	active --[x]--> inactive
//	active --[/]--> prompting (pre-filled with current expr)
//
// The parent screen forwards every tea.KeyMsg to Update and respects the
// returned `handled` bool: when true, the parent must skip its own key handling
// for that message.
type FilterBar struct {
	state   filterBarState
	input   textinput.Model
	expr    string
	matcher tagfilter.Matcher
	err     string
}

type filterBarState int

const (
	filterInactive  filterBarState = iota
	filterPrompting filterBarState = iota
	filterActive    filterBarState = iota
)

// NewFilterBar returns a fresh inactive bar.
func NewFilterBar() FilterBar {
	ti := textinput.New()
	ti.Placeholder = "tag filter (e.g. prod smoke,regression)"
	ti.Prompt = "/"
	ti.CharLimit = 256
	return FilterBar{input: ti}
}

// IsActive reports whether the bar currently has a filter applied.
func (b FilterBar) IsActive() bool { return b.state == filterActive }

// IsPrompting reports whether the bar is showing the input prompt.
func (b FilterBar) IsPrompting() bool { return b.state == filterPrompting }

// Expr returns the current filter expression ("" if inactive).
func (b FilterBar) Expr() string { return b.expr }

// Matcher returns the compiled matcher (zero matcher if inactive).
func (b FilterBar) Matcher() tagfilter.Matcher { return b.matcher }

// SetExpr applies a filter programmatically.
func (b *FilterBar) SetExpr(expr string) error {
	m, err := tagfilter.Parse(expr)
	if err != nil {
		return err
	}
	b.expr = expr
	b.matcher = m
	if expr == "" {
		b.state = filterInactive
	} else {
		b.state = filterActive
	}
	return nil
}

// Update advances the bar's state machine. Returns the updated bar, a bool
// indicating whether the message was consumed (parent should ignore it),
// and an optional cmd for the bubble tea runtime.
func (b FilterBar) Update(msg tea.Msg) (FilterBar, bool, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return b, false, nil
	}
	switch b.state {
	case filterInactive:
		if km.String() == "/" {
			b.input.SetValue("")
			b.input.Focus()
			b.state = filterPrompting
			b.err = ""
			return b, true, textinput.Blink
		}
		return b, false, nil
	case filterPrompting:
		switch km.Type {
		case tea.KeyEsc:
			b.input.Blur()
			b.input.SetValue("")
			b.err = ""
			if b.expr != "" {
				b.state = filterActive
			} else {
				b.state = filterInactive
			}
			return b, true, nil
		case tea.KeyEnter:
			expr := strings.TrimSpace(b.input.Value())
			if expr == "" {
				b.input.Blur()
				b.expr = ""
				b.matcher = tagfilter.Matcher{}
				b.state = filterInactive
				b.err = ""
				return b, true, nil
			}
			m, err := tagfilter.Parse(expr)
			if err != nil {
				b.err = fmt.Sprintf("invalid filter: %v", err)
				return b, true, nil
			}
			b.expr = expr
			b.matcher = m
			b.input.Blur()
			b.input.SetValue("")
			b.state = filterActive
			b.err = ""
			return b, true, nil
		default:
			var cmd tea.Cmd
			b.input, cmd = b.input.Update(msg)
			return b, true, cmd
		}
	case filterActive:
		switch km.String() {
		case "x":
			b.expr = ""
			b.matcher = tagfilter.Matcher{}
			b.state = filterInactive
			return b, true, nil
		case "/":
			b.input.SetValue(b.expr)
			b.input.SetCursor(len(b.expr))
			b.input.Focus()
			b.state = filterPrompting
			return b, true, textinput.Blink
		}
	}
	return b, false, nil
}

// View returns the bar's rendered string. Empty when inactive.
func (b FilterBar) View() string {
	switch b.state {
	case filterPrompting:
		out := b.input.View()
		if b.err != "" {
			out += "\n  " + b.err
		}
		return out
	case filterActive:
		return fmt.Sprintf("[ filter: %s ]   x:clear", b.expr)
	}
	return ""
}
