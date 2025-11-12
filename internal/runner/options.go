package runner

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

// Requester abstracts executing a single request operation.
// Implementations should return an error for failed requests.
type Requester interface {
	Do(ctx context.Context) error
}

// Options configure the Runner.
type Options struct {
	Concurrency    int           // number of worker goroutines
	TotalRequests  int           // total requests to execute (0 means unlimited until duration/end)
	Duration       time.Duration // overall time limit (0 means no duration cap)
	RatePerSecond  int           // requests per second pacing (0 means unlimited)
	Requester      Requester     // request executor (required)
	LimiterFactory func(rps int) *rate.Limiter // optional injection for tests
}

func (o *Options) normalize() {
	if o.Concurrency <= 0 {
		o.Concurrency = 1
	}
	if o.TotalRequests < 0 {
		o.TotalRequests = 0
	}
	if o.RatePerSecond < 0 {
		o.RatePerSecond = 0
	}
	if o.LimiterFactory == nil {
		o.LimiterFactory = func(rps int) *rate.Limiter {
			if rps <= 0 {
				return rate.NewLimiter(rate.Inf, 0)
			}
			// Burst equal to rps to smooth pacing under concurrency.
			return rate.NewLimiter(rate.Limit(rps), rps)
		}
	}
}
