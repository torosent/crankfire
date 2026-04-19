package screens

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"

	"github.com/torosent/crankfire/internal/store"
	tplrender "github.com/torosent/crankfire/internal/template"
)

type templatePickerStage int

const (
	tpStageList   templatePickerStage = iota
	tpStageParams templatePickerStage = iota
	tpStageDone   templatePickerStage = iota
)

type templatesLoadedMsg struct {
	ids []string
	err error
}

type templateBodyLoadedMsg struct {
	body []byte
	err  error
}

type paramRow struct {
	key   textinput.Model
	value textinput.Model
}

// TemplatePicker is a two-stage flow: pick template, fill freeform params,
// then materialize a Set and hand off to the sets-edit screen.
type TemplatePicker struct {
	ctx       context.Context
	store     store.Store
	stage     templatePickerStage
	templates []string
	cursor    int
	err       error

	// stage 2:
	selectedID string
	body       []byte
	rows       []paramRow
	focusedRow int
	focusedCol int // 0=key, 1=value
	submitErr  string
	createdID  string
}

func NewTemplatePicker(ctx context.Context, st store.Store) *TemplatePicker {
	return &TemplatePicker{ctx: ctx, store: st}
}

func (p *TemplatePicker) Init() tea.Cmd {
	return func() tea.Msg {
		ids, err := p.store.ListTemplates(p.ctx)
		return templatesLoadedMsg{ids: ids, err: err}
	}
}

// IsOnParamForm reports whether the picker has advanced past template selection.
func (p *TemplatePicker) IsOnParamForm() bool { return p.stage == tpStageParams }

// CreatedSetID returns the ID of the set materialized by the picker
// (empty string if not yet created).
func (p *TemplatePicker) CreatedSetID() string { return p.createdID }

func (p *TemplatePicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case templatesLoadedMsg:
		p.templates, p.err = m.ids, m.err
		return p, nil
	case templateBodyLoadedMsg:
		p.body, p.err = m.body, m.err
		if m.err == nil {
			p.stage = tpStageParams
			p.rows = []paramRow{newParamRow()}
			p.rows[0].key.Focus()
		}
		return p, textinput.Blink
	case tea.KeyMsg:
		switch p.stage {
		case tpStageList:
			return p.updateList(m)
		case tpStageParams:
			return p.updateParams(m)
		}
	}
	return p, nil
}

func (p *TemplatePicker) updateList(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.String() {
	case "esc", "q":
		return NewSetsList(p.ctx, p.store), nil
	case "j", "down":
		if p.cursor < len(p.templates)-1 {
			p.cursor++
		}
	case "k", "up":
		if p.cursor > 0 {
			p.cursor--
		}
	case "enter":
		if len(p.templates) == 0 {
			return p, nil
		}
		p.selectedID = p.templates[p.cursor]
		return p, func() tea.Msg {
			body, err := p.store.GetTemplate(p.ctx, p.selectedID)
			return templateBodyLoadedMsg{body: body, err: err}
		}
	}
	return p, nil
}

func (p *TemplatePicker) updateParams(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.String() {
	case "esc":
		p.stage = tpStageList
		p.rows = nil
		p.submitErr = ""
		return p, nil
	case "tab":
		// move forward: col 0 -> col 1, then next row's col 0
		if p.focusedCol == 0 {
			p.focusedCol = 1
		} else {
			p.focusedCol = 0
			if p.focusedRow < len(p.rows)-1 {
				p.focusedRow++
			}
		}
		p.refocus()
		return p, nil
	case "shift+tab":
		if p.focusedCol == 1 {
			p.focusedCol = 0
		} else {
			p.focusedCol = 1
			if p.focusedRow > 0 {
				p.focusedRow--
			}
		}
		p.refocus()
		return p, nil
	case "ctrl+n":
		// add row
		p.rows = append(p.rows, newParamRow())
		p.focusedRow = len(p.rows) - 1
		p.focusedCol = 0
		p.refocus()
		return p, textinput.Blink
	case "ctrl+s":
		return p.submit()
	}
	// forward to focused field
	if p.focusedRow < len(p.rows) {
		var cmd tea.Cmd
		if p.focusedCol == 0 {
			p.rows[p.focusedRow].key, cmd = p.rows[p.focusedRow].key.Update(m)
		} else {
			p.rows[p.focusedRow].value, cmd = p.rows[p.focusedRow].value.Update(m)
		}
		return p, cmd
	}
	return p, nil
}

