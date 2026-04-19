package screens

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/store"
)

type Import struct {
	store  store.Store
	fields []textinput.Model
	labels []string
	focus  int
	err    error
}

func NewImport(s store.Store) Import {
	labels := []string{"Path", "Name"}
	fields := make([]textinput.Model, len(labels))
	for i := range fields {
		fields[i] = textinput.New()
		fields[i].CharLimit = 256
	}
	fields[0].Focus()
	return Import{
		store:  s,
		fields: fields,
		labels: labels,
	}
}

func (i Import) Init() tea.Cmd { return textinput.Blink }

func (i Import) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyTab, tea.KeyDown:
			i.fields[i.focus].Blur()
			i.focus = (i.focus + 1) % len(i.fields)
			i.fields[i.focus].Focus()
			return i, nil
		case tea.KeyShiftTab, tea.KeyUp:
			i.fields[i.focus].Blur()
			i.focus = (i.focus - 1 + len(i.fields)) % len(i.fields)
			i.fields[i.focus].Focus()
			return i, nil
		case tea.KeyEnter:
			return i.submit()
		case tea.KeyEsc:
			return i, popCmd
		}
	}
	var cmd tea.Cmd
	i.fields[i.focus], cmd = i.fields[i.focus].Update(msg)
	return i, cmd
}

func (i Import) submit() (tea.Model, tea.Cmd) {
	path := strings.TrimSpace(i.fields[0].Value())
	name := strings.TrimSpace(i.fields[1].Value())

	if path == "" {
		i.err = errors.New("path is required")
		return i, nil
	}

	if name == "" {
		i.err = errors.New("name is required")
		return i, nil
	}

	// Validate file exists
	if _, err := os.Stat(path); err != nil {
		i.err = fmt.Errorf("file not found: %w", err)
		return i, nil
	}

	// Import the session
	sess, err := i.store.ImportSessionFromConfigFile(context.Background(), path, name)
	if err != nil {
		i.err = err
		return i, nil
	}

	// On success, create commands to pop self and push Edit with the new session
	return i, tea.Sequence(
		func() tea.Msg { return PushMsg{NewEdit(i.store, sess)} },
		func() tea.Msg { return PopMsg{} },
	)
}

func (i Import) View() string {
	var b strings.Builder
	b.WriteString("Import session\n\n")
	for j, f := range i.fields {
		fmt.Fprintf(&b, "%-8s %s\n", i.labels[j]+":", f.View())
	}
	if i.err != nil {
		fmt.Fprintf(&b, "\nerror: %v\n", i.err)
	}
	b.WriteString("\n[Tab/Shift+Tab] navigate  [Enter] import  [Esc] cancel\n")
	return b.String()
}
