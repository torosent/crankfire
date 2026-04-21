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

func TargetBar(width int, current, target, scaleMax float64) string {
	if width <= 0 {
		return ""
	}
	if scaleMax <= 0 {
		scaleMax = 1
	}
	if target > scaleMax {
		scaleMax = target
	}
	filled := int((current / scaleMax) * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}