func (p *TemplatePicker) refocus() {
	for i := range p.rows {
		p.rows[i].key.Blur()
		p.rows[i].value.Blur()
	}
	if p.focusedRow >= len(p.rows) {
		return
	}
	if p.focusedCol == 0 {
		p.rows[p.focusedRow].key.Focus()
	} else {
		p.rows[p.focusedRow].value.Focus()
	}
}

func (p *TemplatePicker) submit() (tea.Model, tea.Cmd) {
	params := make(map[string]string, len(p.rows))
	for _, r := range p.rows {
		k := strings.TrimSpace(r.key.Value())
		if k == "" {
			continue
		}
		params[k] = r.value.Value()
	}
	rendered, err := tplrender.Render(p.body, params)
	if err != nil {
		p.submitErr = fmt.Sprintf("render: %v", err)
		return p, nil
	}
	rendered = stripTemplateMarkerForTUI(rendered)
	var set store.Set
	if err := yaml.Unmarshal(rendered, &set); err != nil {
		p.submitErr = fmt.Sprintf("yaml: %v", err)
		return p, nil
	}
	set.ID = ""
	set.SchemaVersion = 0
	set.CreatedAt = time.Time{}
	set.UpdatedAt = time.Time{}
	if err := p.store.SaveSet(p.ctx, set); err != nil {
		p.submitErr = fmt.Sprintf("save: %v", err)
		return p, nil
	}
	// Find the just-saved set (newest CreatedAt).
	sets, _ := p.store.ListSets(p.ctx)
	var newest store.Set
	for _, s := range sets {
		if s.CreatedAt.After(newest.CreatedAt) {
			newest = s
		}
	}
	p.createdID = newest.ID
	p.stage = tpStageDone
	return NewSetsList(p.ctx, p.store), nil
}

func (p *TemplatePicker) View() string {
	switch p.stage {
	case tpStageList:
		var b strings.Builder
		b.WriteString("Pick a template — enter)select  esc)cancel\n\n")
		if p.err != nil {
			fmt.Fprintf(&b, "Error: %v\n", p.err)
			return b.String()
		}
		if len(p.templates) == 0 {
			b.WriteString("(no templates in templates/)\n")
			return b.String()
		}
		for i, id := range p.templates {
			marker := "  "
			if i == p.cursor {
				marker = "> "
			}
			fmt.Fprintf(&b, "%s%s\n", marker, id)
		}
		return b.String()
	case tpStageParams:
		var b strings.Builder
		fmt.Fprintf(&b, "Params for template %q — tab)next  ctrl+n)add row  ctrl+s)submit  esc)back\n\n", p.selectedID)
		for i, r := range p.rows {
			fmt.Fprintf(&b, "  %s  =  %s", r.key.View(), r.value.View())
			if i == p.focusedRow {
				b.WriteString("  <")
			}
			b.WriteString("\n")
		}
		if p.submitErr != "" {
			fmt.Fprintf(&b, "\nerror: %s\n", p.submitErr)
		}
		return b.String()
	}
	return ""
}

func newParamRow() paramRow {
	k := textinput.New()
	k.Placeholder = "key"
	k.CharLimit = 64
	v := textinput.New()
	v.Placeholder = "value"
	v.CharLimit = 256
	return paramRow{key: k, value: v}
}

// stripTemplateMarkerForTUI removes the `template: true` marker line before
// unmarshalling so the rendered YAML is a valid Set.
func stripTemplateMarkerForTUI(in []byte) []byte {
	lines := strings.Split(string(in), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "template: true" || trimmed == "template: \"true\"" {
			continue
		}
		out = append(out, line)
	}
	return []byte(strings.Join(out, "\n"))
}
