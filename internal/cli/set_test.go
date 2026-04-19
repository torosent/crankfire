package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestSetNewFromTemplate(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	ctx := context.Background()
	tpl := []byte(`template: true
name: api-baseline-{{ .Env }}
description: rate {{ .Rate }}
stages:
- name: smoke
  items:
  - name: i
    session_id: 01F8MECHZX3TBDSZ7XR9PFE7M0
`)
	if err := st.SaveTemplate(ctx, "api-baseline", tpl); err != nil {
		t.Fatal(err)
	}
	// Create the referenced session so SaveSet's downstream consistency holds.
	_ = st.SaveSession(ctx, store.Session{ID: "01F8MECHZX3TBDSZ7XR9PFE7M0", Name: "s"})

	var out, errBuf bytes.Buffer
	code := cli.RunSet(ctx, st, []string{
		"new",
		"--from-template", "api-baseline",
		"--param", "Env=prod",
		"--param", "Rate=500",
		"--name", "api-baseline-prod-500",
	}, &out, &errBuf)
	if code != cli.ExitOK {
		t.Fatalf("code=%d stderr=%s", code, errBuf.String())
	}
	sets, _ := st.ListSets(ctx)
	if len(sets) != 1 {
		t.Fatalf("want 1 set, got %d", len(sets))
	}
	if sets[0].Name != "api-baseline-prod-500" {
		t.Errorf("name = %q, want overridden", sets[0].Name)
	}
	if sets[0].Description != "rate 500" {
		t.Errorf("desc = %q", sets[0].Description)
	}
	// Stdout should print the new set ID
	if !strings.Contains(out.String(), sets[0].ID) {
		t.Errorf("stdout missing new ID:\n%s", out.String())
	}
}

func TestSetNewMissingTemplateExitsUsage(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	var out, errBuf bytes.Buffer
	code := cli.RunSet(context.Background(), st, []string{"new", "--from-template", "nope"}, &out, &errBuf)
	if code != cli.ExitUsage {
		t.Errorf("code=%d, want ExitUsage", code)
	}
}

func TestSetNewBadTemplateExitsUsage(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFS(dir)
	ctx := context.Background()
	_ = st.SaveTemplate(ctx, "bad", []byte("template: true\nname: {{ .Bad\n"))
	var out, errBuf bytes.Buffer
	code := cli.RunSet(ctx, st, []string{"new", "--from-template", "bad"}, &out, &errBuf)
	if code != cli.ExitUsage {
		t.Errorf("code=%d, want ExitUsage; stderr=%s", code, errBuf.String())
	}
}

func TestSetDiffTextOutput(t *testing.T) {
dir := t.TempDir()
st, _ := store.NewFS(dir)
ctx := context.Background()
setID := "01F8MECHZX3TBDSZ7XR9PFE7S0"
_ = st.SaveSet(ctx, store.Set{ID: setID, Name: "n", Stages: []store.Stage{{Name: "s"}}})
r1, _ := st.CreateSetRun(ctx, setID)
r1.Status = store.SetRunCompleted
_ = st.FinalizeSetRun(ctx, r1)
time.Sleep(1100 * time.Millisecond)
r2, _ := st.CreateSetRun(ctx, setID)
r2.Status = store.SetRunCompleted
_ = st.FinalizeSetRun(ctx, r2)
runs, _ := st.ListSetRuns(ctx, setID)
idA := filepath.Base(runs[0].Dir)
idB := filepath.Base(runs[1].Dir)
t.Setenv("CRANKFIRE_DATA_DIR", dir)
var out, errBuf bytes.Buffer
code := cli.RunSet(ctx, st, []string{"diff", idA, idB}, &out, &errBuf)
if code != cli.ExitOK {
t.Fatalf("code=%d stderr=%s", code, errBuf.String())
}
if !strings.Contains(out.String(), "verdict") {
t.Errorf("missing verdict:\n%s", out.String())
}
}

