package threshold

import (
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/metrics"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      Threshold
		wantError bool
	}{
		{
			name:  "valid p95 latency threshold",
			input: "http_req_duration:p95 < 500",
			want: Threshold{
				Metric:    "http_req_duration",
				Aggregate: "p95",
				Operator:  "<",
				Value:     500,
				Raw:       "http_req_duration:p95 < 500",
			},
			wantError: false,
		},
		{
			name:  "valid failure rate threshold",
			input: "http_req_failed:rate < 0.01",
			want: Threshold{
				Metric:    "http_req_failed",
				Aggregate: "rate",
				Operator:  "<",
				Value:     0.01,
				Raw:       "http_req_failed:rate < 0.01",
			},
			wantError: false,
		},
		{
			name:  "valid p99 latency with <=",
			input: "http_req_duration:p99 <= 1000",
			want: Threshold{
				Metric:    "http_req_duration",
				Aggregate: "p99",
				Operator:  "<=",
				Value:     1000,
				Raw:       "http_req_duration:p99 <= 1000",
			},
			wantError: false,
		},
		{
			name:  "valid requests rate threshold with >",
			input: "http_requests:rate > 100",
			want: Threshold{
				Metric:    "http_requests",
				Aggregate: "rate",
				Operator:  ">",
				Value:     100,
				Raw:       "http_requests:rate > 100",
			},
			wantError: false,
		},
		{
			name:  "valid avg latency",
			input: "http_req_duration:avg < 200",
			want: Threshold{
				Metric:    "http_req_duration",
				Aggregate: "avg",
				Operator:  "<",
				Value:     200,
				Raw:       "http_req_duration:avg < 200",
			},
			wantError: false,
		},
		{
			name:      "empty string",
			input:     "",
			wantError: true,
		},
		{
			name:      "invalid format - missing operator",
			input:     "http_req_duration:p95 500",
			wantError: true,
		},
		{
			name:      "invalid metric",
			input:     "invalid_metric:p95 < 500",
			wantError: true,
		},
		{
			name:      "invalid aggregate",
			input:     "http_req_duration:p85 < 500",
			wantError: true,
		},
		{
			name:      "invalid operator",
			input:     "http_req_duration:p95 << 500",
			wantError: true,
		},
		{
			name:      "invalid value - not a number",
			input:     "http_req_duration:p95 < abc",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("Parse() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError {
				if got.Metric != tt.want.Metric {
					t.Errorf("Parse() Metric = %v, want %v", got.Metric, tt.want.Metric)
				}
				if got.Aggregate != tt.want.Aggregate {
					t.Errorf("Parse() Aggregate = %v, want %v", got.Aggregate, tt.want.Aggregate)
				}
				if got.Operator != tt.want.Operator {
					t.Errorf("Parse() Operator = %v, want %v", got.Operator, tt.want.Operator)
				}
				if got.Value != tt.want.Value {
					t.Errorf("Parse() Value = %v, want %v", got.Value, tt.want.Value)
				}
				if got.Raw != tt.want.Raw {
					t.Errorf("Parse() Raw = %v, want %v", got.Raw, tt.want.Raw)
				}
			}
		})
	}
}

func TestParseMultiple(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		wantCount int
		wantError bool
	}{
		{
			name: "multiple valid thresholds",
			input: []string{
				"http_req_duration:p95 < 500",
				"http_req_failed:rate < 0.01",
				"http_requests:rate > 100",
			},
			wantCount: 3,
			wantError: false,
		},
		{
			name:      "empty slice",
			input:     []string{},
			wantCount: 0,
			wantError: false,
		},
		{
			name: "one valid, one invalid",
			input: []string{
				"http_req_duration:p95 < 500",
				"invalid threshold",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMultiple(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("ParseMultiple() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && len(got) != tt.wantCount {
				t.Errorf("ParseMultiple() returned %d thresholds, want %d", len(got), tt.wantCount)
			}
		})
	}
}

func TestEvaluator(t *testing.T) {
	// Create sample stats
	stats := metrics.Stats{
		EndpointStats: metrics.EndpointStats{
			Total:          1000,
			Successes:      980,
			Failures:       20,
			MinLatency:     10 * time.Millisecond,
			MaxLatency:     500 * time.Millisecond,
			MeanLatency:    100 * time.Millisecond,
			P50Latency:     80 * time.Millisecond,
			P90Latency:     200 * time.Millisecond,
			P95Latency:     300 * time.Millisecond,
			P99Latency:     400 * time.Millisecond,
			MinLatencyMs:   10,
			MaxLatencyMs:   500,
			MeanLatencyMs:  100,
			P50LatencyMs:   80,
			P90LatencyMs:   200,
			P95LatencyMs:   300,
			P99LatencyMs:   400,
			RequestsPerSec: 100,
		},
		Duration: 10 * time.Second,
	}

	tests := []struct {
		name       string
		thresholds []string
		wantPass   []bool
	}{
		{
			name: "all thresholds pass",
			thresholds: []string{
				"http_req_duration:p99 < 500",
				"http_req_failed:rate < 0.05",
				"http_requests:rate > 50",
			},
			wantPass: []bool{true, true, true},
		},
		{
			name: "some thresholds fail",
			thresholds: []string{
				"http_req_duration:p99 < 300",
				"http_req_failed:rate < 0.01",
				"http_requests:rate > 50",
			},
			wantPass: []bool{false, false, true},
		},
		{
			name: "latency percentiles",
			thresholds: []string{
				"http_req_duration:p50 < 100",
				"http_req_duration:p90 < 250",
				"http_req_duration:p99 < 450",
			},
			wantPass: []bool{true, true, true},
		},
		{
			name: "avg and max latency",
			thresholds: []string{
				"http_req_duration:avg < 150",
				"http_req_duration:max < 600",
				"http_req_duration:min > 5",
			},
			wantPass: []bool{true, true, true},
		},
		{
			name: "failure count",
			thresholds: []string{
				"http_req_failed:count < 50",
			},
			wantPass: []bool{true},
		},
		{
			name: "request count",
			thresholds: []string{
				"http_requests:count > 900",
			},
			wantPass: []bool{true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thresholds, err := ParseMultiple(tt.thresholds)
			if err != nil {
				t.Fatalf("ParseMultiple() error = %v", err)
			}

			evaluator := NewEvaluator(thresholds)
			results := evaluator.Evaluate(stats)

			if len(results) != len(tt.wantPass) {
				t.Fatalf("got %d results, want %d", len(results), len(tt.wantPass))
			}

			for i, result := range results {
				if result.Pass != tt.wantPass[i] {
					t.Errorf("threshold[%d] %q: got pass=%v, want %v (actual=%.2f)",
						i, result.Threshold.Raw, result.Pass, tt.wantPass[i], result.Actual)
				}
			}
		})
	}
}

