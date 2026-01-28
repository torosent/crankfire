package main

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/torosent/crankfire/internal/threshold"
)

const (
	progressInterval = time.Second
	baseRetryDelay   = 100 * time.Millisecond
	maxRetryDelay    = 5 * time.Second
)

type stderrFailureLogger struct {
	mu sync.Mutex
}

type stderrLogger struct {
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

	// Load HAR endpoints if specified
	if err := loadHAREndpoints(cfg); err != nil {
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
		var builder *httpclient.RequestBuilder
		if cfg.TargetURL != "" {
			var err error
			builder, err = newHTTPRequestBuilder(cfg, authProvider, dataFeeder)
			if err != nil {
				return err
			}
		}

		selector, err := newEndpointSelector(cfg)
		if err != nil {
			return err
		}

		if builder == nil && selector == nil {
			return fmt.Errorf("target URL is required")
		}

		client := httpclient.NewClient(cfg.Timeout)
		httpReq := &httpRequester{
			client:    client,
			builder:   builder,
			collector: collector,
			helper: baseRequesterHelper{
				collector: collector,
				auth:      authProvider,
				feeder:    dataFeeder,
			},
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
		Concurrency:      cfg.Concurrency,
		TotalRequests:    cfg.Total,
		Duration:         cfg.Duration,
		RatePerSecond:    cfg.Rate,
		GracefulShutdown: cfg.GracefulShutdown,
		Requester:        baseRequester,
		ArrivalModel:     toRunnerArrivalModel(cfg.Arrival.Model),
		LoadPatterns:     toRunnerLoadPatterns(cfg.LoadPatterns),
	}

	r := runner.New(opts)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var dash *dashboard.Dashboard
	if cfg.Dashboard {
		targetURL := cfg.TargetURL
		if targetURL == "" && len(cfg.Endpoints) > 0 {
			// Use first endpoint URL if no global target
			targetURL = cfg.Endpoints[0].URL
			if targetURL == "" && cfg.Endpoints[0].Path != "" {
				targetURL = cfg.Endpoints[0].Path
			}
		}
		dashCfg := dashboard.TestConfig{
			TargetURL:   targetURL,
			Concurrency: cfg.Concurrency,
			Duration:    cfg.Duration,
			Total:       cfg.Total,
			Rate:        cfg.Rate,
			Timeout:     cfg.Timeout,
			Retries:     cfg.Retries,
			Protocol:    string(cfg.Protocol),
			Method:      cfg.Method,
			ConfigFile:  cfg.ConfigFile,
		}
		dash, err = dashboard.New(collector, dashCfg, cancel)
		if err != nil {
			return err
		}
		dash.Start()
		defer func() {
			if dash != nil {
				dash.Stop()
			}
		}()
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

	// Start periodic snapshots for HTML report history (if enabled)
	var snapshotTicker *time.Ticker
	var snapshotDone chan struct{}
	var snapshotStop chan struct{}
	if cfg.HTMLOutput != "" {
		snapshotTicker = time.NewTicker(1 * time.Second)
		snapshotDone = make(chan struct{})
		snapshotStop = make(chan struct{})
		go func() {
			defer close(snapshotDone)
			for {
				select {
				case <-snapshotTicker.C:
					collector.Snapshot()
				case <-snapshotStop:
					return
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	result := r.Run(ctx)

	// Stop snapshot collection
	if snapshotTicker != nil {
		snapshotTicker.Stop()
		close(snapshotStop)
		<-snapshotDone
		// Take one final snapshot after the test completes
		collector.Snapshot()
	}

	if dash != nil {
		dash.Stop()
		dash = nil
	}

	stats := collector.Stats(result.Duration)

	// Parse and evaluate thresholds
	var thresholdResults []threshold.Result
	if len(cfg.Thresholds) > 0 {
		thresholds, err := threshold.ParseMultiple(cfg.Thresholds)
		if err != nil {
			return fmt.Errorf("threshold parsing failed: %w", err)
		}
		evaluator := threshold.NewEvaluator(thresholds)
		thresholdResults = evaluator.Evaluate(stats)
	}

	if cfg.JSONOutput {
		if err := output.PrintJSONReport(os.Stdout, stats, thresholdResults); err != nil {
			return err
		}
	} else {
		output.PrintReport(os.Stdout, stats, thresholdResults)
	}

	// Generate HTML report if requested
	if cfg.HTMLOutput != "" {
		history := collector.History()
		file, err := os.Create(cfg.HTMLOutput)
		if err != nil {
			return fmt.Errorf("failed to create HTML report file: %w", err)
		}
		defer file.Close()

		// Prepare metadata
		testedEndpoints := make([]output.TestedEndpoint, len(cfg.Endpoints))
		for i, ep := range cfg.Endpoints {
			testedEndpoints[i] = output.TestedEndpoint{
				Name:   ep.Name,
				Method: ep.Method,
				URL:    ep.URL,
			}
			if testedEndpoints[i].URL == "" {
				testedEndpoints[i].URL = ep.Path
			}
		}

		metadata := output.ReportMetadata{
			TargetURL:       cfg.TargetURL,
			TestedEndpoints: testedEndpoints,
		}

		if err := output.GenerateHTMLReport(file, stats, history, thresholdResults, metadata); err != nil {
			return fmt.Errorf("failed to generate HTML report: %w", err)
		}
		fmt.Fprintf(os.Stderr, "\nHTML report generated: %s\n", cfg.HTMLOutput)
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

func (l *stderrLogger) Warn(format string, args ...interface{}) {
	if format == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(os.Stderr, "[crankfire] warning: "+format+"\n", args...)
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