func TestSetDiffJSONOutput(t *testing.T) {
dir := t.TempDir()
st, _ := store.NewFS(dir)
ctx := context.Background()
setID := "01F8MECHZX3TBDSZ7XR9PFE7S1"
_ = st.SaveSet(ctx, store.Set{ID: setID, Name: "n", Stages: []store.Stage{{Name: "s"}}})
r1, _ := st.CreateSetRun(ctx, setID)
_ = st.FinalizeSetRun(ctx, r1)
time.Sleep(1100 * time.Millisecond)
r2, _ := st.CreateSetRun(ctx, setID)
_ = st.FinalizeSetRun(ctx, r2)
runs, _ := st.ListSetRuns(ctx, setID)
idA := filepath.Base(runs[0].Dir)
idB := filepath.Base(runs[1].Dir)
t.Setenv("CRANKFIRE_DATA_DIR", dir)
var out, errBuf bytes.Buffer
code := cli.RunSet(ctx, st, []string{"diff", "--json", idA, idB}, &out, &errBuf)
if code != cli.ExitOK {
t.Fatalf("code=%d stderr=%s", code, errBuf.String())
}
if !strings.Contains(out.String(), `"OverallVerdict"`) {
t.Errorf("expected JSON, got:\n%s", out.String())
}
}

func TestSetDiffHTMLOutput(t *testing.T) {
dir := t.TempDir()
st, _ := store.NewFS(dir)
ctx := context.Background()
setID := "01F8MECHZX3TBDSZ7XR9PFE7S2"
_ = st.SaveSet(ctx, store.Set{ID: setID, Name: "n", Stages: []store.Stage{{Name: "s"}}})
r1, _ := st.CreateSetRun(ctx, setID)
_ = st.FinalizeSetRun(ctx, r1)
time.Sleep(1100 * time.Millisecond)
r2, _ := st.CreateSetRun(ctx, setID)
_ = st.FinalizeSetRun(ctx, r2)
runs, _ := st.ListSetRuns(ctx, setID)
idA := filepath.Base(runs[0].Dir)
idB := filepath.Base(runs[1].Dir)
t.Setenv("CRANKFIRE_DATA_DIR", dir)
htmlPath := filepath.Join(dir, "diff.html")
var out, errBuf bytes.Buffer
code := cli.RunSet(ctx, st, []string{"diff", "--html", htmlPath, idA, idB}, &out, &errBuf)
if code != cli.ExitOK {
t.Fatalf("code=%d stderr=%s", code, errBuf.String())
}
body, err := os.ReadFile(htmlPath)
if err != nil {
t.Fatalf("read html: %v", err)
}
if !strings.Contains(string(body), "<html") {
t.Errorf("not html: %s", body)
}
}

func TestSetDiffAmbiguousIDExitsUsage(t *testing.T) {
dir := t.TempDir()
st, _ := store.NewFS(dir)
ctx := context.Background()
for _, id := range []string{"01F8MECHZX3TBDSZ7XR9PFE7S3", "01F8MECHZX3TBDSZ7XR9PFE7S4"} {
_ = st.SaveSet(ctx, store.Set{ID: id, Name: "n", Stages: []store.Stage{{Name: "s"}}})
}
ts := time.Now().UTC().Format("2006-01-02T15-04-05.000Z")
for _, setID := range []string{"01F8MECHZX3TBDSZ7XR9PFE7S3", "01F8MECHZX3TBDSZ7XR9PFE7S4"} {
d := filepath.Join(dir, "runs", "sets", setID, ts)
_ = os.MkdirAll(d, 0o755)
_ = os.WriteFile(filepath.Join(d, "set-run.json"), []byte(`{"status":"completed","stages":[]}`), 0o644)
}
t.Setenv("CRANKFIRE_DATA_DIR", dir)
var out, errBuf bytes.Buffer
code := cli.RunSet(ctx, st, []string{"diff", ts, ts}, &out, &errBuf)
if code != cli.ExitUsage {
t.Errorf("code=%d, want ExitUsage; stderr=%s", code, errBuf.String())
}
if !strings.Contains(errBuf.String(), "ambiguous") {
t.Errorf("stderr should mention ambiguity: %q", errBuf.String())
}
}
