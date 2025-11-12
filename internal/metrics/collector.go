package metrics

import (
	"fmt"
	"sync"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
)

// Collector records per-request metrics in a thread-safe manner.
type Collector struct {
	mu           sync.Mutex
	hist         *hdrhistogram.Histogram
	successes    int64
	failures     int64
	minLatency   time.Duration
	maxLatency   time.Duration
	sumLatency   time.Duration
	errorsByType map[string]int64
	start        time.Time
}

// Stats represents aggregated metrics.
type Stats struct {
	Total          int64         `json:"total"`
	Successes      int64         `json:"successes"`
	Failures       int64         `json:"failures"`
	MinLatency     time.Duration `json:"-"`
	MaxLatency     time.Duration `json:"-"`
	MeanLatency    time.Duration `json:"-"`
	P50Latency     time.Duration `json:"-"`
	P90Latency     time.Duration `json:"-"`
	P99Latency     time.Duration `json:"-"`
	Duration       time.Duration `json:"-"`
	RequestsPerSec float64       `json:"requests_per_sec"`

	// JSON-friendly millisecond fields.
	MinLatencyMs  float64            `json:"min_latency_ms"`
	MaxLatencyMs  float64            `json:"max_latency_ms"`
	MeanLatencyMs float64            `json:"mean_latency_ms"`
	P50LatencyMs  float64            `json:"p50_latency_ms"`
	P90LatencyMs  float64            `json:"p90_latency_ms"`
	P99LatencyMs  float64            `json:"p99_latency_ms"`
	DurationMs    float64            `json:"duration_ms"`
	Errors        map[string]int     `json:"errors,omitempty"`
}

func NewCollector() *Collector {
	// Track latencies from 1µs up to 60s with 3 significant figures.
	h := hdrhistogram.New(1, 60_000_000, 3)
	return &Collector{
		hist:         h,
		errorsByType: make(map[string]int64),
		start:        time.Now(),
	}
}

// RecordRequest records a single request's latency and error state.
func (c *Collector) RecordRequest(latency time.Duration, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if latency > 0 {
		us := latency.Microseconds()
		if us < c.hist.LowestTrackableValue() {
			us = c.hist.LowestTrackableValue()
		}
		if us > c.hist.HighestTrackableValue() {
			us = c.hist.HighestTrackableValue()
		}
		_ = c.hist.RecordValue(us)
	}
	c.sumLatency += latency

	if c.minLatency == 0 || latency < c.minLatency {
		c.minLatency = latency
	}
	if latency > c.maxLatency {
		c.maxLatency = latency
	}

	if err == nil {
		c.successes++
	} else {
		c.failures++
		errorType := fmt.Sprintf("%T", err)
		if len(errorType) > 30 {
			errorType = errorType[len(errorType)-30:]
		}
		c.errorsByType[errorType]++
	}
}

// Stats computes and returns current aggregated statistics.
func (c *Collector) Stats(elapsed time.Duration) Stats {
	c.mu.Lock()
	defer c.mu.Unlock()

	total := c.successes + c.failures
	stats := Stats{
		Total:      total,
		Successes:  c.successes,
		Failures:   c.failures,
		MinLatency: c.minLatency,
		MaxLatency: c.maxLatency,
	}

	if total > 0 {
		stats.MeanLatency = time.Duration(int64(c.sumLatency) / total)
	}

	if c.hist.TotalCount() > 0 {
		stats.P50Latency = time.Duration(c.hist.ValueAtQuantile(50)) * time.Microsecond
		stats.P90Latency = time.Duration(c.hist.ValueAtQuantile(90)) * time.Microsecond
		stats.P99Latency = time.Duration(c.hist.ValueAtQuantile(99)) * time.Microsecond
	}

	stats.MinLatencyMs = float64(stats.MinLatency) / float64(time.Millisecond)
	stats.MaxLatencyMs = float64(stats.MaxLatency) / float64(time.Millisecond)
	stats.MeanLatencyMs = float64(stats.MeanLatency) / float64(time.Millisecond)
	stats.P50LatencyMs = float64(stats.P50Latency) / float64(time.Millisecond)
	stats.P90LatencyMs = float64(stats.P90Latency) / float64(time.Millisecond)
	stats.P99LatencyMs = float64(stats.P99Latency) / float64(time.Millisecond)

	stats.Duration = elapsed
	stats.DurationMs = float64(elapsed) / float64(time.Millisecond)
	if elapsed > 0 && total > 0 {
		stats.RequestsPerSec = float64(total) / elapsed.Seconds()
	}

	if len(c.errorsByType) > 0 {
		stats.Errors = make(map[string]int, len(c.errorsByType))
		for k, v := range c.errorsByType {
			stats.Errors[k] = int(v)
		}
	}

	return stats
}

// percentile handled by histogram now; retained as stub for compatibility.
func percentile(_ []time.Duration, _ float64) time.Duration { return 0 }

// GetErrorBreakdown returns a map of error types to their counts.
func (c *Collector) GetErrorBreakdown() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := make(map[string]int)
	for k, v := range c.errorsByType {
		result[k] = int(v)
	}
	return result
}
