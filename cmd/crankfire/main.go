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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/torosent/crankfire/internal/auth"
	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/dashboard"
	"github.com/torosent/crankfire/internal/httpclient"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/output"
	"github.com/torosent/crankfire/internal/runner"
	"github.com/torosent/crankfire/internal/threshold"
)

// makeHeaders converts a map[string]string to http.Header
func makeHeaders(headers map[string]string) http.Header {
	h := make(http.Header)
	for k, v := range headers {
		h.Set(k, v)
	}
	return h
}

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

	authProvider, err := buildAuthProvider(cfg)
	if err != nil {
		return err
	}
	if authProvider != nil {
		defer authProvider.Close()
	}

	dataFeeder, err := buildDataFeeder(cfg)
	if err != nil {
		return err
	}
	if dataFeeder != nil {
		defer dataFeeder.Close()
	}

	collector := metrics.NewCollector()

	// Create protocol-specific requester based on configuration
	var baseRequester runner.Requester
	switch cfg.Protocol {
	case config.ProtocolWebSocket:
		baseRequester = newWebSocketRequester(cfg, collector, authProvider, dataFeeder)
	case config.ProtocolSSE:
		baseRequester = newSSERequester(cfg, collector, authProvider, dataFeeder)
	case config.ProtocolGRPC:
		baseRequester = newGRPCRequester(cfg, collector, authProvider, dataFeeder)
	case config.ProtocolHTTP:
		fallthrough
	default:
		// HTTP protocol
		builder, err := newHTTPRequestBuilder(cfg, authProvider, dataFeeder)
		if err != nil {
			return err
		}

		selector, err := newEndpointSelector(cfg)
		if err != nil {
			return err
		}

		client := httpclient.NewClient(cfg.Timeout)
		httpReq := &httpRequester{
			client:    client,
			builder:   builder,
			collector: collector,
		}

		var wrapped runner.Requester = httpReq
		if cfg.LogErrors {
			wrapped = runner.WithLogging(wrapped, &stderrFailureLogger{})
		}

		if cfg.Retries > 0 {
			wrapped = runner.WithRetry(wrapped, newRetryPolicy(cfg.Retries))
		}

		if selector != nil {
			wrapped = selector.Wrap(wrapped)
		}

		baseRequester = wrapped
	}

	opts := runner.Options{
		Concurrency:   cfg.Concurrency,
		TotalRequests: cfg.Total,
		Duration:      cfg.Duration,
		RatePerSecond: cfg.Rate,
		Requester:     baseRequester,
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

	// Parse and evaluate thresholds
	var thresholdResults []threshold.Result
	var thresholdEvaluator *threshold.Evaluator
	if len(cfg.Thresholds) > 0 {
		thresholds, err := threshold.ParseMultiple(cfg.Thresholds)
		if err != nil {
			return fmt.Errorf("threshold parsing failed: %w", err)
		}
		thresholdEvaluator = threshold.NewEvaluator(thresholds)
		thresholdResults = thresholdEvaluator.Evaluate(stats)
	}

	if cfg.JSONOutput {
		if err := output.PrintJSONReport(os.Stdout, stats, thresholdResults); err != nil {
			return err
		}
	} else {
		output.PrintReport(os.Stdout, stats, thresholdResults)
	}

	// Check if any thresholds failed
	thresholdsFailed := false
	for _, tr := range thresholdResults {
		if !tr.Pass {
			thresholdsFailed = true
			break
		}
	}

	if thresholdsFailed {
		return fmt.Errorf("one or more thresholds failed")
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
	meta := &metrics.RequestMetadata{Protocol: "http"}
	if tmpl := endpointFromContext(ctx); tmpl != nil {
		if tmpl.builder != nil {
			builder = tmpl.builder
		}
		meta.Endpoint = tmpl.name
	}
	if builder == nil {
		err := fmt.Errorf("request builder is not configured")
		meta = annotateStatus(meta, "http", fallbackStatusCode(err))
		r.collector.RecordRequest(time.Since(start), err, meta)
		return err
	}
	req, err := builder.Build(ctx)
	if err != nil {
		meta = annotateStatus(meta, "http", fallbackStatusCode(err))
		r.collector.RecordRequest(time.Since(start), err, meta)
		return err
	}

	resp, err := r.client.Do(req)
	latency := time.Since(start)
	if err != nil {
		meta = annotateStatus(meta, "http", fallbackStatusCode(err))
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
		meta = annotateStatus(meta, "http", strconv.Itoa(resp.StatusCode))
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
	}

	if resultErr != nil && meta.StatusCode == "" {
		meta = annotateStatus(meta, "http", httpStatusCodeFromError(resultErr))
	}
	r.collector.RecordRequest(latency, resultErr, meta)
	return resultErr
}

func httpStatusCodeFromError(err error) string {
	if err == nil {
		return ""
	}
	var httpErr *runner.HTTPError
	if errors.As(err, &httpErr) && httpErr.StatusCode > 0 {
		return strconv.Itoa(httpErr.StatusCode)
	}
	return fallbackStatusCode(err)
}

func newHTTPRequestBuilder(cfg *config.Config, provider auth.Provider, feeder httpclient.Feeder) (*httpclient.RequestBuilder, error) {
	switch {
	case provider != nil && feeder != nil:
		return httpclient.NewRequestBuilderWithAuthAndFeeder(cfg, provider, feeder)
	case provider != nil:
		return httpclient.NewRequestBuilderWithAuth(cfg, provider)
	case feeder != nil:
		return httpclient.NewRequestBuilderWithFeeder(cfg, feeder)
	default:
		return httpclient.NewRequestBuilder(cfg)
	}
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
