// Package metrics provides real-time metrics collection and aggregation for load testing.
//
// The metrics package collects latency measurements, success/failure counts, and
// protocol-specific metrics during load test execution. It uses high-performance
// data structures optimized for concurrent access.
//
// # Collector
//
// The central [Collector] type aggregates metrics from all request workers:
//
//	collector := metrics.NewCollector()
//	collector.Start() // Mark test start for accurate RPS calculation
//
//	// Record a request
//	collector.RecordRequest(latency, err, &metrics.RequestMetadata{
//		Protocol:   "http",
//		Endpoint:   "/api/users",
//		StatusCode: "200",
//	})
//
//	// Get aggregated statistics
//	stats := collector.Stats(elapsed)
//
// # Statistics
//
// The [Stats] type provides comprehensive metrics including:
//   - Request counts (total, successes, failures)
//   - Latency percentiles (P50, P90, P95, P99)
//   - Requests per second (RPS)
//   - Per-endpoint breakdowns
//   - Protocol-specific custom metrics
//
// # Time-Series Data
//
// Use [Collector.Snapshot] for time-series data collection:
//
//	// Called periodically (e.g., every second)
//	collector.Snapshot()
//
//	// Get history for charting
//	history := collector.History()
//
// # Thread Safety
//
// The Collector uses sharded locks and atomic operations to minimize contention
// under high concurrency. It's safe to call RecordRequest from multiple goroutines.
//
// # Protocol Metrics
//
// Custom metrics can be attached via [RequestMetadata.CustomMetrics]:
//
//	meta := &metrics.RequestMetadata{
//		Protocol: "websocket",
//		CustomMetrics: map[string]interface{}{
//			"messages_sent":     10,
//			"messages_received": 8,
//			"bytes_sent":        1024,
//		},
//	}
//
// These are aggregated per-protocol in [Stats.ProtocolMetrics].
package metrics
