package output

import (
	"fmt"
	"io"
	"sort"
	"sync/atomic"
	"time"

	"github.com/torosent/crankfire/internal/metrics"
)

// ProgressReporter displays real-time progress updates.
type ProgressReporter struct {
	collector *metrics.Collector
	ticker    *time.Ticker
	done      chan struct{}
	finished  chan struct{}
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
		finished:  make(chan struct{}),
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
		<-p.finished
	}
}

func (p *ProgressReporter) run() {
	defer close(p.finished)
	for {
		select {
		case <-p.ticker.C:
			elapsed := time.Since(p.start)
			stats := p.collector.Stats(elapsed)
			line := fmt.Sprintf("\rRequests: %d | Successes: %d | Failures: %d | RPS: %.1f",
				stats.Total, stats.Successes, stats.Failures, stats.RequestsPerSec)
			if name, ep, ok := topEndpointSnapshot(stats); ok && stats.Total > 0 {
				share := (float64(ep.Total) / float64(stats.Total)) * 100
				line += fmt.Sprintf(" | Top Endpoint: %s (%.0f%%, P99 %.1fms)", name, share, ep.P99LatencyMs)
			}
			if len(stats.ProtocolMetrics) > 0 {
				for protocol := range stats.ProtocolMetrics {
					line += fmt.Sprintf(" | %s", protocol)
					break // Only show first protocol to keep line concise
				}
			}
			fmt.Fprint(p.writer, line)
		case <-p.done:
			return
		}
	}
}

func topEndpointSnapshot(stats metrics.Stats) (string, metrics.EndpointStats, bool) {
	if len(stats.Endpoints) == 0 {
		return "", metrics.EndpointStats{}, false
	}
	names := make([]string, 0, len(stats.Endpoints))
	for name := range stats.Endpoints {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return stats.Endpoints[names[i]].Total > stats.Endpoints[names[j]].Total
	})
	name := names[0]
	return name, stats.Endpoints[name], true
}
