package setrunner

import (
	"github.com/torosent/crankfire/internal/store"
)

type CompareRow struct {
	Metric        string             // p50|p95|p99|error_rate|rps|total_errors
	LowerIsBetter bool
	Values        map[string]float64 // item name -> value
	Winner        string             // item name or "" if no eligible
}

type Comparison struct {
	Items []string     // item names in input order
	Rows  []CompareRow
}

var compareMetrics = []struct {
	key           string
	lowerIsBetter bool
	get           func(store.RunSummary) float64
}{
	{"p50", true, func(s store.RunSummary) float64 { return s.P50Ms }},
	{"p95", true, func(s store.RunSummary) float64 { return s.P95Ms }},
	{"p99", true, func(s store.RunSummary) float64 { return s.P99Ms }},
	{"error_rate", true, func(s store.RunSummary) float64 {
		if s.TotalRequests == 0 {
			return 0
		}
		return float64(s.Errors) / float64(s.TotalRequests)
	}},
	{"total_errors", true, func(s store.RunSummary) float64 { return float64(s.Errors) }},
	{"rps", false, func(s store.RunSummary) float64 {
		if s.DurationSec == 0 {
			return 0
		}
		return float64(s.TotalRequests) / s.DurationSec
	}},
}

func Compare(items []store.ItemResult) Comparison {
	c := Comparison{}
	if len(items) == 0 {
		return c
	}
	for _, it := range items {
		c.Items = append(c.Items, it.Name)
	}
	for _, m := range compareMetrics {
		row := CompareRow{Metric: m.key, LowerIsBetter: m.lowerIsBetter, Values: map[string]float64{}}
		var bestName string
		var bestVal float64
		first := true
		for _, it := range items {
			v := m.get(it.Summary)
			row.Values[it.Name] = v
			if it.Status != store.RunStatusCompleted {
				continue
			}
			if first {
				bestName, bestVal, first = it.Name, v, false
				continue
			}
			if (m.lowerIsBetter && v < bestVal) || (!m.lowerIsBetter && v > bestVal) {
				bestName, bestVal = it.Name, v
			}
		}
		row.Winner = bestName
		c.Rows = append(c.Rows, row)
	}
	return c
}
