package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/torosent/crankfire/internal/cli"
	"github.com/torosent/crankfire/internal/store"
)

func TestSetListEmpty(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	var stdout, stderr bytes.Buffer
	code := cli.RunSet(context.Background(), st, []string{"list"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit: %d stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "No sets") {
		t.Errorf("stdout: %s", stdout.String())
	}
}

func TestSetUnknownSubcommand(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	var out, err bytes.Buffer
	code := cli.RunSet(context.Background(), st, []string{"frobnicate"}, &out, &err)
	if code != 1 {
		t.Errorf("exit: got %d want 1", code)
	}
	if !strings.Contains(err.String(), "unknown subcommand") {
		t.Errorf("stderr: %s", err.String())
	}
}

func TestSetShowMissing(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	var out, errb bytes.Buffer
	code := cli.RunSet(context.Background(), st, []string{"show", "01HXNOPE"}, &out, &errb)
	if code != 1 {
		t.Errorf("exit: %d", code)
	}
}
