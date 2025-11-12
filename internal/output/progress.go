package output

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/torosent/crankfire/internal/metrics"
)

// ProgressReporter displays real-time progress updates.
type ProgressReporter struct {
	collector *metrics.Collector
	ticker    *time.Ticker
	done      chan struct{}
	writer    io.Writer
	active    int32
	start     time.Time
}

// NewProgressReporter creates a progress reporter that updates at the given interval.
func NewProgressReporter(collector *metrics.Collector, interval time.Duration, writer io.Writer) *ProgressReporter {
	if writer == nil {
		writer = io.Discard
	}
	return &ProgressReporter{
		collector: collector,
		ticker:    time.NewTicker(interval),
		done:      make(chan struct{}),
		writer:    writer,
		start:     time.Now(),
	}
}

// Start begins displaying progress updates in a background goroutine.
func (p *ProgressReporter) Start() {
	if !atomic.CompareAndSwapInt32(&p.active, 0, 1) {
		return // already running
	}
	go p.run()
}

// Stop halts progress updates.
func (p *ProgressReporter) Stop() {
	if atomic.CompareAndSwapInt32(&p.active, 1, 0) {
		close(p.done)
		p.ticker.Stop()
	}
}

func (p *ProgressReporter) run() {
	for {
		select {
		case <-p.ticker.C:
			elapsed := time.Since(p.start)
			stats := p.collector.Stats(elapsed)
			fmt.Fprintf(p.writer, "\rRequests: %d | Successes: %d | Failures: %d | RPS: %.1f",
				stats.Total, stats.Successes, stats.Failures, stats.RequestsPerSec)
		case <-p.done:
			return
		}
	}
}
