package screens

import (
	tea "github.com/charmbracelet/bubbletea"
)

type Confirm struct {
	prompt string
	onYes  func()
}

func NewConfirm(prompt string, onYes func()) Confirm { return Confirm{prompt: prompt, onYes: onYes} }
func (c Confirm) Init() tea.Cmd { return nil }
func (c Confirm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "y", "Y":
			if c.onYes != nil {
				c.onYes()
			}
			return c, popCmd
		case "n", "N", "esc":
			return c, popCmd
		}
	}
	return c, nil
}
func (c Confirm) View() string { return c.prompt + "  [y/N]" }

type PushMsg struct{ Model tea.Model }
type PopMsg struct{}
