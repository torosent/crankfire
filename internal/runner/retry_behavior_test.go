package runner_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/runner"
)

type singleAttemptRequester struct{ attempts int }

func (r *singleAttemptRequester) Do(ctx context.Context) error {
	r.attempts++
	return errors.New("permanent failure")
}

func TestRetryShouldRetryStopsEarly(t *testing.T) {
	req := &singleAttemptRequester{}
	policy := runner.RetryPolicy{
		MaxAttempts: 5,
		ShouldRetry: func(err error) bool { return false }, // never retry
		DelayFunc:   func(int, error) time.Duration { return 0 },
	}
	wrapped := runner.WithRetry(req, policy)
	err := wrapped.Do(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	if req.attempts != 1 {
		t.Fatalf("expected 1 attempt got %d", req.attempts)
	}
}
