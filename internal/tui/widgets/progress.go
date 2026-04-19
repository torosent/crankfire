// internal/tui/widgets/progress.go
package widgets

import "strings"

func Progress(width int, frac float64) string {
	if width <= 0 {
		return ""
	}
	if frac < 0 {
		frac = 0
	} else if frac > 1 {
		frac = 1
	}
	filled := int(float64(width) * frac)
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}
