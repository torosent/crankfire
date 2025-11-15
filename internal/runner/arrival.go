package runner

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type arrivalController interface {
	Wait(ctx context.Context) error
	SetRate(rps float64)
}

func newArrivalController(opt Options, plan *patternPlan) arrivalController {
	baseRate := float64(opt.RatePerSecond)
	if plan != nil {
		if rate, ok := plan.rateAt(0); ok {
			baseRate = rate
		} else {
			baseRate = 0
		}
	}

	switch opt.ArrivalModel {
	case ArrivalModelPoisson:
		var sampler func() float64
		if opt.PoissonSampler != nil {
			sampler = opt.PoissonSampler
		} else {
			seeded := rand.New(rand.NewSource(opt.RandomSeed))
			sampler = seeded.ExpFloat64
		}
		ctrl := &poissonArrival{sample: sampler}
		ctrl.SetRate(baseRate)
		return ctrl
	default:
		limiter := opt.LimiterFactory(opt.RatePerSecond)
		ctrl := &uniformArrival{limiter: limiter}
		if plan != nil {
			ctrl.SetRate(baseRate)
		}
		return ctrl
	}
}

// uniformArrival delegates pacing to a rate.Limiter (uniform spacing).
type uniformArrival struct {
	limiter *rate.Limiter
}

func (u *uniformArrival) Wait(ctx context.Context) error {
	if u == nil || u.limiter == nil {
		return nil
	}
	return u.limiter.Wait(ctx)
}

func (u *uniformArrival) SetRate(rps float64) {
	if u == nil || u.limiter == nil {
		return
	}
	if rps <= 0 {
		u.limiter.SetLimit(rate.Inf)
		u.limiter.SetBurst(0)
		return
	}
	u.limiter.SetLimit(rate.Limit(rps))
	burst := int(math.Ceil(rps))
	if burst < 1 {
		burst = 1
	}
	u.limiter.SetBurst(burst)
}

// poissonArrival samples exponential inter-arrival times to approximate a Poisson process.
type poissonArrival struct {
	mu     sync.Mutex
	rate   float64
	sample func() float64
}

func (p *poissonArrival) Wait(ctx context.Context) error {
	delay := p.nextDelay()
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (p *poissonArrival) SetRate(rps float64) {
	if p == nil {
		return
	}
	if rps < 0 {
		rps = 0
	}
	p.mu.Lock()
	p.rate = rps
	p.mu.Unlock()
}

func (p *poissonArrival) nextDelay() time.Duration {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.rate <= 0 || p.sample == nil {
		return 0
	}

	value := p.sample()
	delay := float64(time.Second) * value / p.rate
	if delay > math.MaxInt64 {
		delay = math.MaxInt64
	}
	return time.Duration(delay)
}
