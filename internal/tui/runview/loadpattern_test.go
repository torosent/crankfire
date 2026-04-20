package runview_test

import (
	"strings"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/tui/runview"
)

func TestRenderLoadPatternStripHighlightsCurrentStep(t *testing.T) {
	lp := &runview.LoadPattern{
		Name:  "ramp+steady",
		Total: 30 * time.Second,
		Steps: []runview.PatternStep{
			{Label: "10 RPS", Duration: 10 * time.Second, Start: 0},
			{Label: "10→100 RPS", Duration: 10 * time.Second, Start: 10 * time.Second},
			{Label: "100 RPS", Duration: 10 * time.Second, Start: 20 * time.Second},
		},
	}
	out := runview.RenderLoadPatternStrip(lp, 15*time.Second, 30)
	if !strings.Contains(out, "ramp+steady") {
		t.Errorf("missing pattern name in: %s", out)
	}
	if !strings.Contains(out, "✓ 10 RPS") {
		t.Errorf("expected first step marked done, got: %s", out)
	}
	if !strings.Contains(out, "► 10→100 RPS") {
		t.Errorf("expected current step marked active, got: %s", out)
	}
	if !strings.Contains(out, "· 100 RPS") {
		t.Errorf("expected future step marked pending, got: %s", out)
	}
	if !strings.Contains(out, "50%") {
		t.Errorf("expected progress 50%%, got: %s", out)
	}
}

func TestRenderLoadPatternStripNilReturnsEmpty(t *testing.T) {
	if got := runview.RenderLoadPatternStrip(nil, 0, 30); got != "" {
		t.Errorf("expected empty string for nil pattern, got %q", got)
	}
}
