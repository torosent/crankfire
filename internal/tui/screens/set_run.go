package screens

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/torosent/crankfire/internal/setrunner"
	"github.com/torosent/crankfire/internal/store"
)

type setRunEvtMsg struct{ e setrunner.Event }

type setRunDoneMsg struct {
	run store.SetRun
	err error
}

// SetRun is the live set-execution screen. It launches the setrunner in a
// goroutine and reflects per-item progress via the event channel.
type SetRun struct {
	// parentCtx is the uncancelled context used when navigating away.
	parentCtx context.Context
	// runCtx / cancel are the child context used exclusively by the runner goroutine.
	runCtx  context.Context
	cancel  context.CancelFunc
	store   store.Store
	builder setrunner.Builder
	setID   string
	setName string

	currentStage string
	itemStatus   map[string]string  // item name → status label
	itemP95      map[string]float64 // item name → p95 latency ms

	finalRun *store.SetRun
	err      error
	width    int

	events chan setrunner.Event
	doneCh chan setRunDoneMsg
}

// NewSetRun constructs the live set-run screen.
// builder is the setrunner.Builder used to execute each item (pass cli.NewSetBuilder()
// in production; a fake in tests).
func NewSetRun(ctx context.Context, st store.Store, builder setrunner.Builder, setID string) tea.Model {
	runCtx, cancel := context.WithCancel(ctx)
	return &SetRun{
		parentCtx:  ctx,
		runCtx:     runCtx,
		cancel:     cancel,
		store:      st,
		builder:    builder,
		setID:      setID,
		itemStatus: map[string]string{},
		itemP95:    map[string]float64{},
		doneCh:     make(chan setRunDoneMsg, 1),
	}
}

// Init loads the set name synchronously (so View shows it immediately) then
// starts the runner goroutine and returns two long-lived tea.Cmds: one that
// blocks on the event channel and one that blocks for the final done signal.
func (m *SetRun) Init() tea.Cmd {
	if s, err := m.store.GetSet(m.runCtx, m.setID); err == nil {
		m.setName = s.Name
	}
	m.events = make(chan setrunner.Event, 64)
	r := setrunner.New(m.store, m.builder)
	go func() {
		run, err := r.Run(m.runCtx, m.setID, m.events)
		close(m.events)
		m.doneCh <- setRunDoneMsg{run: run, err: err}
	}()
	return tea.Batch(m.waitEvent(), m.waitDone())
}

// waitEvent returns a blocking tea.Cmd that reads one event from the channel
// and delivers it as setRunEvtMsg. Re-issued after every event so we never
// miss events. Returns nil when the channel is closed.
func (m *SetRun) waitEvent() tea.Cmd {
	return func() tea.Msg {
		e, ok := <-m.events
		if !ok {
			return nil
		}
		return setRunEvtMsg{e}
	}
}

// waitDone blocks until the runner goroutine sends its final result.
func (m *SetRun) waitDone() tea.Cmd {
	return func() tea.Msg {
		return <-m.doneCh
	}
}

func (m *SetRun) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case setRunEvtMsg:
		m.applyEvent(msg.e)
		return m, m.waitEvent()

	case setRunDoneMsg:
		m.finalRun = &msg.run
		m.err = msg.err

	case tea.WindowSizeMsg:
		m.width = msg.Width

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.cancel()
			if m.finalRun != nil {
				return NewSetCompare(m.parentCtx, m.store, *m.finalRun), nil
			}
			return NewSetsDetail(m.parentCtx, m.store, m.setID), nil
		}
	}
	return m, nil
}

// IngestForTest is a test-only hook that applies an event directly without the
// goroutine. This lets unit tests verify rendering without running a real runner.
func (m *SetRun) IngestForTest(ev setrunner.Event) { m.applyEvent(ev) }

// WaitForTest blocks until the runner goroutine has fully exited. Test-only
// helper so tempdir cleanup doesn't race with background writes.
func (m *SetRun) WaitForTest() { <-m.doneCh }

func (m *SetRun) applyEvent(ev setrunner.Event) {
	switch ev.Kind {
	case setrunner.EventStageStarted:
		m.currentStage = ev.Stage
	case setrunner.EventItemStarted:
		m.itemStatus[ev.Item] = "running"
	case setrunner.EventItemEnded:
		if ev.Result != nil {
			m.itemStatus[ev.Item] = string(ev.Result.Status)
			m.itemP95[ev.Item] = ev.Result.Summary.P95Ms
		}
	}
}

func (m *SetRun) View() string {
	var b strings.Builder
	b.WriteString("Set Run — q)cancel  esc)back\n\n")
	name := m.setName
	if name == "" {
		name = m.setID
	}
	fmt.Fprintf(&b, "Set: %s\nCurrent stage: %s\n\n", name, m.currentStage)
	if len(m.itemStatus) == 0 {
		b.WriteString("Waiting for items to start…\n")
	}
	for itemName, status := range m.itemStatus {
		fmt.Fprintf(&b, "  %-20s %-10s p95=%.0fms\n", itemName, status, m.itemP95[itemName])
	}
	if m.finalRun != nil {
		mark := "✓ all thresholds passed"
		if !m.finalRun.AllThresholdsPassed {
			mark = "✗ thresholds failed"
		}
		fmt.Fprintf(&b, "\nFinal: %s — %s. Press q to go back.\n", m.finalRun.Status, mark)
	}
	if m.err != nil {
		fmt.Fprintf(&b, "\nError: %v\n", m.err)
	}
	return b.String()
}
