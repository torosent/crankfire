package metrics

import (
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

// DataPoint represents a snapshot of metrics at a specific point in time.
type DataPoint struct {
	Timestamp          time.Time     `json:"timestamp"`
	TotalRequests      int64         `json:"total_requests"`
	SuccessfulRequests int64         `json:"successful_requests"`
	Errors             int64         `json:"errors"`
	CurrentRPS         float64       `json:"current_rps"`
	P50Latency         time.Duration `json:"-"`
	P95Latency         time.Duration `json:"-"`
	P99Latency         time.Duration `json:"-"`
	P50LatencyMs       float64       `json:"p50_latency_ms"`
	P95LatencyMs       float64       `json:"p95_latency_ms"`
	P99LatencyMs       float64       `json:"p99_latency_ms"`
}

type Collector struct {
	mu            sync.Mutex
	total         *statsBucket
	endpoints     map[string]*statsBucket
	customMetrics map[string]map[string]interface{} // protocol -> aggregated metrics
	startTime     time.Time
	started       bool
	history       []DataPoint
	lastSnapshot  snapshotState
}

type snapshotState struct {
	timestamp     time.Time
	totalRequests int64
	successCount  int64
	failureCount  int64
}

// RequestMetadata annotates a measurement with optional labels.
type RequestMetadata struct {
	Endpoint      string
	Protocol      string                 // Protocol used (http, websocket, sse, grpc)
	StatusCode    string                 // Exact status/close code for failures
	CustomMetrics map[string]interface{} // Protocol-specific metrics
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
	P95Latency     time.Duration `json:"-"`
	P99Latency     time.Duration `json:"-"`
	RequestsPerSec float64       `json:"requests_per_sec"`

	// JSON-friendly millisecond fields.
	MinLatencyMs  float64                   `json:"min_latency_ms"`
	MaxLatencyMs  float64                   `json:"max_latency_ms"`
	MeanLatencyMs float64                   `json:"mean_latency_ms"`
	P50LatencyMs  float64                   `json:"p50_latency_ms"`
	P90LatencyMs  float64                   `json:"p90_latency_ms"`
	P95LatencyMs  float64                   `json:"p95_latency_ms"`
	P99LatencyMs  float64                   `json:"p99_latency_ms"`
	StatusBuckets map[string]map[string]int `json:"status_buckets,omitempty"`
}

// Stats represents aggregated metrics, including optional breakdowns.
type Stats struct {
	EndpointStats
	Duration        time.Duration                     `json:"-"`
	DurationMs      float64                           `json:"duration_ms"`
	Endpoints       map[string]EndpointStats          `json:"endpoints,omitempty"`
	ProtocolMetrics map[string]map[string]interface{} `json:"protocol_metrics,omitempty"`
}

// NewCollector allocates a Collector.
func NewCollector() *Collector {
	return &Collector{
		total:         newStatsBucket(),
		endpoints:     make(map[string]*statsBucket),
		customMetrics: make(map[string]map[string]interface{}),
		history:       []DataPoint{},
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

	var endpoint string
	var protocol string
	var statusCode string
	var customMetrics map[string]interface{}
	if meta != nil {
		endpoint = meta.Endpoint
		protocol = meta.Protocol
		statusCode = meta.StatusCode
		customMetrics = meta.CustomMetrics
	}

	c.total.record(latency, err, protocol, statusCode)
	if endpoint != "" {
		bucket, ok := c.endpoints[endpoint]
		if !ok {
			bucket = newStatsBucket()
			c.endpoints[endpoint] = bucket
		}
		bucket.record(latency, err, protocol, statusCode)
	}

	// Aggregate CustomMetrics by protocol
	if protocol != "" && len(customMetrics) > 0 {
		if c.customMetrics[protocol] == nil {
			c.customMetrics[protocol] = make(map[string]interface{})
		}
		for key, value := range customMetrics {
			c.aggregateMetric(protocol, key, value)
		}
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

	// Copy protocol metrics
	protocolMetrics := make(map[string]map[string]interface{}, len(c.customMetrics))
	for protocol, metrics := range c.customMetrics {
		protocolMetrics[protocol] = copyMetrics(metrics)
	}

	return Stats{
		EndpointStats:   summary,
		Duration:        actualElapsed,
		DurationMs:      float64(actualElapsed) / float64(time.Millisecond),
		Endpoints:       endpointSnaps,
		ProtocolMetrics: protocolMetrics,
	}
}

type statsBucket struct {
	hist          *hdrhistogram.Histogram
	successes     int64
	failures      int64
	minLatency    time.Duration
	maxLatency    time.Duration
	sumLatency    time.Duration
	statusBuckets map[string]map[string]int64
}

func newStatsBucket() *statsBucket {
	return &statsBucket{
		hist: hdrhistogram.New(1, 60_000_000, 3),
	}
}

func (b *statsBucket) record(latency time.Duration, err error, protocol, statusCode string) {
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
	if protocol != "" && statusCode != "" {
		b.recordStatus(protocol, statusCode)
	}
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
		stats.P95Latency = time.Duration(b.hist.ValueAtQuantile(95)) * time.Microsecond
		stats.P99Latency = time.Duration(b.hist.ValueAtQuantile(99)) * time.Microsecond
	}

	stats.MinLatencyMs = float64(stats.MinLatency) / float64(time.Millisecond)
	stats.MaxLatencyMs = float64(stats.MaxLatency) / float64(time.Millisecond)
	stats.MeanLatencyMs = float64(stats.MeanLatency) / float64(time.Millisecond)
	stats.P50LatencyMs = float64(stats.P50Latency) / float64(time.Millisecond)
	stats.P90LatencyMs = float64(stats.P90Latency) / float64(time.Millisecond)
	stats.P95LatencyMs = float64(stats.P95Latency) / float64(time.Millisecond)
	stats.P99LatencyMs = float64(stats.P99Latency) / float64(time.Millisecond)

	// Only calculate RPS if enough time has elapsed to avoid unrealistic values.

	elapsedSeconds := elapsed.Seconds()
	if elapsed >= minElapsedForRPS && total > 0 && elapsedSeconds > 0 {
		stats.RequestsPerSec = float64(total) / elapsedSeconds
	}

	if len(b.statusBuckets) > 0 {
		stats.StatusBuckets = copyStatusBuckets(b.statusBuckets)
	}

	return stats
}

func (b *statsBucket) recordStatus(protocol, statusCode string) {
	if b.statusBuckets == nil {
		b.statusBuckets = make(map[string]map[string]int64)
	}
	if b.statusBuckets[protocol] == nil {
		b.statusBuckets[protocol] = make(map[string]int64)
	}
	b.statusBuckets[protocol][statusCode]++
}

func copyStatusBuckets(src map[string]map[string]int64) map[string]map[string]int {
	if len(src) == 0 {
		return nil
	}
	result := make(map[string]map[string]int, len(src))
	for protocol, buckets := range src {
		if len(buckets) == 0 {
			continue
		}
		copied := make(map[string]int, len(buckets))
		for status, count := range buckets {
			copied[status] = int(count)
		}
		result[protocol] = copied
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// aggregateMetric aggregates a custom metric value (assumes caller holds lock)
func (c *Collector) aggregateMetric(protocol, key string, value interface{}) {
	current := c.customMetrics[protocol][key]
	switch v := value.(type) {
	case int:
		if existing, ok := current.(int); ok {
			c.customMetrics[protocol][key] = existing + v
		} else {
			c.customMetrics[protocol][key] = v
		}
	case int64:
		if existing, ok := current.(int64); ok {
			c.customMetrics[protocol][key] = existing + v
		} else {
			c.customMetrics[protocol][key] = v
		}
	case float64:
		if existing, ok := current.(float64); ok {
			c.customMetrics[protocol][key] = existing + v
		} else {
			c.customMetrics[protocol][key] = v
		}
	default:
		// For non-numeric types, just keep the latest value
		c.customMetrics[protocol][key] = v
	}
}

func copyMetrics(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return nil
	}
	result := make(map[string]interface{}, len(src))
	for k, v := range src {
		result[k] = v
	}
	return result
}

// Snapshot records the current state of metrics as a DataPoint and appends it to history.
// This method is thread-safe and can be called periodically to build time-series data.
func (c *Collector) Snapshot() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// Calculate elapsed time since start or last snapshot
	var elapsed time.Duration
	if c.lastSnapshot.timestamp.IsZero() {
		if c.started {
			elapsed = now.Sub(c.startTime)
		} else {
			elapsed = 0
		}
	} else {
		elapsed = now.Sub(c.lastSnapshot.timestamp)
	}

	// Get current totals
	currentTotal := c.total.successes + c.total.failures
	currentSuccess := c.total.successes
	currentFailures := c.total.failures

	// Calculate delta for RPS
	var deltaRequests int64
	if !c.lastSnapshot.timestamp.IsZero() {
		deltaRequests = currentTotal - c.lastSnapshot.totalRequests
	} else {
		deltaRequests = currentTotal
	}

	// Calculate current RPS
	var currentRPS float64
	if elapsed >= minElapsedForRPS && elapsed.Seconds() > 0 {
		currentRPS = float64(deltaRequests) / elapsed.Seconds()
	}

	// Get percentile data
	var p50, p95, p99 time.Duration
	if c.total.hist.TotalCount() > 0 {
		p50 = time.Duration(c.total.hist.ValueAtQuantile(50)) * time.Microsecond
		p95 = time.Duration(c.total.hist.ValueAtQuantile(95)) * time.Microsecond
		p99 = time.Duration(c.total.hist.ValueAtQuantile(99)) * time.Microsecond
	}

	dataPoint := DataPoint{
		Timestamp:          now,
		TotalRequests:      currentTotal,
		SuccessfulRequests: currentSuccess,
		Errors:             currentFailures,
		CurrentRPS:         currentRPS,
		P50Latency:         p50,
		P95Latency:         p95,
		P99Latency:         p99,
		P50LatencyMs:       float64(p50) / float64(time.Millisecond),
		P95LatencyMs:       float64(p95) / float64(time.Millisecond),
		P99LatencyMs:       float64(p99) / float64(time.Millisecond),
	}

	c.history = append(c.history, dataPoint)

	// Update last snapshot state
	c.lastSnapshot = snapshotState{
		timestamp:     now,
		totalRequests: currentTotal,
		successCount:  currentSuccess,
		failureCount:  currentFailures,
	}
}

// History returns a copy of the recorded history data points.
func (c *Collector) History() []DataPoint {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := make([]DataPoint, len(c.history))
	copy(result, c.history)
	return result
}
