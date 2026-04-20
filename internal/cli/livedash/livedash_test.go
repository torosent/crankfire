package livedash_test

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/cli/livedash"
	"github.com/torosent/crankfire/internal/metrics"
)

func TestBuildSnapshotPopulatesAllFields(t *testing.T) {
	c := metrics.NewCollector()
	c.Start()
	c.RecordRequest(50*time.Millisecond, nil, &metrics.RequestMetadata{Endpoint: "GET /", Protocol: "http", StatusCode: "200"})
	c.RecordRequest(80*time.Millisecond, nil, &metrics.RequestMetadata{Endpoint: "GET /", Protocol: "http", StatusCode: "200"})
	c.RecordRequest(20*time.Millisecond, fmt.Errorf("status 500"), &metrics.RequestMetadata{Endpoint: "GET /", Protocol: "http", StatusCode: "500"})
	c.Snapshot()

	snap := livedash.BuildSnapshot(c, 1*time.Second)
	if snap.Stats == nil {
		t.Fatalf("expected Stats populated")
	}
	if snap.Stats.Total != 3 {
		t.Errorf("Stats.Total = %d, want 3", snap.Stats.Total)
	}
	if snap.Elapsed != 1*time.Second {
		t.Errorf("Elapsed = %s, want 1s", snap.Elapsed)
	}
	if got := snap.StatusBuckets["http"]["500"]; got != 1 {
		t.Errorf("StatusBuckets http/500 = %d, want 1", got)
	}
	if len(snap.Endpoints) == 0 {
		t.Errorf("expected at least one endpoint row")
	}
	if snap.Endpoints[0].Path != "/" || snap.Endpoints[0].Method != "GET" {
		t.Errorf("endpoint = %+v, want method=GET path=/", snap.Endpoints[0])
	}
}

func TestRenderViaRunviewIncludesHeaderAndStatus(t *testing.T) {
	c := metrics.NewCollector()
	c.Start()
	c.RecordRequest(10*time.Millisecond, fmt.Errorf("status 500"), &metrics.RequestMetadata{Endpoint: "GET /", Protocol: "http", StatusCode: "500"})
	c.Snapshot()

	d := livedash.New(c, livedash.Opts{
		Title:  "T",
		Header: []string{"Target: https://example.com"},
		Total:  10,
	}, func() {})

	snap := livedash.BuildSnapshot(c, 500*time.Millisecond)
	model := d.ModelForTest()
	updated, _ := model.Update(snap)
	out := updated.View()
	if !strings.Contains(out, "Target: https://example.com") {
		t.Errorf("missing header: %s", out)
	}
	if !strings.Contains(out, "HTTP 500 1") {
		t.Errorf("missing failing status row: %s", out)
	}
}

func TestKeyQInvokesShutdownAndQuits(t *testing.T) {
	c := metrics.NewCollector()
	c.Start()
	var called int32
	d := livedash.New(c, livedash.Opts{Title: "x"}, func() {
		atomic.AddInt32(&called, 1)
	})
	if _, cmd := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}); cmd == nil {
		t.Fatalf("expected tea.Quit cmd, got nil")
	}
	if atomic.LoadInt32(&called) != 1 {
		t.Errorf("shutdown called %d times, want 1", atomic.LoadInt32(&called))
	}
}

func TestStopReturnsFinalStats(t *testing.T) {
	c := metrics.NewCollector()
	c.Start()
	c.RecordRequest(10*time.Millisecond, nil, &metrics.RequestMetadata{Endpoint: "GET /", Protocol: "http", StatusCode: "200"})
	c.Snapshot()

	d := livedash.New(c, livedash.Opts{Title: "x", Interval: 10 * time.Millisecond}, func() {})
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(40 * time.Millisecond)
	stats := d.Stop()
	if stats.Total < 1 {
		t.Errorf("expected stats.Total >= 1, got %d", stats.Total)
	}
}
