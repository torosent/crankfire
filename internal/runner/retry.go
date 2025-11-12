package runner

import (
	"context"
	"fmt"
	"time"
)

// HTTPError represents an HTTP request failure with status details.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

// FailureLogger logs failed requests.
type FailureLogger interface {
	LogFailure(err error)
}

// RetryPolicy configures retry behavior.
type RetryPolicy struct {
	MaxAttempts int                                      // total attempts including initial try
	Delay       time.Duration                            // fixed delay between retries (used if DelayFunc nil)
	ShouldRetry func(error) bool                         // predicate; if nil, all errors retried
	DelayFunc   func(attempt int, err error) time.Duration // dynamic backoff; attempt is 1-based
}

// retryRequester wraps a Requester with retry logic.
type retryRequester struct {
	inner  Requester
	policy RetryPolicy
}

// WithRetry wraps a Requester with retry capability.
func WithRetry(req Requester, policy RetryPolicy) Requester {
	if policy.MaxAttempts <= 1 {
		return req // no retries needed
	}
	return &retryRequester{
		inner:  req,
		policy: policy,
	}
}

func (r *retryRequester) Do(ctx context.Context) error {
	var lastErr error
	for attempt := 1; attempt <= r.policy.MaxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		lastErr = r.inner.Do(ctx)
		if lastErr == nil {
			return nil // success
		}

		// Don't delay after the last attempt.
		if attempt < r.policy.MaxAttempts {
			if r.policy.ShouldRetry != nil && !r.policy.ShouldRetry(lastErr) {
				return lastErr
			}
			var delay time.Duration
			if r.policy.DelayFunc != nil {
				delay = r.policy.DelayFunc(attempt, lastErr)
			} else {
				delay = r.policy.Delay
			}
			if delay > 0 {
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}
	return lastErr
}

// loggingRequester wraps a Requester with failure logging.
type loggingRequester struct {
	inner  Requester
	logger FailureLogger
}

// WithLogging wraps a Requester to log failures.
func WithLogging(req Requester, logger FailureLogger) Requester {
	if logger == nil {
		return req
	}
	return &loggingRequester{
		inner:  req,
		logger: logger,
	}
}

func (l *loggingRequester) Do(ctx context.Context) error {
	err := l.inner.Do(ctx)
	if err != nil && l.logger != nil {
		l.logger.LogFailure(err)
	}
	return err
}
