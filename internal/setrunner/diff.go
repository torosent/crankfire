// internal/setrunner/diff.go
package setrunner

import (
	"github.com/torosent/crankfire/internal/store"
)

const (
	regressionP95FracDelta = 0.05
	regressionErrRateDelta = 0.005
)

type DiffRow struct {
	ItemName     string
	Stage        string
	P50DeltaMs   float64
	P95DeltaMs   float64
	P99DeltaMs   float64
	ErrRateDelta float64
	RPSDelta     float64
	APresent     bool
	BPresent     bool
}

type DiffResult struct {
	A, B           store.SetRun
	Rows           []DiffRow
	OverallVerdict string
}

func Diff(a, b store.SetRun) DiffResult {
	res := DiffResult{A: a, B: b}
	itemsA := flattenItems(a)
	itemsB := flattenItems(b)
	seen := map[string]bool{}
	var order []string
	for _, it := range a.Stages {
		for _, item := range it.Items {
			if !seen[item.Name] {
				seen[item.Name] = true
				order = append(order, item.Name)
			}
		}
	}
	for _, it := range b.Stages {
		for _, item := range it.Items {
			if !seen[item.Name] {
				seen[item.Name] = true
				order = append(order, item.Name)
			}
		}
	}
	hasRegression, hasImprovement := false, false
	for _, name := range order {
		ia, okA := itemsA[name]
		ib, okB := itemsB[name]
		row := DiffRow{ItemName: name, APresent: okA, BPresent: okB}
		if okA {
			row.Stage = ia.stage
		} else {
			row.Stage = ib.stage
		}
		if okA && okB {
			row.P50DeltaMs = ib.it.Summary.P50Ms - ia.it.Summary.P50Ms
			row.P95DeltaMs = ib.it.Summary.P95Ms - ia.it.Summary.P95Ms
			row.P99DeltaMs = ib.it.Summary.P99Ms - ia.it.Summary.P99Ms
			row.ErrRateDelta = errRate(ib.it.Summary) - errRate(ia.it.Summary)
			row.RPSDelta = rps(ib.it.Summary) - rps(ia.it.Summary)
			p95Frac := 0.0
			if ia.it.Summary.P95Ms != 0 {
				p95Frac = row.P95DeltaMs / ia.it.Summary.P95Ms
			}
			if p95Frac > regressionP95FracDelta || row.ErrRateDelta > regressionErrRateDelta {
				hasRegression = true
			}
			if p95Frac < -regressionP95FracDelta || row.ErrRateDelta < -regressionErrRateDelta {
				hasImprovement = true
			}
		}
		res.Rows = append(res.Rows, row)
	}
	switch {
	case hasRegression && hasImprovement:
		res.OverallVerdict = "mixed"
	case hasRegression:
		res.OverallVerdict = "regressed"
	case hasImprovement:
		res.OverallVerdict = "improved"
	default:
		res.OverallVerdict = "unchanged"
	}
	return res
}

type itemRef struct {
	it    store.ItemResult
	stage string
}

func flattenItems(r store.SetRun) map[string]itemRef {
	out := map[string]itemRef{}
	for _, st := range r.Stages {
		for _, it := range st.Items {
			out[it.Name] = itemRef{it: it, stage: st.Name}
		}
	}
	return out
}

func errRate(s store.RunSummary) float64 {
	if s.TotalRequests == 0 {
		return 0
	}
	return float64(s.Errors) / float64(s.TotalRequests)
}

func rps(s store.RunSummary) float64 {
	if s.DurationSec == 0 {
		return 0
	}
	return float64(s.TotalRequests) / s.DurationSec
}
