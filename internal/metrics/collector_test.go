package metrics_test

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/metrics"
)

func TestCollectorLatencyStats(t *testing.T) {
	c := metrics.NewCollector()

	// Record deterministic latencies.
	c.RecordRequest(10*time.Millisecond, nil, nil)
	c.RecordRequest(20*time.Millisecond, nil, nil)
	c.RecordRequest(30*time.Millisecond, nil, nil)
	c.RecordRequest(40*time.Millisecond, nil, nil)
	c.RecordRequest(50*time.Millisecond, nil, nil)

	stats := c.Stats(0)

	if stats.Total != 5 {
		t.Errorf("expected total 5, got %d", stats.Total)
	}
	if stats.Successes != 5 {
		t.Errorf("expected successes 5, got %d", stats.Successes)
	}
	if stats.Failures != 0 {
		t.Errorf("expected failures 0, got %d", stats.Failures)
	}
	if stats.MinLatency != 10*time.Millisecond {
		t.Errorf("expected min 10ms, got %s", stats.MinLatency)
	}
	if stats.MaxLatency != 50*time.Millisecond {
		t.Errorf("expected max 50ms, got %s", stats.MaxLatency)
	}
	expectedMean := 30 * time.Millisecond
	if stats.MeanLatency != expectedMean {
		t.Errorf("expected mean 30ms, got %s", stats.MeanLatency)
	}
}

func TestPercentilesCalculations(t *testing.T) {
	c := metrics.NewCollector()

	// 100 samples: 1ms, 2ms, ..., 100ms.
	for i := 1; i <= 100; i++ {
		c.RecordRequest(time.Duration(i)*time.Millisecond, nil, nil)
	}

	stats := c.Stats(0)

	// P50 should be around 50ms or 51ms (depends on interpolation).
	if stats.P50Latency < 49*time.Millisecond || stats.P50Latency > 51*time.Millisecond {
		t.Errorf("expected P50 ~50ms, got %s", stats.P50Latency)
	}
	// P90 should be around 90ms or 91ms.
	if stats.P90Latency < 89*time.Millisecond || stats.P90Latency > 91*time.Millisecond {
		t.Errorf("expected P90 ~90ms, got %s", stats.P90Latency)
	}
	// P99 should be around 99ms or 100ms.
	if stats.P99Latency < 98*time.Millisecond || stats.P99Latency > 100*time.Millisecond {
		t.Errorf("expected P99 ~99ms, got %s", stats.P99Latency)
	}
}

func TestJSONReportSchema(t *testing.T) {
	c := metrics.NewCollector()

	c.RecordRequest(15*time.Millisecond, nil, nil)
	c.RecordRequest(25*time.Millisecond, nil, nil)

	stats := c.Stats(100 * time.Millisecond)

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("failed to marshal stats: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	requiredFields := []string{"total", "successes", "failures", "min_latency_ms", "max_latency_ms", "mean_latency_ms", "p50_latency_ms", "p90_latency_ms", "p99_latency_ms", "duration_ms", "requests_per_sec"}
	for _, field := range requiredFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("missing field %q in JSON output", field)
		}
	}
}

func TestConcurrentRecording(t *testing.T) {
	c := metrics.NewCollector()

	var wg sync.WaitGroup
	workers := 10
	recordsPerWorker := 100

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < recordsPerWorker; j++ {
				c.RecordRequest(time.Millisecond, nil, nil)
			}
		}()
	}
	wg.Wait()

	stats := c.Stats(0)
	expected := workers * recordsPerWorker
	if stats.Total != int64(expected) {
		t.Errorf("expected total %d, got %d", expected, stats.Total)
	}
}

func TestEndpointBreakdown(t *testing.T) {
	c := metrics.NewCollector()
	c.RecordRequest(10*time.Millisecond, nil, &metrics.RequestMetadata{Endpoint: "users"})
	c.RecordRequest(20*time.Millisecond, nil, &metrics.RequestMetadata{Endpoint: "users"})
	c.RecordRequest(15*time.Millisecond, nil, &metrics.RequestMetadata{Endpoint: "orders"})

	stats := c.Stats(2 * time.Second)
	if len(stats.Endpoints) != 2 {
		t.Fatalf("expected 2 endpoint stats, got %d", len(stats.Endpoints))
	}
	users := stats.Endpoints["users"]
	if users.Total != 2 {
		t.Fatalf("expected users total 2, got %d", users.Total)
	}
	if users.P50LatencyMs == 0 {
		t.Fatalf("expected percentile calculations for users endpoint")
	}
	if users.RequestsPerSec <= 0 {
		t.Fatalf("expected users RPS to be > 0")
	}
}
