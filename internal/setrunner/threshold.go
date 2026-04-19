package setrunner

import (
	"github.com/torosent/crankfire/internal/store"
)

// MetricSnapshot is the minimal stat surface threshold checks consume.
// Populated from metrics.Collector.Snapshot() inside the runner.
type MetricSnapshot struct {
	P50         float64
	P95         float64
	P99         float64
	ErrorRate   float64
	RPS         float64
	TotalErrors float64
}

func metricValue(snap MetricSnapshot, metric string) (float64, bool) {
	switch metric {
	case "p50":
		return snap.P50, true
	case "p95":
		return snap.P95, true
	case "p99":
		return snap.P99, true
	case "error_rate":
		return snap.ErrorRate, true
	case "rps":
		return snap.RPS, true
	case "total_errors":
		return snap.TotalErrors, true
	}
	return 0, false
}

func compare(actual, want float64, op string) bool {
	switch op {
	case "lt":
		return actual < want
	case "le":
		return actual <= want
	case "gt":
		return actual > want
	case "ge":
		return actual >= want
	case "eq":
		return actual == want
	}
	return false
}

// EvaluateThresholds applies thresholds against an aggregate snapshot
// and the per-item snapshots. Scope semantics:
//   - "aggregate" (or empty): applied once to the aggregate snapshot.
//   - "per_item": applied to every item snapshot — produces N results.
//   - "<item-name>": applied to that item only; fails closed if absent.
func EvaluateThresholds(ths []store.Threshold, agg MetricSnapshot, perItem map[string]MetricSnapshot) []store.ThresholdResult {
	var out []store.ThresholdResult
	for _, t := range ths {
		scope := t.Scope
		if scope == "" {
			scope = "aggregate"
		}
		switch scope {
		case "aggregate":
			out = append(out, evalOne(t, agg))
		case "per_item":
			for name, snap := range perItem {
				cp := t
				cp.Scope = name
				out = append(out, evalOne(cp, snap))
			}
		default:
			snap, ok := perItem[scope]
			if !ok {
				out = append(out, store.ThresholdResult{Threshold: t, Passed: false})
				continue
			}
			out = append(out, evalOne(t, snap))
		}
	}
	return out
}

func evalOne(t store.Threshold, snap MetricSnapshot) store.ThresholdResult {
	v, ok := metricValue(snap, t.Metric)
	if !ok {
		return store.ThresholdResult{Threshold: t, Passed: false}
	}
	return store.ThresholdResult{Threshold: t, Actual: v, Passed: compare(v, t.Value, t.Op)}
}
