package screens_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tui/screens"
)

func TestHistoryShowsRuns(t *testing.T) {
	s, _ := store.NewFS(t.TempDir())
	sess := store.Session{Name: "x"}
	s.SaveSession(context.Background(), sess)
	list, _ := s.ListSessions(context.Background())
	sid := list[0].ID
	run, _ := s.CreateRun(context.Background(), sid)
	os.WriteFile(filepath.Join(run.Dir, "report.html"), []byte("<html/>"), 0o644)
	s.FinalizeRun(context.Background(), run, store.RunSummary{TotalRequests: 42, P95Ms: 12})

	m := screens.NewHistory(s, sid)
	var model tea.Model = m

	// Simulate the async load by calling the Init function directly
	initCmd := m.Init()
	loadedMsg := initCmd().(screens.HistoryLoadedMsg)

	model, _ = model.Update(loadedMsg)
	if !strings.Contains(model.View(), "42") {
		t.Errorf("history view missing total: %s", model.View())
	}
}
