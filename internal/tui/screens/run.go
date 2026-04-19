package screens

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/cli"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/output"
	"github.com/torosent/crankfire/internal/runner"
	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tui/runview"
)

const runTickInterval = 250 * time.Millisecond

// runStatus is the lifecycle state of the Run screen.
type runStatus int

const (
	runStatusStarting runStatus = iota
	runStatusRunning
	runStatusCancelled
	runStatusCompleted
	runStatusFailed
)

func (s runStatus) String() string {
	switch s {
	case runStatusStarting:
		return "starting"
	case runStatusRunning:
		return "running"
	case runStatusCancelled:
		return "cancelled"
	case runStatusCompleted:
		return "completed"
	case runStatusFailed:
		return "failed"
	}
	return "unknown"
}

// runStartedMsg is delivered after BuildRunner + CreateRun succeed and the
// background goroutine has been launched.
type runStartedMsg struct {
	run       store.Run
	collector *metrics.Collector
	cancel    context.CancelFunc
	err       error
}

// runFinishedMsg is delivered when the background goroutine exits.
type runFinishedMsg struct {
	result runner.Result
	err    error
}

// runTickMsg is delivered by the periodic snapshot ticker.
type runTickMsg struct{}

// Run drives an in-process load test for a session, polls live metrics, and
// finalizes the run via the store on exit.
type Run struct {
	store store.Store
	sess  store.Session

	mu        sync.Mutex
	status    runStatus
	statusMsg string

	run       store.Run
	collector *metrics.Collector
	cancel    context.CancelFunc

	view runview.Model

	finalized bool
}

// NewRun creates a Run screen for the given session.
func NewRun(s store.Store, sess store.Session) *Run {
	return &Run{
		store:  s,
		sess:   sess,
		status: runStatusStarting,
		view: runview.New(runview.Options{
			Title: sess.Name,
			Total: int64(sess.Config.Total),
		}),
	}
}

// Init kicks off the run: creates the run record, builds the runner, and
// schedules the first snapshot tick.
func (r *Run) Init() tea.Cmd {
	startCmd := func() tea.Msg {
		ctx := context.Background()
		run, err := r.store.CreateRun(ctx, r.sess.ID)
		if err != nil {
			return runStartedMsg{err: fmt.Errorf("create run: %w", err)}
		}

		runCtx, cancel := context.WithCancel(context.Background())
		runnerInst, collector, cleanup, err := cli.BuildRunner(runCtx, r.sess.Config)
		if err != nil {
			cancel()
			return runStartedMsg{run: run, err: fmt.Errorf("build runner: %w", err)}
		}

		go func() {
			defer cleanup()
			collector.Start()
			result := runnerInst.Run(runCtx)
			collector.Snapshot()
			runFinishedProgram(r, result, runCtx.Err())
		}()

		return runStartedMsg{run: run, collector: collector, cancel: cancel}
	}
	return tea.Batch(startCmd, runTickCmd())
}

// runFinishedProgram is a package-level hook so the goroutine can deliver the
// runFinishedMsg into the model. We use a small registry keyed by Run pointer
// because tea.Cmd functions are evaluated synchronously off the main loop and
// can't easily marshal channel state across an Update call. The registry is
// drained by the next runTickMsg.
var (
	finishMu      sync.Mutex
	finishPending = map[*Run]runFinishedMsg{}
)

func runFinishedProgram(r *Run, result runner.Result, ctxErr error) {
	finishMu.Lock()
	defer finishMu.Unlock()
	finishPending[r] = runFinishedMsg{result: result, err: ctxErr}
}

func consumeFinish(r *Run) (runFinishedMsg, bool) {
	finishMu.Lock()
	defer finishMu.Unlock()
	msg, ok := finishPending[r]
	if ok {
		delete(finishPending, r)
	}
	return msg, ok
}

func runTickCmd() tea.Cmd {
	return tea.Tick(runTickInterval, func(time.Time) tea.Msg { return runTickMsg{} })
}

// Update advances the model.
func (r *Run) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "q", "esc", "ctrl+c":
			return r, r.cancelRun()
		}
	case runStartedMsg:
		r.mu.Lock()
		r.run = m.run
		r.collector = m.collector
		r.cancel = m.cancel
		if m.err != nil {
			r.status = runStatusFailed
			r.statusMsg = m.err.Error()
			r.mu.Unlock()
			return r, r.finalizeCmd(runner.Result{}, m.err)
		}
		if r.status == runStatusStarting {
			r.status = runStatusRunning
		}
		r.mu.Unlock()
		return r, nil
	case runTickMsg:
		// Drain any background-finish event first.
		if fin, ok := consumeFinish(r); ok {
			return r, r.handleFinished(fin)
		}
		var cmds []tea.Cmd
		if r.collector != nil {
			r.collector.Snapshot()
			hist := r.collector.History()
			if len(hist) > 0 {
				snap := hist[len(hist)-1]
				updated, _ := r.view.Update(runview.SnapshotMsg{Snap: snap})
				r.view = updated
			}
		}
		if !r.isFinalized() {
			cmds = append(cmds, runTickCmd())
		}
		return r, tea.Batch(cmds...)
	case runFinishedMsg:
		return r, r.handleFinished(m)
	}
	return r, nil
}

