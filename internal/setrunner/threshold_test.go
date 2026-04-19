package setrunner_test

import (
	"testing"

	"github.com/torosent/crankfire/internal/setrunner"
	"github.com/torosent/crankfire/internal/store"
)

func TestEvaluateThresholdsAggregate(t *testing.T) {
	snap := setrunner.MetricSnapshot{P50: 100, P95: 400, P99: 700, ErrorRate: 0.005, RPS: 200, TotalErrors: 5}
	got := setrunner.EvaluateThresholds(
		[]store.Threshold{
			{Metric: "p95", Op: "lt", Value: 500, Scope: "aggregate"},
			{Metric: "error_rate", Op: "lt", Value: 0.01, Scope: "aggregate"},
			{Metric: "p99", Op: "lt", Value: 600, Scope: "aggregate"}, // fail
		},
		snap, nil,
	)
	if len(got) != 3 {
		t.Fatalf("len: %d", len(got))
	}
	if !got[0].Passed || !got[1].Passed || got[2].Passed {
		t.Errorf("results: %+v", got)
	}
}

func TestEvaluateThresholdsPerItem(t *testing.T) {
	per := map[string]setrunner.MetricSnapshot{
		"login":  {P95: 200},
		"search": {P95: 600}, // fails
	}
	got := setrunner.EvaluateThresholds(
		[]store.Threshold{{Metric: "p95", Op: "lt", Value: 500, Scope: "per_item"}},
		setrunner.MetricSnapshot{}, per,
	)
	// per_item explodes into one result per item
	if len(got) != 2 {
		t.Fatalf("len: %d", len(got))
	}
	pass, fail := 0, 0
	for _, r := range got {
		if r.Passed {
			pass++
		} else {
			fail++
		}
	}
	if pass != 1 || fail != 1 {
		t.Errorf("pass=%d fail=%d", pass, fail)
	}
}

func TestEvaluateThresholdsSpecificItem(t *testing.T) {
	per := map[string]setrunner.MetricSnapshot{"login": {P95: 600}}
	got := setrunner.EvaluateThresholds(
		[]store.Threshold{{Metric: "p95", Op: "lt", Value: 500, Scope: "login"}},
		setrunner.MetricSnapshot{}, per,
	)
	if len(got) != 1 || got[0].Passed {
		t.Errorf("expected 1 fail: %+v", got)
	}
}

func TestEvaluateThresholdsUnknownItemScope(t *testing.T) {
	got := setrunner.EvaluateThresholds(
		[]store.Threshold{{Metric: "p95", Op: "lt", Value: 1, Scope: "ghost"}},
		setrunner.MetricSnapshot{}, map[string]setrunner.MetricSnapshot{"login": {}},
	)
	if len(got) != 1 || got[0].Passed {
		t.Errorf("unknown scope must fail closed: %+v", got)
	}
}
