package runner

import (
	"testing"

	"golang.org/x/time/rate"
)

func TestOptionsNormalize(t *testing.T) {
	tests := []struct {
		name     string
		input    Options
		validate func(*testing.T, Options)
	}{
		{
			name:  "defaults",
			input: Options{},
			validate: func(t *testing.T, o Options) {
				if o.Concurrency != 1 {
					t.Errorf("Concurrency = %d, want 1", o.Concurrency)
				}
				if o.ArrivalModel != ArrivalModelUniform {
					t.Errorf("ArrivalModel = %q, want %q", o.ArrivalModel, ArrivalModelUniform)
				}
				if o.RandomSeed == 0 {
					t.Error("RandomSeed should be non-zero")
				}
				if o.LimiterFactory == nil {
					t.Error("LimiterFactory should not be nil")
				}
			},
		},
		{
			name: "negative values corrected",
			input: Options{
				Concurrency:   -5,
				TotalRequests: -10,
				RatePerSecond: -1,
			},
			validate: func(t *testing.T, o Options) {
				if o.Concurrency != 1 {
					t.Errorf("Concurrency = %d, want 1", o.Concurrency)
				}
				if o.TotalRequests != 0 {
					t.Errorf("TotalRequests = %d, want 0", o.TotalRequests)
				}
				if o.RatePerSecond != 0 {
					t.Errorf("RatePerSecond = %d, want 0", o.RatePerSecond)
				}
			},
		},
		{
			name: "preserve valid values",
			input: Options{
				Concurrency:   10,
				TotalRequests: 100,
				RatePerSecond: 50,
				ArrivalModel:  ArrivalModelPoisson,
				RandomSeed:    12345,
			},
			validate: func(t *testing.T, o Options) {
				if o.Concurrency != 10 {
					t.Errorf("Concurrency = %d, want 10", o.Concurrency)
				}
				if o.TotalRequests != 100 {
					t.Errorf("TotalRequests = %d, want 100", o.TotalRequests)
				}
				if o.RatePerSecond != 50 {
					t.Errorf("RatePerSecond = %d, want 50", o.RatePerSecond)
				}
				if o.ArrivalModel != ArrivalModelPoisson {
					t.Errorf("ArrivalModel = %q, want %q", o.ArrivalModel, ArrivalModelPoisson)
				}
				if o.RandomSeed != 12345 {
					t.Errorf("RandomSeed = %d, want 12345", o.RandomSeed)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// normalize is private, but called by NewRunner or similar.
			// Since we are testing options.go, we can access it if we are in the same package.
			// However, normalize() is a method on *Options.
			opts := tt.input
			opts.normalize()
			tt.validate(t, opts)
		})
	}
}

func TestLimiterFactory(t *testing.T) {
	opts := Options{}
	opts.normalize()

	// Test unlimited
	limiter := opts.LimiterFactory(0)
	if limiter.Limit() != rate.Inf {
		t.Errorf("Limit(0) = %v, want Inf", limiter.Limit())
	}

	// Test limited
	rps := 100
	limiter = opts.LimiterFactory(rps)
	if limiter.Limit() != rate.Limit(rps) {
		t.Errorf("Limit(%d) = %v, want %v", rps, limiter.Limit(), rate.Limit(rps))
	}
	if limiter.Burst() != rps {
		t.Errorf("Burst(%d) = %d, want %d", rps, limiter.Burst(), rps)
	}
}
