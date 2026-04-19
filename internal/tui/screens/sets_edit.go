package screens

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"gopkg.in/yaml.v3"

	"github.com/torosent/crankfire/internal/store"
)

type SetsEdit struct {
	ctx    context.Context
	store  store.Store
	mode   setsEditMode
	set    store.Set
	name   textinput.Model
	desc   textinput.Model
	yaml   textarea.Model
	status string
	saved  bool
	width  int
	height int
}

type setsEditMode int

const (
	setsEditModeForm setsEditMode = iota
	setsEditModeYAML
)

func NewSetsEdit(ctx context.Context, st store.Store, initial store.Set) tea.Model {
	name := textinput.New()
	name.Placeholder = "set name"
	name.SetValue(initial.Name)
	name.Focus()

	desc := textinput.New()
	desc.Placeholder = "description (optional)"
	desc.SetValue(initial.Description)

	ta := textarea.New()
	ta.Placeholder = "raw YAML (advanced)"
	if data, err := yaml.Marshal(initial); err == nil {
		ta.SetValue(string(data))
	}

	return &SetsEdit{
		ctx: ctx, store: st, mode: setsEditModeForm, set: initial,
		name: name, desc: desc, yaml: ta,
	}
}

func (m *SetsEdit) Init() tea.Cmd { return textinput.Blink }

func (m *SetsEdit) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.yaml.SetWidth(msg.Width - 4)
		m.yaml.SetHeight(msg.Height - 8)
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return NewSetsList(m.ctx, m.store), nil
		case "y":
			if m.mode == setsEditModeForm {
				m.mode = setsEditModeYAML
				if data, err := yaml.Marshal(m.collectFromForm()); err == nil {
					m.yaml.SetValue(string(data))
				}
				m.yaml.Focus()
				return m, nil
			}
		case "f":
			if m.mode == setsEditModeYAML {
				var s store.Set
				if err := yaml.Unmarshal([]byte(m.yaml.Value()), &s); err == nil {
					m.set = s
					m.name.SetValue(s.Name)
					m.desc.SetValue(s.Description)
				} else {
					m.status = fmt.Sprintf("YAML error: %v", err)
				}
				m.mode = setsEditModeForm
				return m, nil
			}
		case "ctrl+s":
			return m.Save()
		case "tab":
			if m.mode == setsEditModeForm {
				if m.name.Focused() {
					m.name.Blur()
					m.desc.Focus()
				} else {
					m.desc.Blur()
					m.name.Focus()
				}
			}
		}
	}
	var cmd tea.Cmd
	if m.mode == setsEditModeForm {
		if m.name.Focused() {
			m.name, cmd = m.name.Update(msg)
		} else {
			m.desc, cmd = m.desc.Update(msg)
		}
	} else {
		m.yaml, cmd = m.yaml.Update(msg)
	}
	return m, cmd
}

// Save is exported so tests can invoke it without simulating keystrokes.
func (m *SetsEdit) Save() (tea.Model, tea.Cmd) {
	var s store.Set
	if m.mode == setsEditModeYAML {
		if err := yaml.Unmarshal([]byte(m.yaml.Value()), &s); err != nil {
			m.status = fmt.Sprintf("YAML parse: %v", err)
			return m, nil
		}
	} else {
		s = m.collectFromForm()
	}
	if err := m.store.SaveSet(m.ctx, s); err != nil {
		m.status = fmt.Sprintf("save: %v", err)
		return m, nil
	}
	m.saved = true
	return NewSetsList(m.ctx, m.store), nil
}

func (m *SetsEdit) collectFromForm() store.Set {
	out := m.set
	out.Name = m.name.Value()
	out.Description = m.desc.Value()
	return out
}

func (m *SetsEdit) View() string {
	var b strings.Builder
	switch m.mode {
	case setsEditModeForm:
		b.WriteString("Form — y)aml mode  ctrl+s)ave  esc)ancel\n\n")
		b.WriteString("Name: " + m.name.View() + "\n")
		b.WriteString("Desc: " + m.desc.View() + "\n\n")
		b.WriteString("(Stage/item editing: edit YAML directly with y for now.)\n")
	case setsEditModeYAML:
		b.WriteString("YAML — f)orm mode  ctrl+s)ave  esc)ancel\n\n")
		b.WriteString(m.yaml.View() + "\n")
	}
	if m.status != "" {
		fmt.Fprintf(&b, "\n%s\n", m.status)
	}
	return b.String()
}
