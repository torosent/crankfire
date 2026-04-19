// internal/tui/widgets/tables.go
package widgets

import (
	"fmt"
	"sort"
	"strings"
)

func PercentileTable(p map[string]float64) string {
	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%-5s %8.1f\n", k, p[k])
	}
	return b.String()
}

type EndpointRow struct {
	Method, Path string
	Count        int64
	P95Ms        float64
	ErrPct       float64
}

func EndpointTable(rows []EndpointRow, max int) string {
	if max > 0 && len(rows) > max {
		rows = rows[:max]
	}
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%-6s %-20s %8d  p95 %5.0fms  err %4.1f%%\n",
			r.Method, r.Path, r.Count, r.P95Ms, r.ErrPct)
	}
	return b.String()
}
