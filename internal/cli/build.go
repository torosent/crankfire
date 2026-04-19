package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/torosent/crankfire/internal/auth"
	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/httpclient"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/runner"
	"github.com/torosent/crankfire/internal/tracing"
)

// BuildRunner constructs the full runner dependency graph for an in-process
// run, mirroring the wiring used by the CLI Run() entrypoint. The returned
// cleanup func releases auth/data feeder/tracing resources and is safe to
// call once.
//
// The provided ctx is used to initialize the tracing provider (matching the
// CLI behavior).
func BuildRunner(ctx context.Context, cfg config.Config) (*runner.Runner, *metrics.Collector, func(), error) {
	tracingProvider, err := tracing.Init(ctx, cfg.Tracing)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("tracing init: %w", err)
	}

	authProvider, err := buildAuthProvider(&cfg)
	if err != nil {
		shutdownTracing(tracingProvider)
		return nil, nil, nil, err
	}

	dataFeeder, err := buildDataFeeder(&cfg)
	if err != nil {
		if authProvider != nil {
			authProvider.Close()
		}
		shutdownTracing(tracingProvider)
		return nil, nil, nil, err
	}

	collector := metrics.NewCollector()

	baseRequester, err := buildRequester(&cfg, collector, authProvider, dataFeeder, tracingProvider)
	if err != nil {
		if dataFeeder != nil {
			dataFeeder.Close()
		}
		if authProvider != nil {
			authProvider.Close()
		}
		shutdownTracing(tracingProvider)
		return nil, nil, nil, err
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

	var cleaned bool
	cleanup := func() {
		if cleaned {
			return
		}
		cleaned = true
		if dataFeeder != nil {
			dataFeeder.Close()
		}
		if authProvider != nil {
			authProvider.Close()
		}
		shutdownTracing(tracingProvider)
	}

	return r, collector, cleanup, nil
}

func shutdownTracing(p *tracing.Provider) {
	if p == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = p.Shutdown(ctx)
}

// buildRequester constructs the protocol-specific requester chain (auth,
// retries, logging, endpoint selection) shared by the CLI Run() entrypoint
// and BuildRunner.
func buildRequester(
	cfg *config.Config,
	collector *metrics.Collector,
	authProvider auth.Provider,
	dataFeeder httpclient.Feeder,
	tracingProvider *tracing.Provider,
) (runner.Requester, error) {
	switch cfg.Protocol {
	case config.ProtocolWebSocket:
		return newWebSocketRequester(cfg, collector, authProvider, dataFeeder, tracingProvider), nil
	case config.ProtocolSSE:
		return newSSERequester(cfg, collector, authProvider, dataFeeder, tracingProvider), nil
	case config.ProtocolGRPC:
		return newGRPCRequester(cfg, collector, authProvider, dataFeeder, tracingProvider), nil
	case config.ProtocolHTTP:
		fallthrough
	default:
		var builder *httpclient.RequestBuilder
		if cfg.TargetURL != "" {
			b, err := newHTTPRequestBuilder(cfg, authProvider, dataFeeder)
			if err != nil {
				return nil, err
			}
			builder = b
		}

		selector, err := newEndpointSelector(cfg)
		if err != nil {
			return nil, err
		}

		if builder == nil && selector == nil {
			return nil, fmt.Errorf("target URL is required")
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
				tracing:   tracingProvider,
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
		return wrapped, nil
	}
}
