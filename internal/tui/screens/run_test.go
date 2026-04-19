package screens_test

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tui/screens"
)

func TestRunScreenStartsAndCancels(t *testing.T) {
	s, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}
	sess := store.Session{Name: "x", Config: config.Config{
		TargetURL: "http://127.0.0.1:0", Protocol: config.ProtocolHTTP, Total: 1,
		Concurrency: 1, Timeout: 100 * time.Millisecond,
	}}
	if err := s.SaveSession(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	m := screens.NewRun(s, list[0])
	var cur tea.Model = m
	cur.Init()
	// Send Esc to cancel; expect a finalize message back.
	cur, _ = cur.Update(tea.KeyMsg{Type: tea.KeyEsc})

	// Drive the model until it finalizes. We dispatch any tea.Cmd messages
	// the model emits so its background goroutines and tickers can deliver
	// their results.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		view := cur.View()
		if strings.Contains(view, "cancelled") || strings.Contains(view, "completed") || strings.Contains(view, "failed") {
			return
		}
		time.Sleep(20 * time.Millisecond)
		// Pump a no-op update so any pending tea.Cmds get drained via
		// follow-up messages produced by the goroutine.
		cur, _ = cur.Update(tickPing{})
	}
	t.Errorf("run screen did not finalize within deadline; view=%q", cur.View())
}

type tickPing struct{}
