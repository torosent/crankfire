package metrics

import (
	"fmt"
	"sync"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
)

const (
	// minElapsedForRPS is the minimum elapsed time required before calculating RPS.
	// This prevents unrealistic RPS values during the initial moments of a test
	// (e.g., 10 requests completing in 1ms would show as 10,000 RPS).
	minElapsedForRPS = 100 * time.Millisecond
)

type Collector struct {
	mu        sync.Mutex
	total     *statsBucket
	endpoints map[string]*statsBucket
	startTime time.Time
	started   bool
}

// RequestMetadata annotates a measurement with optional labels.
type RequestMetadata struct {
	Endpoint string
}

// EndpointStats represents aggregated metrics for a logical bucket (overall or per-endpoint).
type EndpointStats struct {
	Total          int64         `json:"total"`
	Successes      int64         `json:"successes"`
	Failures       int64         `json:"failures"`
	MinLatency     time.Duration `json:"-"`
	MaxLatency     time.Duration `json:"-"`
	MeanLatency    time.Duration `json:"-"`
	P50Latency     time.Duration `json:"-"`
	P90Latency     time.Duration `json:"-"`
	P99Latency     time.Duration `json:"-"`
	RequestsPerSec float64       `json:"requests_per_sec"`

	// JSON-friendly millisecond fields.
	MinLatencyMs  float64        `json:"min_latency_ms"`
	MaxLatencyMs  float64        `json:"max_latency_ms"`
	MeanLatencyMs float64        `json:"mean_latency_ms"`
	P50LatencyMs  float64        `json:"p50_latency_ms"`
	P90LatencyMs  float64        `json:"p90_latency_ms"`
	P99LatencyMs  float64        `json:"p99_latency_ms"`
	Errors        map[string]int `json:"errors,omitempty"`
}

// Stats represents aggregated metrics, including optional breakdowns.
type Stats struct {
	EndpointStats
	Duration   time.Duration            `json:"-"`
	DurationMs float64                  `json:"duration_ms"`
	Endpoints  map[string]EndpointStats `json:"endpoints,omitempty"`
}

// NewCollector allocates a Collector.
func NewCollector() *Collector {
	return &Collector{
		total:     newStatsBucket(),
		endpoints: make(map[string]*statsBucket),
	}
}

// Start marks the beginning of the test for accurate RPS calculation.
// This should be called when the test actually begins, not when the Collector is created.
func (c *Collector) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.started {
		c.startTime = time.Now()
		c.started = true
	}
}

// RecordRequest records a single request's latency and error state.
func (c *Collector) RecordRequest(latency time.Duration, err error, meta *RequestMetadata) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.total.record(latency, err)
	if meta != nil && meta.Endpoint != "" {
		bucket, ok := c.endpoints[meta.Endpoint]
		if !ok {
			bucket = newStatsBucket()
			c.endpoints[meta.Endpoint] = bucket
		}
		bucket.record(latency, err)
	}
}

// Stats computes and returns current aggregated statistics.
// The elapsed parameter is only used if Start() has not been called.
// If Start() was called, we use the actual time since Start() for accurate RPS.
func (c *Collector) Stats(elapsed time.Duration) Stats {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If Start() was called, use actual elapsed time since then.
	// Otherwise use the provided elapsed (for tests or cases where Start() wasn't called).
	actualElapsed := elapsed
	if c.started {
		actualElapsed = time.Since(c.startTime)
	}

	summary := c.total.snapshot(actualElapsed)
	endpointSnaps := make(map[string]EndpointStats, len(c.endpoints))
	for name, bucket := range c.endpoints {
		endpointSnaps[name] = bucket.snapshot(actualElapsed)
	}

	return Stats{
		EndpointStats: summary,
		Duration:      actualElapsed,
		DurationMs:    float64(actualElapsed) / float64(time.Millisecond),
		Endpoints:     endpointSnaps,
	}
}

// GetErrorBreakdown returns a map of error types to their counts.
func (c *Collector) GetErrorBreakdown() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return copyErrors(c.total.errorsByType)
}

type statsBucket struct {
	hist         *hdrhistogram.Histogram
	successes    int64
	failures     int64
	minLatency   time.Duration
	maxLatency   time.Duration
	sumLatency   time.Duration
	errorsByType map[string]int64
}

func newStatsBucket() *statsBucket {
	return &statsBucket{
		hist:         hdrhistogram.New(1, 60_000_000, 3),
		errorsByType: make(map[string]int64),
	}
}

func (b *statsBucket) record(latency time.Duration, err error) {
	if latency > 0 {
		us := latency.Microseconds()
		if us < b.hist.LowestTrackableValue() {
			us = b.hist.LowestTrackableValue()
		}
		if us > b.hist.HighestTrackableValue() {
			us = b.hist.HighestTrackableValue()
		}
		_ = b.hist.RecordValue(us)
	}
	b.sumLatency += latency

	if b.minLatency == 0 || latency < b.minLatency {
		b.minLatency = latency
	}
	if latency > b.maxLatency {
		b.maxLatency = latency
	}

	if err == nil {
		b.successes++
		return
	}
	b.failures++
	errorType := fmt.Sprintf("%T", err)
	if len(errorType) > 30 {
		errorType = errorType[len(errorType)-30:]
	}
	b.errorsByType[errorType]++
}

func (b *statsBucket) snapshot(elapsed time.Duration) EndpointStats {
	total := b.successes + b.failures
	stats := EndpointStats{
		Total:      total,
		Successes:  b.successes,
		Failures:   b.failures,
		MinLatency: b.minLatency,
		MaxLatency: b.maxLatency,
	}

	if total > 0 {
		stats.MeanLatency = time.Duration(int64(b.sumLatency) / total)
	}

	if b.hist.TotalCount() > 0 {
		stats.P50Latency = time.Duration(b.hist.ValueAtQuantile(50)) * time.Microsecond
		stats.P90Latency = time.Duration(b.hist.ValueAtQuantile(90)) * time.Microsecond
		stats.P99Latency = time.Duration(b.hist.ValueAtQuantile(99)) * time.Microsecond
	}

	stats.MinLatencyMs = float64(stats.MinLatency) / float64(time.Millisecond)
	stats.MaxLatencyMs = float64(stats.MaxLatency) / float64(time.Millisecond)
	stats.MeanLatencyMs = float64(stats.MeanLatency) / float64(time.Millisecond)
	stats.P50LatencyMs = float64(stats.P50Latency) / float64(time.Millisecond)
	stats.P90LatencyMs = float64(stats.P90Latency) / float64(time.Millisecond)
	stats.P99LatencyMs = float64(stats.P99Latency) / float64(time.Millisecond)

	// Only calculate RPS if enough time has elapsed to avoid unrealistic values.

	elapsedSeconds := elapsed.Seconds()
	if elapsed >= minElapsedForRPS && total > 0 && elapsedSeconds > 0 {
		stats.RequestsPerSec = float64(total) / elapsedSeconds
	}

	if len(b.errorsByType) > 0 {
		stats.Errors = copyErrors(b.errorsByType)
	}

	return stats
}

func copyErrors(src map[string]int64) map[string]int {
	if len(src) == 0 {
		return nil
	}
	result := make(map[string]int, len(src))
	for k, v := range src {
		result[k] = int(v)
	}
	return result
}
