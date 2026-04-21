package widgets

import "strings"

// Marker represents a horizontal reference line drawn at a specific value
// within a TrendChart. Markers are normalized against the chart's data scale
// and only rendered if they fall within the visible vertical range.
type Marker struct {
	Label string
	Value float64
	Rune  rune
}

// TrendChart renders a left-to-right trend visualization with optional
// horizontal marker lines. Samples are drawn as vertical bars (█) with
// height proportional to their value. When sample count exceeds width,
// only the most recent samples are displayed (right-aligned). Markers
// appear as horizontal lines using the specified rune.
func TrendChart(samples []float64, width, height int, markers []Marker) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	visible := samples
	if len(visible) > width {
		visible = visible[len(visible)-width:]
	}
	levels := normalizeSparklineLevels(visible, height)
	lines := make([][]rune, height)
	for row := range lines {
		lines[row] = []rune(strings.Repeat(" ", width))
	}
	for col, level := range levels {
		for row := height - 1; row >= height-level; row-- {
			lines[row][col] = '█'
		}
	}
	// Markers are normalized against the same data scale as the chart samples.
	// Markers whose normalized row falls outside [0, height) are skipped.
	if min, max, ok := sampleRange(visible); ok {
		for _, marker := range markers {
			level, ok := normalizeSparklineLevel(marker.Value, min, max, height)
			if !ok {
				continue
			}
			row := height - level
			if row >= 0 && row < height {
				for col := 0; col < width; col++ {
					if lines[row][col] == ' ' {
						lines[row][col] = marker.Rune
					}
				}
			}
		}
	}
	out := make([]string, 0, height)
	for _, line := range lines {
		out = append(out, string(line))
	}
	return strings.Join(out, "\n")
}
