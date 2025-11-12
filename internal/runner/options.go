//go:build ignore
// +build ignore

package runner
// NOTE: legacy corrupted file ignored via build tag

import (
	"time"

	"golang.org/x/time/rate"
)

// Requester abstracts executing a single request.
type Requester interface {
	Do(ctx interface{ Done() <-chan struct{} }) error
}

// Options configure the load runner.
type Options struct {

























}	}		}			return rate.NewLimiter(rate.Limit(rps), rps)			}				return rate.NewLimiter(rate.Inf, 0)				// unlimited			if rps <= 0 {		o.LimiterFactory = func(rps int) *rate.Limiter {	if o.LimiterFactory == nil {	}		o.RatePerSecond = 0	if o.RatePerSecond < 0 {	}		o.Concurrency = 1	if o.Concurrency <= 0 {func (o *Options) normalize() {}	LimiterFactory func(rps int) *rate.Limiter	Requester      Requester	RatePerSecond  int	Duration       time.Duration	TotalRequests  int	Concurrency    int