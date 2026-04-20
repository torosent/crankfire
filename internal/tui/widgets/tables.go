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
	Method   string
	Path     string
	Count    int64
	SharePct float64
	RPS      float64
	P95Ms    float64
	P99Ms    float64
	ErrPct   float64
	Errors   int64
}

func EndpointTable(rows []EndpointRow, max int) string {
	if max > 0 && len(rows) > max {
		rows = rows[:max]
	}
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b,
			"%-6s %-24s %8d  %5.1f%%  rps %6.1f  p95 %5.0fms  p99 %5.0fms  err %d (%4.1f%%)\n",
			r.Method, r.Path, r.Count, r.SharePct, r.RPS, r.P95Ms, r.P99Ms, r.Errors, r.ErrPct,
		)
	}
	return b.String()
}
