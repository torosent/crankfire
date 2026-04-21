// internal/tui/widgets/sparkline.go
package widgets

import "strings"

var sparkRunes = []rune("▁▂▃▄▅▆▇█")

func Sparkline(samples []float64, width int) string {
	if width <= 0 {
		return ""
	}
	if len(samples) == 0 {
		return strings.Repeat(" ", width)
	}
	if len(samples) > width {
		samples = samples[len(samples)-width:]
	}
	var min, max float64 = samples[0], samples[0]
	for _, v := range samples {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	rng := max - min
	var b strings.Builder
	for i := 0; i < width-len(samples); i++ {
		b.WriteByte(' ')
	}
	for _, v := range samples {
		idx := 0
		if rng > 0 {
			idx = int(((v - min) / rng) * float64(len(sparkRunes)-1))
		}
		b.WriteRune(sparkRunes[idx])
	}
	return b.String()
}

// SparklineArea renders a multi-line sparkline/area chart with the requested
// width and height. The final line retains the single-line sparkline output,
// while the rows above fill the chart vertically with block cells.
func SparklineArea(samples []float64, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if height == 1 {
		return Sparkline(samples, width)
	}

	visible := samples
	if len(visible) > width {
		visible = visible[len(visible)-width:]
	}
	levels := normalizeSparklineLevels(visible, height-1)
	leftPad := width - len(visible)

	lines := make([]string, 0, height)
	for row := height - 2; row >= 0; row-- {
		var b strings.Builder
		for i := 0; i < leftPad; i++ {
			b.WriteByte(' ')
		}
		for _, level := range levels {
			if level > row {
				b.WriteRune('█')
				continue
			}
			b.WriteByte(' ')
		}
		lines = append(lines, b.String())
	}
	lines = append(lines, Sparkline(samples, width))
	return strings.Join(lines, "\n")
}

func normalizeSparklineLevels(samples []float64, height int) []int {
	levels := make([]int, len(samples))
	if len(samples) == 0 || height <= 0 {
		return levels
	}

	min, max, ok := sampleRange(samples)
	if !ok {
		return levels
	}
	for i, v := range samples {
		level, _ := normalizeSparklineLevel(v, min, max, height)
		levels[i] = level
	}
	return levels
}

func sampleRange(samples []float64) (float64, float64, bool) {
	if len(samples) == 0 {
		return 0, 0, false
	}
	min, max := samples[0], samples[0]
	for _, v := range samples[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max, true
}

func normalizeSparklineLevel(value, min, max float64, height int) (int, bool) {
	if height <= 0 {
		return 0, false
	}
	if max == min {
		if value != min {
			return 0, false
		}
		return 1, true
	}
	if value < min || value > max {
		return 0, false
	}
	level := int(((value - min) / (max - min)) * float64(height))
	if level < 1 {
		level = 1
	}
	if level > height {
		level = height
	}
	return level, true
}
