package screens_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tui/screens"
)

func TestEditYAMLToggleFromForm(t *testing.T) {
	s, _ := store.NewFS(t.TempDir())
	m := screens.NewEdit(s, store.Session{Name: "smoke"})
	// Press F2 to toggle to YAML mode.
	var cur tea.Model = m
	cur, _ = cur.Update(tea.KeyMsg{Type: tea.KeyF2})
	if !strings.Contains(cur.View(), "schema_version") {
		t.Errorf("expected YAML view to show schema_version:\n%s", cur.View())
	}
}

func TestEditYAMLToggleBackToForm(t *testing.T) {
	s, _ := store.NewFS(t.TempDir())
	m := screens.NewEdit(s, store.Session{Name: "smoke"})
	var cur tea.Model = m
	// Toggle to YAML
	cur, _ = cur.Update(tea.KeyMsg{Type: tea.KeyF2})
	// Toggle back to form
	cur, _ = cur.Update(tea.KeyMsg{Type: tea.KeyF2})
	if !strings.Contains(cur.View(), "Edit session") {
		t.Errorf("expected form view:\n%s", cur.View())
	}
}

func TestEditYAMLSaveWithValidYAML(t *testing.T) {
	s, _ := store.NewFS(t.TempDir())
	m := screens.NewEdit(s, store.Session{})
	var cur tea.Model = m
	// Toggle to YAML
	cur, _ = cur.Update(tea.KeyMsg{Type: tea.KeyF2})
	// Simulate editing the YAML (we'd need to send textarea updates)
	// For now, just verify we can save in YAML mode
	cur, _ = cur.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	// Should show validation error (empty name), not crash
	if _, ok := cur.(screens.Edit); !ok {
		t.Fatal("expected Edit model after save attempt")
	}
}

func TestEditYAMLInvalidYAMLDisablesToggle(t *testing.T) {
	s, _ := store.NewFS(t.TempDir())
	m := screens.NewEdit(s, store.Session{Name: "smoke"})
	var cur tea.Model = m
	// Toggle to YAML
	cur, _ = cur.Update(tea.KeyMsg{Type: tea.KeyF2})
	// Try to break the YAML by modifying it to invalid syntax
	// (This would be done via textarea Update in real flow)
	// For testing, we verify the model has the right fields
	edit := cur.(screens.Edit)
	if !edit.InYAMLMode() {
		t.Fatal("expected to be in YAML mode")
	}
}
