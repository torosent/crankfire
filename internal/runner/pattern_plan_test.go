package runner

import (
	"testing"
	"time"
)

func TestCompilePatternPlanRamp(t *testing.T) {
	plan := compilePatternPlan([]LoadPattern{
		{
			Type:     LoadPatternTypeRamp,
			FromRPS:  10,
			ToRPS:    110,
			Duration: 10 * time.Second,
		},
	})
	if plan == nil {
		t.Fatalf("expected plan")
	}
	rate, ok := plan.rateAt(5 * time.Second)
	if !ok {
		t.Fatalf("rateAt returned false")
	}
	if rate < 60 || rate > 61 {
		t.Fatalf("unexpected ramp rate: %f", rate)
	}
}

func TestCompilePatternPlanStepAndSpike(t *testing.T) {
	plan := compilePatternPlan([]LoadPattern{
		{
			Type: LoadPatternTypeStep,
			Steps: []LoadStep{
				{RPS: 50, Duration: time.Second},
				{RPS: 100, Duration: 2 * time.Second},
			},
		},
		{
			Type:     LoadPatternTypeSpike,
			RPS:      500,
			Duration: 500 * time.Millisecond,
		},
	})
	if plan == nil {
		t.Fatalf("expected plan")
	}
	rate, ok := plan.rateAt(1500 * time.Millisecond)
	if !ok {
		t.Fatalf("rateAt false")
	}
	if rate != 100 {
		t.Fatalf("expected 100, got %f", rate)
	}
	rate, ok = plan.rateAt(3200 * time.Millisecond)
	if !ok {
		t.Fatalf("rateAt false for spike")
	}
	if rate != 500 {
		t.Fatalf("expected spike rate 500, got %f", rate)
	}
}

func TestPlanRateAtAfterEnd(t *testing.T) {
	plan := compilePatternPlan([]LoadPattern{{
		Type:     LoadPatternTypeSpike,
		RPS:      100,
		Duration: time.Second,
	}})
	if plan == nil {
		t.Fatalf("plan nil")
	}
	if _, ok := plan.rateAt(2 * time.Second); ok {
		t.Fatalf("expected no rate after end")
	}
}

func TestPlanRateNeverBelowOne(t *testing.T) {
	plan := compilePatternPlan([]LoadPattern{
		{Type: LoadPatternTypeRamp, FromRPS: 0, ToRPS: 5, Duration: 3 * time.Second},
		{Type: LoadPatternTypeStep, Steps: []LoadStep{{RPS: 0, Duration: time.Second}}},
	})
	for _, at := range []time.Duration{0, 100 * time.Millisecond, 3500 * time.Millisecond} {
		if rate, ok := plan.rateAt(at); !ok || rate < 1 {
			t.Fatalf("rateAt(%s) = %f, %v; a rate below 1 means unlimited to the limiter", at, rate, ok)
		}
	}
}

func TestCompilePatternPlanConstant(t *testing.T) {
	plan := compilePatternPlan([]LoadPattern{{Type: LoadPatternTypeConstant, RPS: 5, Duration: 2 * time.Second}})
	if plan == nil {
		t.Fatalf("constant pattern was dropped")
	}
	if rate, ok := plan.rateAt(time.Second); !ok || rate != 5 {
		t.Fatalf("rateAt = %f, %v", rate, ok)
	}
	if _, ok := plan.rateAt(3 * time.Second); ok {
		t.Fatalf("expected the plan to end after 2s")
	}
}
