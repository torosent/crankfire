package runner_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/runner"
)

// TestRetryRespectsMaxAttempts verifies retry count is honored.
func TestRetryRespectsMaxAttempts(t *testing.T) {
	var attempts int64
	failUntil := int64(3)

	requester := &retryableRequester{
		attempts:  &attempts,
		failUntil: failUntil,
	}

	policy := runner.RetryPolicy{
		MaxAttempts: 5,
		DelayFunc: func(attempt int, err error) time.Duration {
			return time.Duration(attempt) * time.Millisecond // linear backoff for test determinism
		},
	}

	r := runner.New(runner.Options{
		Concurrency:   1,
		TotalRequests: 1,
		Requester:     runner.WithRetry(requester, policy),
	})

	res := r.Run(context.Background())

	if res.Total != 1 {
		t.Errorf("expected total 1, got %d", res.Total)
	}
	if res.Errors != 0 {
		t.Errorf("expected errors 0, got %d", res.Errors)
	}
	// Should succeed on 4th attempt (3 retries after initial failure).
	if attempts != 4 {
		t.Errorf("expected 4 attempts, got %d", attempts)
	}
}

func TestRetryExceedsMaxAttempts(t *testing.T) {
	var attempts int64

	requester := &retryableRequester{
		attempts:  &attempts,
		failUntil: 100, // always fails
	}

	policy := runner.RetryPolicy{
		MaxAttempts: 3,
		DelayFunc:   func(attempt int, err error) time.Duration { return time.Millisecond },
	}

	r := runner.New(runner.Options{
		Concurrency:   1,
		TotalRequests: 1,
		Requester:     runner.WithRetry(requester, policy),
	})

	res := r.Run(context.Background())

	if res.Errors != 1 {
		t.Errorf("expected errors 1, got %d", res.Errors)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts (max), got %d", attempts)
	}
}

func TestNon2xxRecorded(t *testing.T) {
	logger := &testLogger{}

	requester := &statusRequester{
		statusCode: 500,
	}

	r := runner.New(runner.Options{
		Concurrency:   1,
		TotalRequests: 2,
		Requester:     runner.WithLogging(requester, logger),
	})

	res := r.Run(context.Background())

	if res.Total != 2 {
		t.Errorf("expected total 2, got %d", res.Total)
	}
	if logger.count != 2 {
		t.Errorf("expected 2 logged failures, got %d", logger.count)
	}
}

type retryableRequester struct {
	attempts  *int64
	failUntil int64
}

func (r *retryableRequester) Do(ctx context.Context) error {
	attempt := atomic.AddInt64(r.attempts, 1)
	if attempt <= r.failUntil {
		return errors.New("transient failure")
	}
	return nil
}

type statusRequester struct {
	statusCode int
}

func (s *statusRequester) Do(ctx context.Context) error {
	if s.statusCode >= 400 {
		return &runner.HTTPError{StatusCode: s.statusCode, Body: "error body"}
	}
	return nil
}

type testLogger struct {
	count int
}

func (l *testLogger) LogFailure(err error) {
	l.count++
}
