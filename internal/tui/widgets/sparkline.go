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
