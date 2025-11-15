package metrics_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/metrics"
)

type testError struct{}

func (e *testError) Error() string { return "testError" }

func TestStatsJSONIncludesErrorsAndDuration(t *testing.T) {
	c := metrics.NewCollector()
	c.RecordRequest(10*time.Millisecond, nil, nil)
	c.RecordRequest(15*time.Millisecond, errors.New("boom"), nil)
	c.RecordRequest(20*time.Millisecond, &testError{}, nil)

	elapsed := 150 * time.Millisecond
	stats := c.Stats(elapsed)
	if stats.Duration != elapsed {
		t.Fatalf("expected Duration %s got %s", elapsed, stats.Duration)
	}
	if stats.RequestsPerSec == 0 {
		// For 3 requests over 150ms we expect >0 RPS
		t.Fatalf("expected non-zero RequestsPerSec")
	}
	if len(stats.Errors) == 0 {
		t.Fatalf("expected error breakdown")
	}
	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := parsed["duration_ms"]; !ok {
		t.Errorf("missing duration_ms in JSON")
	}
	if _, ok := parsed["errors"]; !ok {
		t.Errorf("missing errors in JSON")
	}
}
