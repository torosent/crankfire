package runner

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Result captures execution summary.
type Result struct {
	Total    int64
	Errors   int64
	Duration time.Duration
}

// Runner coordinates concurrent execution with rate limiting.
type Runner struct {
	opt Options
}

func New(opt Options) *Runner {
	opt.normalize()
	return &Runner{opt: opt}
}

func (r *Runner) Run(ctx context.Context) Result {
	start := time.Now()
	var total int64
	var errs int64

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if r.opt.Duration > 0 {
		// Stop after duration elapses.
		deadlineCtx, deadlineCancel := context.WithTimeout(ctx, r.opt.Duration)
		ctx = deadlineCtx
		defer deadlineCancel()
	}

	limiter := r.opt.LimiterFactory(r.opt.RatePerSecond)

	permits := make(chan struct{}, r.opt.Concurrency)

	// Scheduler: serializes rate limiting to avoid burst overshoot across workers.
	go func() {
		defer close(permits)
		for {
			if ctx.Err() != nil {
				return
			}
			current := atomic.LoadInt64(&total)
			if r.opt.TotalRequests > 0 && current >= int64(r.opt.TotalRequests) {
				return
			}
			if err := limiter.Wait(ctx); err != nil {
				return
			}
			// Increment total before releasing permit so workers only execute allocated slots.
			atomic.AddInt64(&total, 1)
			select {
			case permits <- struct{}{}:
			case <-ctx.Done():
				return
			}
		}
	}()

	var wg sync.WaitGroup
	wg.Add(r.opt.Concurrency)
	for i := 0; i < r.opt.Concurrency; i++ {
		go func() {
			defer wg.Done()
			for range permits {
				if r.opt.Requester != nil {
					err := r.opt.Requester.Do(ctx)
					if err != nil {
						atomic.AddInt64(&errs, 1)
					}
				}
				if ctx.Err() != nil {
					return
				}
			}
		}()
	}
	wg.Wait()

	return Result{
		Total:    atomic.LoadInt64(&total),
		Errors:   atomic.LoadInt64(&errs),
		Duration: time.Since(start),
	}
}
