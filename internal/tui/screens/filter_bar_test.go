package screens_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/tui/screens"
)

func key(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestFilterBarStartsInactive(t *testing.T) {
	bar := screens.NewFilterBar()
	if bar.IsActive() {
		t.Error("new bar should be inactive")
	}
	if bar.Expr() != "" {
		t.Errorf("expr = %q, want empty", bar.Expr())
	}
	if !bar.Matcher().IsZero() {
		t.Error("matcher should be zero (match all)")
	}
}

func TestFilterBarSlashOpensPrompt(t *testing.T) {
	bar := screens.NewFilterBar()
	bar, handled, _ := bar.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if !handled {
		t.Fatal("/ should be consumed by bar")
	}
	if !bar.IsPrompting() {
		t.Error("bar should be prompting after /")
	}
	v := bar.View()
	if !strings.Contains(v, "filter:") && !strings.Contains(v, "/") {
		t.Errorf("view does not show prompt: %q", v)
	}
}

func TestFilterBarEnterAppliesExpr(t *testing.T) {
	bar := screens.NewFilterBar()
	bar, _, _ = bar.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	for _, r := range "prod" {
		bar, _, _ = bar.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	bar, _, _ = bar.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if bar.IsPrompting() {
		t.Error("Enter should exit prompt")
	}
	if !bar.IsActive() {
		t.Error("bar should be active")
	}
	if bar.Expr() != "prod" {
		t.Errorf("expr = %q, want %q", bar.Expr(), "prod")
	}
	if !bar.Matcher().Matches([]string{"prod"}) {
		t.Error("matcher should match [prod]")
	}
	if bar.Matcher().Matches([]string{"staging"}) {
		t.Error("matcher should not match [staging]")
	}
}

func TestFilterBarEscClearsPrompt(t *testing.T) {
	bar := screens.NewFilterBar()
	bar, _, _ = bar.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	bar, _, _ = bar.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	bar, _, _ = bar.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if bar.IsPrompting() {
		t.Error("Esc should leave prompt")
	}
	if bar.IsActive() {
		t.Error("Esc with no prior expr should leave bar inactive")
	}
}

func TestFilterBarXClearsActive(t *testing.T) {
	bar := screens.NewFilterBar()
	bar.SetExpr("prod")
	bar, handled, _ := bar.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if !handled {
		t.Error("x should be consumed when active")
	}
	if bar.IsActive() {
		t.Error("bar should be inactive after x")
	}
}

func TestFilterBarInvalidExprShowsError(t *testing.T) {
	bar := screens.NewFilterBar()
	bar, _, _ = bar.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	for _, r := range "bad!" {
		bar, _, _ = bar.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	bar, _, _ = bar.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Invalid filter: still prompting with error visible
	if !bar.IsPrompting() {
		t.Error("invalid filter should keep bar prompting")
	}
	if !strings.Contains(bar.View(), "invalid") {
		t.Errorf("view should show error, got %q", bar.View())
	}
}

func TestFilterBarSetExprApplies(t *testing.T) {
	bar := screens.NewFilterBar()
	if err := bar.SetExpr("prod,staging"); err != nil {
		t.Fatalf("SetExpr: %v", err)
	}
	if !bar.IsActive() {
		t.Error("bar should be active after SetExpr")
	}
	if !bar.Matcher().Matches([]string{"staging"}) {
		t.Error("matcher should match [staging]")
	}
}