func TestCompareValues(t *testing.T) {
	tests := []struct {
		name     string
		actual   float64
		operator string
		expected float64
		want     bool
	}{
		{"less than true", 50, "<", 100, true},
		{"less than false", 100, "<", 50, false},
		{"less than equal", 100, "<", 100, false},
		{"less than or equal true", 50, "<=", 100, true},
		{"less than or equal equal", 100, "<=", 100, true},
		{"less than or equal false", 150, "<=", 100, false},
		{"greater than true", 150, ">", 100, true},
		{"greater than false", 50, ">", 100, false},
		{"greater than equal", 100, ">", 100, false},
		{"greater than or equal true", 150, ">=", 100, true},
		{"greater than or equal equal", 100, ">=", 100, true},
		{"greater than or equal false", 50, ">=", 100, false},
		{"equal true", 100, "==", 100, true},
		{"equal false", 100, "==", 101, false},
		{"equal with floating point precision", 100.0000000001, "==", 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareValues(tt.actual, tt.operator, tt.expected)
			if got != tt.want {
				t.Errorf("compareValues(%.2f, %s, %.2f) = %v, want %v",
					tt.actual, tt.operator, tt.expected, got, tt.want)
			}
		})
	}
}

func TestExtractMetricValue(t *testing.T) {
	stats := metrics.Stats{
		EndpointStats: metrics.EndpointStats{
			Total:          1000,
			Successes:      950,
			Failures:       50,
			MinLatencyMs:   10.5,
			MaxLatencyMs:   500.25,
			MeanLatencyMs:  100.75,
			P50LatencyMs:   80.5,
			P90LatencyMs:   200.25,
			P95LatencyMs:   300.5,
			P99LatencyMs:   400.5,
			RequestsPerSec: 123.45,
		},
	}

	tests := []struct {
		name      string
		threshold Threshold
		want      float64
		wantError bool
	}{
		{
			name:      "http_req_duration p50",
			threshold: Threshold{Metric: "http_req_duration", Aggregate: "p50"},
			want:      80.5,
		},
		{
			name:      "http_req_duration p90",
			threshold: Threshold{Metric: "http_req_duration", Aggregate: "p90"},
			want:      200.25,
		},
		{
			name:      "http_req_duration p95",
			threshold: Threshold{Metric: "http_req_duration", Aggregate: "p95"},
			want:      300.5,
		},
		{
			name:      "http_req_duration p99",
			threshold: Threshold{Metric: "http_req_duration", Aggregate: "p99"},
			want:      400.5,
		},
		{
			name:      "http_req_duration avg",
			threshold: Threshold{Metric: "http_req_duration", Aggregate: "avg"},
			want:      100.75,
		},
		{
			name:      "http_req_duration min",
			threshold: Threshold{Metric: "http_req_duration", Aggregate: "min"},
			want:      10.5,
		},
		{
			name:      "http_req_duration max",
			threshold: Threshold{Metric: "http_req_duration", Aggregate: "max"},
			want:      500.25,
		},
		{
			name:      "http_req_failed rate",
			threshold: Threshold{Metric: "http_req_failed", Aggregate: "rate"},
			want:      0.05,
		},
		{
			name:      "http_req_failed count",
			threshold: Threshold{Metric: "http_req_failed", Aggregate: "count"},
			want:      50,
		},
		{
			name:      "http_requests rate",
			threshold: Threshold{Metric: "http_requests", Aggregate: "rate"},
			want:      123.45,
		},
		{
			name:      "http_requests count",
			threshold: Threshold{Metric: "http_requests", Aggregate: "count"},
			want:      1000,
		},
		{
			name:      "unsupported metric",
			threshold: Threshold{Metric: "invalid_metric", Aggregate: "p95"},
			wantError: true,
		},
		{
			name:      "unsupported aggregate for metric",
			threshold: Threshold{Metric: "http_req_failed", Aggregate: "p95"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractMetricValue(tt.threshold, stats)
			if (err != nil) != tt.wantError {
				t.Errorf("extractMetricValue() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && got != tt.want {
				t.Errorf("extractMetricValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
