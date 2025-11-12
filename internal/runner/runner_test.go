package runner_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/torosent/crankfire/internal/runner"
)

// fakeRequester simulates performing a request with fixed latency.
type fakeRequester struct {
	latency   time.Duration
	calls     *int64
	failAfter int64 // if >0, fails after this many successful calls
}

func (f *fakeRequester) Do(ctx context.Context) error {
	if f.calls != nil {
		atomic.AddInt64(f.calls, 1)
	}
	select {
	case <-time.After(f.latency):
	case <-ctx.Done():
		return ctx.Err()
	}
	if f.failAfter > 0 && atomic.LoadInt64(f.calls) > f.failAfter {
		return context.DeadlineExceeded // arbitrary error
	}
	return nil
}

// TestRunnerRespectsTotalRequests ensures total limit stops execution.
func TestRunnerRespectsTotalRequests(t *testing.T) {
	var calls int64
	r := runner.New(runner.Options{
		Concurrency: 4,
		TotalRequests: 25,
		Requester: &fakeRequester{latency: 1 * time.Millisecond, calls: &calls},
	})
	res := r.Run(context.Background())
	if res.Total != 25 {
		t.Fatalf("expected total 25, got %d", res.Total)
	}
	if calls != 25 {
		t.Fatalf("expected requester called 25 times, got %d", calls)
	}
}

// TestRunnerHonorsDuration ensures duration cap stops even if total not reached.
func TestRunnerHonorsDuration(t *testing.T) {
	var calls int64
	r := runner.New(runner.Options{
		Concurrency: 10,
		Duration: 50 * time.Millisecond,
		TotalRequests: 0,
		Requester: &fakeRequester{latency: 5 * time.Millisecond, calls: &calls},
	})
	start := time.Now()
	res := r.Run(context.Background())
	elapsed := time.Since(start)
	if elapsed < 50*time.Millisecond || elapsed > 250*time.Millisecond {
		// allow some scheduling fudge but not extremely off
		t.Fatalf("duration enforcement off: %s", elapsed)
	}
	if res.Duration <= 0 {
		t.Fatalf("result duration not recorded")
	}
	if res.Total <= 0 {
		t.Fatalf("expected some requests executed")
	}
}

// TestRateLimiterCapsThroughput ensures rate limiter restricts RPS.
func TestRateLimiterCapsThroughput(t *testing.T) {
	var calls int64
	rateLimit := 100 // requests per second theoretical maximum
	duration := 100 * time.Millisecond
	r := runner.New(runner.Options{
		Concurrency:    20,
		Duration:       duration,
		RatePerSecond:  rateLimit,
		Requester:      &fakeRequester{latency: 0, calls: &calls},
		LimiterFactory: func(rps int) *rate.Limiter { return rate.NewLimiter(rate.Limit(rps), 1) },
	})
	res := r.Run(context.Background())
	// expected upper bound ~ rateLimit * (duration seconds)
	maxExpected := int(float64(rateLimit) * (float64(duration) / float64(time.Second)) * 1.20) // 20% slack
	if int(res.Total) > maxExpected {
		t.Fatalf("rate limiter exceeded: total=%d max=%d", res.Total, maxExpected)
	}
	if calls != res.Total {
		t.Fatalf("calls mismatch: %d vs %d", calls, res.Total)
	}
}
