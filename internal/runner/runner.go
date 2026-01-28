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
	opt     Options
	plan    *patternPlan
	arrival arrivalController
}

func New(opt Options) *Runner {
	opt.normalize()
	plan := compilePatternPlan(opt.LoadPatterns)
	arrival := newArrivalController(opt, plan)
	return &Runner{opt: opt, plan: plan, arrival: arrival}
}

func (r *Runner) Run(ctx context.Context) Result {
	start := time.Now()
	var total int64
	var errs int64

	// runCtx is used for in-flight requests.
	// It is cancelled on Ctrl+C (via the parent ctx) and may be cancelled after
	// a grace period when the test ends naturally.
	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	// schedulerCtx controls when to stop issuing *new* requests.
	// It stops on duration/total/pattern completion or parent cancellation, but does not
	// automatically cancel in-flight requests.
	schedulerCtx, schedulerCancel := context.WithCancel(runCtx)
	defer schedulerCancel()

	if r.opt.Duration > 0 {
		deadlineCtx, deadlineCancel := context.WithTimeout(schedulerCtx, r.opt.Duration)
		schedulerCtx = deadlineCtx
		defer deadlineCancel()
	}

	var patternCancel context.CancelFunc
	if r.plan != nil {
		patternCtx, cancelPattern := context.WithCancel(schedulerCtx)
		schedulerCtx = patternCtx
		patternCancel = cancelPattern
		go r.runPatternController(patternCtx, patternCancel)
	}

	permits := make(chan struct{}, r.opt.Concurrency)

	scheduleDone := make(chan struct{})

	// Scheduler: serializes rate limiting to avoid burst overshoot across workers.
	go func() {
		defer close(scheduleDone)
		defer close(permits)
		for {
			if schedulerCtx.Err() != nil {
				return
			}
			current := atomic.LoadInt64(&total)
			if r.opt.TotalRequests > 0 && current >= int64(r.opt.TotalRequests) {
				return
			}
			if r.arrival != nil {
				if err := r.arrival.Wait(schedulerCtx); err != nil {
					return
				}
			}
			// Increment total before releasing permit so workers only execute allocated slots.
			atomic.AddInt64(&total, 1)
			select {
			case permits <- struct{}{}:
			case <-schedulerCtx.Done():
				atomic.AddInt64(&total, -1)
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
					err := r.opt.Requester.Do(runCtx)
					if err != nil {
						atomic.AddInt64(&errs, 1)
					}
				}
			}
		}()
	}

	// Wait until scheduling is complete (duration/total/pattern end or cancellation)
	// before applying the graceful shutdown window.
	<-scheduleDone

	workersDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(workersDone)
	}()

	// If GracefulShutdown is negative, cancel immediately after scheduling stops.
	// Otherwise, allow in-flight requests to finish for up to the configured window,
	// then force cancel.
	if r.opt.GracefulShutdown < 0 {
		runCancel()
		<-workersDone
	} else {
		timer := time.NewTimer(r.opt.GracefulShutdown)
		defer timer.Stop()
		select {
		case <-workersDone:
			// All workers finished within grace.
		case <-runCtx.Done():
			// Parent cancellation (Ctrl+C): just wait for workers to exit.
			<-workersDone
		case <-timer.C:
			// Grace elapsed: force cancel in-flight requests.
			runCancel()
			<-workersDone
		}
	}

	return Result{
		Total:    atomic.LoadInt64(&total),
		Errors:   atomic.LoadInt64(&errs),
		Duration: time.Since(start),
	}
}

func (r *Runner) runPatternController(ctx context.Context, cancel context.CancelFunc) {
	if r.plan == nil || r.arrival == nil {
		if cancel != nil {
			cancel()
		}
		return
	}
	defer func() {
		if cancel != nil {
			cancel()
		}
	}()

	start := time.Now()
	if initial, ok := r.plan.rateAt(0); ok {
		r.arrival.SetRate(initial)
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			elapsed := time.Since(start)
			rps, ok := r.plan.rateAt(elapsed)
			if !ok {
				return
			}
			r.arrival.SetRate(rps)
		}
	}
}
