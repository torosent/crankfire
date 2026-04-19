package screens_test

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tui/screens"
)

func TestEditFormSavesNewSession(t *testing.T) {
	s, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	m := screens.NewEdit(s, store.Session{})
	msgs := []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("smoke")},
		{Type: tea.KeyTab},
		{Type: tea.KeyRunes, Runes: []rune("https://example.com")},
		{Type: tea.KeyCtrlS},
	}
	var cur tea.Model = m
	for _, k := range msgs {
		cur, _ = cur.Update(k)
	}
	list, err := s.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Name != "smoke" {
		t.Fatalf("expected one session named smoke, got %#v", list)
	}
	if !strings.Contains(list[0].Config.TargetURL, "example.com") {
		t.Errorf("target not saved: %#v", list[0].Config)
	}
	if list[0].ID == "" {
		t.Error("expected ID to be assigned")
	}
}

func TestEditFormTabNavigation(t *testing.T) {
	s, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	m := screens.NewEdit(s, store.Session{})
	if got := m.FocusIndex(); got != 0 {
		t.Fatalf("initial focus = %d, want 0", got)
	}
	cur, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if got := cur.(screens.Edit).FocusIndex(); got != 1 {
		t.Fatalf("after tab focus = %d, want 1", got)
	}
	cur, _ = cur.(screens.Edit).Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if got := cur.(screens.Edit).FocusIndex(); got != 0 {
		t.Fatalf("after shift-tab focus = %d, want 0", got)
	}
}

func TestEditFormValidationEmptyName(t *testing.T) {
	s, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	m := screens.NewEdit(s, store.Session{})
	cur, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cur.(screens.Edit).Err() == nil {
		t.Error("expected validation error for empty name")
	}
	list, _ := s.ListSessions(context.Background())
	if len(list) != 0 {
		t.Errorf("expected no sessions saved, got %d", len(list))
	}
}

func TestEditFormReusesIDOnEdit(t *testing.T) {
	s, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	existing := store.Session{ID: "01ABCDEFGHIJKLMNOPQRSTUVWX", Name: "old"}
	m := screens.NewEdit(s, existing)
	// Clear name field, type new name.
	cur, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	cur, _ = cur.(screens.Edit).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("renamed")})
	cur, _ = cur.(screens.Edit).Update(tea.KeyMsg{Type: tea.KeyTab})
	cur, _ = cur.(screens.Edit).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("https://x.test")})
	cur, _ = cur.(screens.Edit).Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	list, _ := s.ListSessions(context.Background())
	if len(list) != 1 {
		t.Fatalf("expected 1 session, got %d", len(list))
	}
	if list[0].ID != existing.ID {
		t.Errorf("expected reused ID %q, got %q", existing.ID, list[0].ID)
	}
}
