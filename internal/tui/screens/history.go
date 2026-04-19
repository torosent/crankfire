package screens

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/store"
)

type HistoryLoadedMsg struct {
	Runs []store.Run
	Err  error
}

type History struct {
	store     store.Store
	sessionID string
	runs      []store.Run
	cursor    int
	err       error
}

func NewHistory(s store.Store, sessionID string) History {
	return History{store: s, sessionID: sessionID}
}

func (h History) Init() tea.Cmd {
	return func() tea.Msg {
		runs, err := h.store.ListRuns(context.Background(), h.sessionID)
		return HistoryLoadedMsg{Runs: runs, Err: err}
	}
}

func (h History) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case HistoryLoadedMsg:
		h.runs = m.Runs
		h.err = m.Err
		h.cursor = 0
	case tea.KeyMsg:
		switch m.String() {
		case "esc":
			return h, popCmd
		case "up", "k":
			if h.cursor > 0 {
				h.cursor--
			}
		case "down", "j":
			if h.cursor < len(h.runs)-1 {
				h.cursor++
			}
		case "o":
			if len(h.runs) > 0 {
				run := h.runs[h.cursor]
				reportPath := filepath.Join(run.Dir, "report.html")
				if _, err := os.Stat(reportPath); err == nil {
					// Don't block the UI
					go func() {
						var cmd *exec.Cmd
						switch runtime.GOOS {
						case "darwin":
							cmd = exec.Command("open", reportPath)
						case "windows":
							cmd = exec.Command("start", reportPath)
						default:
							cmd = exec.Command("xdg-open", reportPath)
						}
						_ = cmd.Start()
					}()
				}
			}
		}
	}
	return h, nil
}

func (h History) View() string {
	var b strings.Builder
	b.WriteString("Run History\n\n")

	if h.err != nil {
		fmt.Fprintf(&b, "error: %v\n", h.err)
		return b.String()
	}

	if len(h.runs) == 0 {
		b.WriteString("(no runs yet)\n")
		b.WriteString("\n[Esc] back\n")
		return b.String()
	}

	// Header
	fmt.Fprintf(&b, "%-20s %-12s %-10s %-8s %-8s\n", "Started", "Status", "Total", "P95ms", "Errors")
	b.WriteString(strings.Repeat("-", 60) + "\n")

	for i, run := range h.runs {
		prefix := "  "
		if i == h.cursor {
			prefix = "> "
		}

		startedStr := run.StartedAt.Format("2006-01-02 15:04")
		statusStr := string(run.Status)
		totalStr := fmt.Sprintf("%d", run.Summary.TotalRequests)
		p95Str := fmt.Sprintf("%.1f", run.Summary.P95Ms)
		errorStr := fmt.Sprintf("%d", run.Summary.Errors)

		fmt.Fprintf(&b, "%s%-20s %-12s %-10s %-8s %-8s\n",
			prefix, startedStr, statusStr, totalStr, p95Str, errorStr)
	}

	b.WriteString("\n[o] open report  [Esc] back\n")
	return b.String()
}
