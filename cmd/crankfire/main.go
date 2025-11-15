package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/dashboard"
	"github.com/torosent/crankfire/internal/httpclient"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/output"
	"github.com/torosent/crankfire/internal/runner"
)

const (
	progressInterval   = time.Second
	maxLoggedBodyBytes = 1024
	baseRetryDelay     = 100 * time.Millisecond
	maxRetryDelay      = 5 * time.Second
)

type httpRequester struct {
	client    *http.Client
	builder   *httpclient.RequestBuilder
	collector *metrics.Collector
}

type stderrFailureLogger struct {
	mu sync.Mutex
}

type jitterSource struct {
	mu  sync.Mutex
	rnd *rand.Rand
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	loader := config.NewLoader()
	cfg, err := loader.Load(args)
	if err != nil {
		if errors.Is(err, config.ErrHelpRequested) {
			return nil
		}
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	builder, err := httpclient.NewRequestBuilder(cfg)
	if err != nil {
		return err
	}

	selector, err := newEndpointSelector(cfg)
	if err != nil {
		return err
	}

	client := httpclient.NewClient(cfg.Timeout)
	collector := metrics.NewCollector()

	requester := &httpRequester{
		client:    client,
		builder:   builder,
		collector: collector,
	}

	var wrapped runner.Requester = requester
	if cfg.LogErrors {
		wrapped = runner.WithLogging(wrapped, &stderrFailureLogger{})
	}

	if cfg.Retries > 0 {
		wrapped = runner.WithRetry(wrapped, newRetryPolicy(cfg.Retries))
	}

	if selector != nil {
		wrapped = selector.Wrap(wrapped)
	}

	opts := runner.Options{
		Concurrency:   cfg.Concurrency,
		TotalRequests: cfg.Total,
		Duration:      cfg.Duration,
		RatePerSecond: cfg.Rate,
		Requester:     wrapped,
		ArrivalModel:  toRunnerArrivalModel(cfg.Arrival.Model),
		LoadPatterns:  toRunnerLoadPatterns(cfg.LoadPatterns),
	}

	r := runner.New(opts)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var dash *dashboard.Dashboard
	if cfg.Dashboard {
		dash, err = dashboard.New(collector)
		if err != nil {
			return err
		}
		dash.Start()
		defer dash.Stop()
	}

	var progress *output.ProgressReporter
	if !cfg.JSONOutput && !cfg.Dashboard {
		progress = output.NewProgressReporter(collector, progressInterval, os.Stdout)
		progress.Start()
		defer func() {
			progress.Stop()
			fmt.Fprintln(os.Stdout)
		}()
	}

	// Mark the actual start time in the collector for accurate RPS calculation.
	// This ensures dashboard/progress reporters (which may have been created earlier)
	// use the correct elapsed time since the test actually began.
	collector.Start()
	result := r.Run(ctx)
	stats := collector.Stats(result.Duration)

	if cfg.JSONOutput {
		if err := output.PrintJSONReport(os.Stdout, stats); err != nil {
			return err
		}
	} else {
		output.PrintReport(os.Stdout, stats)
	}

	if result.Errors > 0 {
		return fmt.Errorf("%d requests failed", result.Errors)
	}
	return nil
}

func (r *httpRequester) Do(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	start := time.Now()
	builder := r.builder
	meta := &metrics.RequestMetadata{}
	if tmpl := endpointFromContext(ctx); tmpl != nil {
		if tmpl.builder != nil {
			builder = tmpl.builder
		}
		meta.Endpoint = tmpl.name
	}
	if meta.Endpoint == "" {
		meta = nil
	}
	if builder == nil {
		err := fmt.Errorf("request builder is not configured")
		r.collector.RecordRequest(time.Since(start), err, meta)
		return err
	}
	req, err := builder.Build(ctx)
	if err != nil {
		r.collector.RecordRequest(time.Since(start), err, meta)
		return err
	}

	resp, err := r.client.Do(req)
	latency := time.Since(start)
	if err != nil {
		r.collector.RecordRequest(latency, err, meta)
		return err
	}
	defer resp.Body.Close()

	var resultErr error
	if resp.StatusCode >= 400 {
		snippet, readErr := io.ReadAll(io.LimitReader(resp.Body, maxLoggedBodyBytes))
		if readErr != nil {
			resultErr = readErr
		} else {
			resultErr = &runner.HTTPError{
				StatusCode: resp.StatusCode,
				Body:       strings.TrimSpace(string(snippet)),
			}
		}
		_, _ = io.Copy(io.Discard, resp.Body)
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
	}

	r.collector.RecordRequest(latency, resultErr, meta)
	return resultErr
}

func toRunnerArrivalModel(model config.ArrivalModel) runner.ArrivalModel {
	switch strings.ToLower(string(model)) {
	case string(config.ArrivalModelPoisson):
		return runner.ArrivalModelPoisson
	default:
		return runner.ArrivalModelUniform
	}
}

func toRunnerLoadPatterns(patterns []config.LoadPattern) []runner.LoadPattern {
	if len(patterns) == 0 {
		return nil
	}
	result := make([]runner.LoadPattern, len(patterns))
	for i, p := range patterns {
		result[i] = runner.LoadPattern{
			Name:     p.Name,
			Type:     runner.LoadPatternType(p.Type),
			FromRPS:  p.FromRPS,
			ToRPS:    p.ToRPS,
			Duration: p.Duration,
			Steps:    toRunnerLoadSteps(p.Steps),
			RPS:      p.RPS,
		}
	}
	return result
}

func toRunnerLoadSteps(steps []config.LoadStep) []runner.LoadStep {
	if len(steps) == 0 {
		return nil
	}
	result := make([]runner.LoadStep, len(steps))
	for i, s := range steps {
		result[i] = runner.LoadStep{
			RPS:      s.RPS,
			Duration: s.Duration,
		}
	}
	return result
}

func (l *stderrFailureLogger) LogFailure(err error) {
	if err == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(os.Stderr, "[crankfire] request failed: %v\n", err)
}

func newRetryPolicy(retries int) runner.RetryPolicy {
	source := &jitterSource{rnd: rand.New(rand.NewSource(time.Now().UnixNano()))}

	return runner.RetryPolicy{
		MaxAttempts: retries + 1,
		ShouldRetry: func(err error) bool {
			if err == nil {
				return false
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return false
			}

			var httpErr *runner.HTTPError
			if errors.As(err, &httpErr) {
				if httpErr.StatusCode == http.StatusTooManyRequests {
					return true
				}
				return httpErr.StatusCode >= 500
			}

			return true
		},
		DelayFunc: func(attempt int, err error) time.Duration {
			if attempt < 1 {
				attempt = 1
			}
			backoff := time.Duration(1<<uint(attempt-1)) * baseRetryDelay
			if backoff > maxRetryDelay {
				backoff = maxRetryDelay
			}
			return backoff + source.jitter(backoff/2)
		},
	}
}

func (j *jitterSource) jitter(max time.Duration) time.Duration {
	if j == nil || max <= 0 {
		return 0
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return time.Duration(j.rnd.Int63n(int64(max)))
}
