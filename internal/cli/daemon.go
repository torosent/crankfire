package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/gofrs/flock"

	"github.com/torosent/crankfire/internal/scheduler"
	"github.com/torosent/crankfire/internal/setrunner"
	"github.com/torosent/crankfire/internal/store"
)

// daemonDrainTimeout is how long the daemon waits for in-flight runs to
// complete after receiving SIGINT/SIGTERM before forcing exit.
const daemonDrainTimeout = 30 * time.Second

// RunDaemon is the entry point for `crankfire daemon`. Blocks until ctx
// is cancelled (typically by SIGINT/SIGTERM via signal handler installed
// here, or by parent caller in tests).
func RunDaemon(ctx context.Context, st store.Store, dataDir string, args []string, stdout, stderr io.Writer) int {
	lockPath := filepath.Join(dataDir, "daemon.lock")
	lk := flock.New(lockPath)
	got, err := lk.TryLock()
	if err != nil {
		fmt.Fprintf(stderr, "lock %s: %v\n", lockPath, err)
		return ExitUsage
	}
	if !got {
		fmt.Fprintf(stderr, "another daemon is already running (see %s)\n", lockPath)
		return ExitUsage
	}
	defer lk.Unlock()

	logger := newJSONLogger(stdout)

	// Install signal handlers that cancel the root context.
	rootCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	sigCh := make(chan os.Signal, 4)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigCh)

	// Build the runner + scheduler.
	runner := setrunner.New(st, NewSetBuilder())
	var inflight sync.WaitGroup
	mgr := scheduler.New(func(fireCtx context.Context, setID string) {
		inflight.Add(1)
		defer inflight.Done()
		logger.log("info", setID, "fire-start", "")
		_, err := runner.Run(fireCtx, setID, nil)
		if err != nil {
			logger.log("error", setID, "fire-failed", err.Error())
			return
		}
		logger.log("info", setID, "fire-completed", "")
	})

	if err := mgr.Reload(rootCtx, st); err != nil {
		logger.log("error", "", "reload-failed", err.Error())
		return ExitRunnerError
	}
	logger.log("info", "", "started", fmt.Sprintf("%d entries", mgr.EntryCount()))

	// Goroutine: dispatch signals.
	go func() {
		for {
			select {
			case <-rootCtx.Done():
				return
			case s := <-sigCh:
				switch s {
				case syscall.SIGINT, syscall.SIGTERM:
					logger.log("info", "", "signal", s.String())
					cancel()
					return
				case syscall.SIGHUP:
					if err := mgr.Reload(rootCtx, st); err != nil {
						logger.log("error", "", "reload-failed", err.Error())
					} else {
						logger.log("info", "", "reload-ok", fmt.Sprintf("%d entries", mgr.EntryCount()))
					}
				}
			}
		}
	}()

	// Run the cron — blocks until rootCtx is done.
	mgr.Run(rootCtx)

	// Drain in-flight runs.
	drained := make(chan struct{})
	go func() {
		inflight.Wait()
		close(drained)
	}()
	select {
	case <-drained:
		logger.log("info", "", "drained", "")
	case <-time.After(daemonDrainTimeout):
		logger.log("warn", "", "drain-timeout", "")
	}
	logger.log("info", "", "stopped", "")
	return ExitOK
}

type jsonLogger struct {
	mu  sync.Mutex
	out io.Writer
}

func newJSONLogger(w io.Writer) *jsonLogger { return &jsonLogger{out: w} }

func (l *jsonLogger) log(level, setID, event, msg string) {
	rec := map[string]string{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"level": level,
		"event": event,
	}
	if setID != "" {
		rec["setID"] = setID
	}
	if msg != "" {
		rec["msg"] = msg
	}
	data, _ := json.Marshal(rec)
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.out.Write(append(data, '\n'))
}
