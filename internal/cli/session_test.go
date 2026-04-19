package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/torosent/crankfire/internal/cli"
	"github.com/torosent/crankfire/internal/store"
)

func newTempStore(t *testing.T) store.Store {
	t.Helper()
	st, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}
	return st
}

func TestSessionListFiltersByTag(t *testing.T) {
	st := newTempStore(t)
	ctx := context.Background()
	for _, sess := range []store.Session{
		{Name: "a", Tags: []string{"prod"}},
		{Name: "b", Tags: []string{"staging"}},
		{Name: "c", Tags: []string{"prod", "smoke"}},
	} {
		if err := st.SaveSession(ctx, sess); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	var out, errBuf bytes.Buffer
	code := cli.RunSession(ctx, st, []string{"list", "--tag", "prod"}, &out, &errBuf)
	if code != cli.ExitOK {
		t.Fatalf("code=%d stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "  a  ") || !strings.Contains(out.String(), "  c  ") || strings.Contains(out.String(), "  b  ") {
		t.Errorf("unexpected output:\n%s", out.String())
	}
}

func TestSessionListAndOfOrTags(t *testing.T) {
	st := newTempStore(t)
	ctx := context.Background()
	for _, sess := range []store.Session{
		{Name: "a", Tags: []string{"prod", "smoke"}},
		{Name: "b", Tags: []string{"prod", "regression"}},
		{Name: "c", Tags: []string{"prod"}},
	} {
		_ = st.SaveSession(ctx, sess)
	}
	var out, errBuf bytes.Buffer
	// Two --tag flags = AND of two groups; comma inside = OR
	code := cli.RunSession(ctx, st, []string{"list", "--tag", "prod", "--tag", "smoke,regression"}, &out, &errBuf)
	if code != cli.ExitOK {
		t.Fatalf("code=%d", code)
	}
	got := out.String()
	if !strings.Contains(got, "  a  ") || !strings.Contains(got, "  b  ") || strings.Contains(got, "  c  ") {
		t.Errorf("unexpected output:\n%s", got)
	}
}

func TestSessionEditAddRemoveTag(t *testing.T) {
	st := newTempStore(t)
	ctx := context.Background()
	if err := st.SaveSession(ctx, store.Session{ID: "01F8MECHZX3TBDSZ7XR9PFE7M0", Name: "a", Tags: []string{"old"}}); err != nil {
		t.Fatalf("save: %v", err)
	}
	var out, errBuf bytes.Buffer
	code := cli.RunSession(ctx, st, []string{"edit", "01F8MECHZX3TBDSZ7XR9PFE7M0", "--add-tag", "prod", "--add-tag", "smoke", "--remove-tag", "old"}, &out, &errBuf)
	if code != cli.ExitOK {
		t.Fatalf("code=%d stderr=%s", code, errBuf.String())
	}
	got, err := st.GetSession(ctx, "01F8MECHZX3TBDSZ7XR9PFE7M0")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	want := []string{"prod", "smoke"}
	if len(got.Tags) != 2 || got.Tags[0] != want[0] || got.Tags[1] != want[1] {
		t.Errorf("Tags = %v, want %v", got.Tags, want)
	}
}

func TestSessionEditInvalidTagExitsUsage(t *testing.T) {
	st := newTempStore(t)
	ctx := context.Background()
	_ = st.SaveSession(ctx, store.Session{ID: "01F8MECHZX3TBDSZ7XR9PFE7M0", Name: "a"})
	var out, errBuf bytes.Buffer
	code := cli.RunSession(ctx, st, []string{"edit", "01F8MECHZX3TBDSZ7XR9PFE7M0", "--add-tag", "bad tag!"}, &out, &errBuf)
	if code != cli.ExitUsage {
		t.Errorf("code=%d, want ExitUsage", code)
	}
}
