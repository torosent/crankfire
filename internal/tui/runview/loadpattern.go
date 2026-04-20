package runview

import (
	"fmt"
	"strings"
	"time"

	"github.com/torosent/crankfire/internal/tui/widgets"
)

// PatternStep describes one compiled segment of a load pattern.
type PatternStep struct {
	Label    string
	Duration time.Duration
	Start    time.Duration
}

// LoadPattern is the read-only descriptor passed via runview.Options.
type LoadPattern struct {
	Name  string
	Total time.Duration
	Steps []PatternStep
}

// RenderLoadPatternStrip returns a multi-line description of a load pattern's
// progress: a progress bar with elapsed/total, followed by a step strip with
// the active step highlighted. Returns "" if lp is nil.
func RenderLoadPatternStrip(lp *LoadPattern, elapsed time.Duration, barWidth int) string {
	if lp == nil {
		return ""
	}
	pct := 0.0
	if lp.Total > 0 {
		pct = float64(elapsed) / float64(lp.Total)
		if pct > 1 {
			pct = 1
		}
	}
	bar := widgets.Progress(barWidth, pct)
	totalStr := formatPatternDuration(lp.Total)
	if lp.Total <= 0 {
		totalStr = "?"
	}
	header := fmt.Sprintf("Load Pattern: %s\n%s %.0f%% | %s / %s",
		lp.Name, bar, pct*100,
		formatPatternDuration(elapsed.Round(time.Second)), totalStr)

	parts := make([]string, 0, len(lp.Steps))
	for _, s := range lp.Steps {
		label := s.Label + "×" + formatPatternDuration(s.Duration)
		switch {
		case elapsed >= s.Start && elapsed < s.Start+s.Duration:
			parts = append(parts, "► "+label)
		case elapsed >= s.Start+s.Duration:
			parts = append(parts, "✓ "+label)
		default:
			parts = append(parts, "· "+label)
		}
	}
	return header + "\n" + strings.Join(parts, "  ")
}

// formatPatternDuration renders a duration trimming "0s" / "0m" tails after
// a unit letter — "1m0s" → "1m", "2h0m0s" → "2h", "20s" stays as "20s".
func formatPatternDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	s := d.String()
	for _, suf := range []string{"0s", "0m"} {
		if len(s) >= 3 && strings.HasSuffix(s, suf) {
			prev := s[len(s)-3]
			if prev >= 'a' && prev <= 'z' {
				s = s[:len(s)-2]
			}
		}
	}
	if s == "" {
		return "0s"
	}
	return s
}
