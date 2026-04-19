package cli_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/flock"

	"github.com/torosent/crankfire/internal/cli"
	"github.com/torosent/crankfire/internal/store"
)

func TestDaemonLockContentionExits(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	// Pre-acquire the lock from another flock to simulate a running daemon.
	holder := flock.New(filepath.Join(dir, "daemon.lock"))
	got, err := holder.TryLock()
	if err != nil || !got {
		t.Fatalf("pre-lock failed: %v %v", got, err)
	}
	defer holder.Unlock()

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	code := cli.RunDaemon(ctx, st, dir, []string{}, stdout, stderr)
	if code != cli.ExitUsage {
		t.Errorf("code = %d, want ExitUsage; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "daemon.lock") {
		t.Errorf("stderr should mention lock contention: %q", stderr.String())
	}
}

func TestDaemonStartsAndExitsOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	code := cli.RunDaemon(ctx, st, dir, []string{}, stdout, stderr)
	if code != cli.ExitOK {
		t.Errorf("code = %d, want ExitOK; stderr=%s", code, stderr.String())
	}
	// Lock should be released — re-acquiring it should succeed.
	holder := flock.New(filepath.Join(dir, "daemon.lock"))
	got, err := holder.TryLock()
	if err != nil || !got {
		t.Errorf("lock not released after daemon exit: %v %v", got, err)
	}
	holder.Unlock()
	// Should have logged a "started" line.
	if !strings.Contains(stdout.String(), `"event":"started"`) {
		t.Errorf("missing started log: %s", stdout.String())
	}
	_ = fmt.Stringer(nil)
	_ = os.Stdin
}