func (r *Run) cancelRun() tea.Cmd {
	r.mu.Lock()
	if r.status == runStatusCompleted || r.status == runStatusFailed || r.status == runStatusCancelled {
		r.mu.Unlock()
		return popCmd
	}
	r.status = runStatusCancelled
	r.statusMsg = "cancelled by user"
	cancel := r.cancel
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	// If the runner never started (BuildRunner failed or hadn't completed),
	// finalize immediately so the screen exits cleanly.
	r.mu.Lock()
	hasRun := r.run.SessionID != ""
	finalized := r.finalized
	r.mu.Unlock()
	if hasRun && !finalized {
		// Wait for the background goroutine to deliver runFinishedMsg via the
		// next tick; if no goroutine is running, finalize now.
		if r.collector == nil {
			return r.finalizeCmd(runner.Result{}, context.Canceled)
		}
	} else if !hasRun {
		return r.finalizeCmd(runner.Result{}, context.Canceled)
	}
	return nil
}

func (r *Run) handleFinished(m runFinishedMsg) tea.Cmd {
	r.mu.Lock()
	if r.status != runStatusCancelled {
		if m.err != nil && !errors.Is(m.err, context.Canceled) {
			r.status = runStatusFailed
			r.statusMsg = m.err.Error()
		} else if m.result.Errors > 0 {
			r.status = runStatusCompleted
			r.statusMsg = fmt.Sprintf("%d errors", m.result.Errors)
		} else {
			r.status = runStatusCompleted
		}
	}
	r.mu.Unlock()
	return r.finalizeCmd(m.result, m.err)
}

func (r *Run) finalizeCmd(result runner.Result, runErr error) tea.Cmd {
	return func() tea.Msg {
		r.mu.Lock()
		if r.finalized {
			r.mu.Unlock()
			return PopMsg{}
		}
		r.finalized = true
		run := r.run
		collector := r.collector
		status := r.status
		statusMsg := r.statusMsg
		r.mu.Unlock()

		var summary store.RunSummary
		if collector != nil {
			stats := collector.Stats(result.Duration)
			summary = store.RunSummary{
				TotalRequests: stats.Total,
				Errors:        stats.Failures,
				DurationSec:   result.Duration.Seconds(),
				P50Ms:         stats.P50LatencyMs,
				P95Ms:         stats.P95LatencyMs,
				P99Ms:         stats.P99LatencyMs,
			}
		}
		if statusMsg != "" {
			summary.ErrorMessage = statusMsg
		}

		// Map our internal status onto the store's RunStatus.
		switch status {
		case runStatusCompleted:
			run.Status = store.RunStatusCompleted
		case runStatusFailed:
			run.Status = store.RunStatusFailed
		case runStatusCancelled:
			run.Status = store.RunStatusCancelled
		default:
			run.Status = store.RunStatusFailed
		}

		// Write result.json + report.html into the run directory if we have
		// a collector and the directory exists.
		if collector != nil && run.Dir != "" {
			stats := collector.Stats(result.Duration)
			if f, err := os.Create(filepath.Join(run.Dir, "result.json")); err == nil {
				_ = output.PrintJSONReport(f, stats, nil)
				_ = f.Close()
			}
			if f, err := os.Create(filepath.Join(run.Dir, "report.html")); err == nil {
				_ = output.GenerateHTMLReport(f, stats, collector.History(), nil, output.ReportMetadata{
					TargetURL: r.sess.Config.TargetURL,
				})
				_ = f.Close()
			}
		}

		if run.SessionID != "" {
			_ = r.store.FinalizeRun(context.Background(), run, summary)
		}

		return PopMsg{}
	}
}

func (r *Run) isFinalized() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.finalized
}

// View renders the live dashboard plus a status footer.
func (r *Run) View() string {
	r.mu.Lock()
	status := r.status
	statusMsg := r.statusMsg
	r.mu.Unlock()

	var b strings.Builder
	b.WriteString(r.view.View())
	b.WriteString("\nStatus: ")
	b.WriteString(status.String())
	if statusMsg != "" {
		b.WriteString(" — ")
		b.WriteString(statusMsg)
	}
	b.WriteString("\n[q/esc] cancel\n")
	return b.String()
}
