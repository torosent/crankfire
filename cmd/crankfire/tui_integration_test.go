//go:build integration

package main_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hinshun/vt10x"
	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tui"
)

// TestTUIEndToEnd drives the bubbletea TUI through a realistic
// import -> list -> run flow and asserts the on-disk store reaches the
// expected state. Per the plan, the test is intentionally tolerant: it
// inspects the user-visible promise (result.json on disk) rather than
// asserting exact terminal pixels.
func TestTUIEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dir := t.TempDir()
	s, err := store.NewFS(dir)
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}

	ctx := context.Background()
	sess := store.Session{
		Name: "smoke",
		Config: config.Config{
			TargetURL: srv.URL,
			Protocol:  config.ProtocolHTTP,
			Total:       5,
			Concurrency: 1,
		},
	}
	if err := s.SaveSession(ctx, sess); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	list, err := s.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 session, got %d", len(list))
	}
	sid := list[0].ID

	// Virtual terminal sink so bubbletea's escape-sequence output is parsed
	// instead of polluting the test's stdout.
	term := vt10x.New(vt10x.WithSize(120, 40))

	// os.Pipe gives us an *os.File for input, which bubbletea handles cleanly.
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer inR.Close()
	defer inW.Close()

	p := tea.NewProgram(
		tui.NewRoot(s),
		tea.WithInput(inR),
		tea.WithOutput(term),
	)

	progDone := make(chan error, 1)
	go func() {
		_, err := p.Run()
		progDone <- err
	}()

	// Give the program a moment to register its input handler before we
	// send keystrokes.
	time.Sleep(200 * time.Millisecond)

	// Press 'r' to launch a run for the only (selected) session.
	if _, err := inW.Write([]byte("r")); err != nil {
		t.Fatalf("write 'r': %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		runs, err := s.ListRuns(ctx, sid)
		if err == nil && len(runs) == 1 && runs[0].Status != store.RunStatusRunning {
			if runs[0].Status != store.RunStatusCompleted {
				p.Quit()
				<-progDone
				t.Fatalf("run status = %q, want completed", runs[0].Status)
			}
			if _, err := os.Stat(filepath.Join(runs[0].Dir, "result.json")); err != nil {
				p.Quit()
				<-progDone
				t.Fatalf("result.json missing: %v", err)
			}
			p.Quit()
			select {
			case <-progDone:
			case <-time.After(5 * time.Second):
				t.Fatalf("program did not exit after Quit")
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	p.Quit()
	<-progDone
	t.Fatalf("run did not finalize within deadline")
}
