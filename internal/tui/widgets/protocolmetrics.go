package widgets

import (
	"fmt"
	"sort"
	"strings"
)

// ProtocolMetricsBlock renders per-protocol metrics, alphabetically by
// protocol, limited to perProtocol metrics each. Returns "" if input empty.
func ProtocolMetricsBlock(metrics map[string]map[string]interface{}, perProtocol int) string {
	if len(metrics) == 0 {
		return ""
	}
	protocols := make([]string, 0, len(metrics))
	for p := range metrics {
		protocols = append(protocols, p)
	}
	sort.Strings(protocols)

	var b strings.Builder
	for _, p := range protocols {
		fmt.Fprintf(&b, "%s:\n", p)
		keys := make([]string, 0, len(metrics[p]))
		for k := range metrics[p] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		count := 0
		for _, k := range keys {
			val := metrics[p][k]
			// Skip non-numeric values
			switch val.(type) {
			case string:
				continue
			}
			if perProtocol > 0 && count >= perProtocol {
				break
			}
			fmt.Fprintf(&b, "  %s: %s\n", k, formatMetricValue(val))
			count++
		}
	}
	return b.String()
}

func formatMetricValue(v interface{}) string {
	switch x := v.(type) {
	case int:
		return fmt.Sprintf("%d", x)
	case int64:
		return fmt.Sprintf("%d", x)
	case float64:
		if x > 1000 {
			return fmt.Sprintf("%.0f", x)
		}
		return fmt.Sprintf("%.2f", x)
	case string:
		return x
	default:
		return fmt.Sprintf("%v", v)
	}
}
