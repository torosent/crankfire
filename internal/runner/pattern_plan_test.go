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
	if plan.totalDuration() != 10*time.Second {
		t.Fatalf("duration = %s", plan.totalDuration())
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
	if plan.maxBurst() != 500 {
		t.Fatalf("max burst = %d", plan.maxBurst())
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
