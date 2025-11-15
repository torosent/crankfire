package runner

import (
	"context"
	"testing"
	"time"
)

func TestPoissonArrivalNextDelayUsesSampler(t *testing.T) {
	ctrl := &poissonArrival{sample: func() float64 { return 1 }}
	ctrl.SetRate(200)
	delay := ctrl.nextDelay()
	expected := time.Second / 200
	if delay != expected {
		t.Fatalf("expected delay %s, got %s", expected, delay)
	}
}

func TestPoissonArrivalWaitCancelledContext(t *testing.T) {
	ctrl := &poissonArrival{sample: func() float64 { return 1 }}
	ctrl.SetRate(0.000001)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ctrl.Wait(ctx); err == nil {
		t.Fatalf("expected context error when cancelled")
	}
}
