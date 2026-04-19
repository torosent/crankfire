package screens_test

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tui/screens"
)

func TestTemplatePickerListsTemplates(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	ctx := context.Background()
	for _, id := range []string{"baseline", "smoke"} {
		_ = st.SaveTemplate(ctx, id, []byte("template: true\nname: x\nstages: []\n"))
	}
	p := screens.NewTemplatePicker(ctx, st)
	cmd := p.Init()
	if cmd != nil {
		msg := cmd()
		m, _ := p.Update(msg)
		p = m.(*screens.TemplatePicker)
	}
	v := p.View()
	if !strings.Contains(v, "baseline") || !strings.Contains(v, "smoke") {
		t.Errorf("view missing template IDs:\n%s", v)
	}
}

func TestTemplatePickerEnterAdvancesToParamForm(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	ctx := context.Background()
	_ = st.SaveTemplate(ctx, "baseline", []byte("template: true\nname: api-{{ .Env }}\nstages: []\n"))
	p := screens.NewTemplatePicker(ctx, st)
	cmd := p.Init()
	msg := cmd()
	m, _ := p.Update(msg)
	p = m.(*screens.TemplatePicker)
	// Enter
	m, cmd = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	p = m.(*screens.TemplatePicker)
	if cmd != nil {
		msg = cmd()
		m, _ = p.Update(msg)
		p = m.(*screens.TemplatePicker)
	}
	if !p.IsOnParamForm() {
		t.Error("enter should advance to param form")
	}
}
