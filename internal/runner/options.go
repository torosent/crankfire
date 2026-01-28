package runner

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

// Requester abstracts executing a single request operation.
// Implementations should return an error for failed requests.
type Requester interface {
	Do(ctx context.Context) error
}

// Options configure the Runner.
type Options struct {
	Concurrency      int                         // number of worker goroutines
	TotalRequests    int                         // total requests to execute (0 means unlimited until duration/end)
	Duration         time.Duration               // overall time limit (0 means no duration cap)
	RatePerSecond    int                         // requests per second pacing (0 means unlimited)
	GracefulShutdown time.Duration               // max time to wait for in-flight requests after scheduling stops (0 is normalized to default 5s, <0=cancel immediately)
	Requester        Requester                   // request executor (required)
	LimiterFactory   func(rps int) *rate.Limiter // optional injection for tests
	LoadPatterns     []LoadPattern               // optional pattern schedule
	ArrivalModel     ArrivalModel                // pacing model (uniform or poisson)
	RandomSeed       int64                       // seed used for stochastic models (0 => auto)
	PoissonSampler   func() float64              // optional sampler override for tests (returns Exp(1))
}

func (o *Options) normalize() {
	if o.Concurrency <= 0 {
		o.Concurrency = 1
	}
	if o.TotalRequests < 0 {
		o.TotalRequests = 0
	}
	if o.RatePerSecond < 0 {
		o.RatePerSecond = 0
	}
	if o.GracefulShutdown == 0 {
		o.GracefulShutdown = 5 * time.Second // default: 5s grace for in-flight requests
	}
	if o.LimiterFactory == nil {
		o.LimiterFactory = func(rps int) *rate.Limiter {
			if rps <= 0 {
				return rate.NewLimiter(rate.Inf, 0)
			}
			// Burst equal to rps to smooth pacing under concurrency.
			return rate.NewLimiter(rate.Limit(rps), rps)
		}
	}
	if o.ArrivalModel == "" {
		o.ArrivalModel = ArrivalModelUniform
	}
	if o.RandomSeed == 0 {
		o.RandomSeed = time.Now().UnixNano()
	}
}

type LoadPatternType string

const (
	LoadPatternTypeRamp  LoadPatternType = "ramp"
	LoadPatternTypeStep  LoadPatternType = "step"
	LoadPatternTypeSpike LoadPatternType = "spike"
)

type LoadPattern struct {
	Name     string
	Type     LoadPatternType
	FromRPS  int
	ToRPS    int
	Duration time.Duration
	Steps    []LoadStep
	RPS      int
}

type LoadStep struct {
	RPS      int
	Duration time.Duration
}

type ArrivalModel string

const (
	ArrivalModelUniform ArrivalModel = "uniform"
	ArrivalModelPoisson ArrivalModel = "poisson"
)
