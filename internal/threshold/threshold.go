package threshold

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/torosent/crankfire/internal/metrics"
)

// Threshold represents a performance assertion that can pass or fail.
type Threshold struct {
	Metric    string  // e.g., "http_req_duration", "http_req_failed"
	Aggregate string  // e.g., "p95", "p99", "avg", "max", "rate"
	Operator  string  // e.g., "<", "<=", ">", ">=", "=="
	Value     float64 // The threshold value to compare against
	Raw       string  // Original threshold string for display
}

// Result represents the outcome of evaluating a threshold.
type Result struct {
	Threshold Threshold
	Actual    float64
	Pass      bool
	Message   string
}

// Evaluator evaluates thresholds against collected metrics.
type Evaluator struct {
	thresholds []Threshold
}

// NewEvaluator creates a new threshold evaluator.
func NewEvaluator(thresholds []Threshold) *Evaluator {
	return &Evaluator{
		thresholds: thresholds,
	}
}

// Evaluate checks all thresholds against the provided stats.
func (e *Evaluator) Evaluate(stats metrics.Stats) []Result {
	if len(e.thresholds) == 0 {
		return nil
	}

	results := make([]Result, 0, len(e.thresholds))
	for _, t := range e.thresholds {
		result := e.evaluateOne(t, stats)
		results = append(results, result)
	}
	return results
}

func (e *Evaluator) evaluateOne(t Threshold, stats metrics.Stats) Result {
	actual, err := extractMetricValue(t, stats)
	if err != nil {
		return Result{
			Threshold: t,
			Actual:    0,
			Pass:      false,
			Message:   fmt.Sprintf("error: %v", err),
		}
	}

	pass := compareValues(actual, t.Operator, t.Value)
	status := "✓"
	if !pass {
		status = "✗"
	}

	message := fmt.Sprintf("%s %s: %.2f %s %.2f", status, t.Raw, actual, t.Operator, t.Value)
	return Result{
		Threshold: t,
		Actual:    actual,
		Pass:      pass,
		Message:   message,
	}
}

// Parse parses a threshold string into a Threshold struct.
// Supported formats:
// - "http_req_duration:p95 < 500"     (latency percentile in ms)
// - "http_req_duration:avg < 200"     (average latency in ms)
// - "http_req_duration:max < 1000"    (max latency in ms)
// - "http_req_failed:rate < 0.01"     (failure rate as decimal)
// - "http_req_failed:count < 10"      (failure count)
// - "http_requests:rate > 100"        (requests per second)
func Parse(s string) (Threshold, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Threshold{}, fmt.Errorf("empty threshold string")
	}

	// Pattern: metric:aggregate operator value
	// e.g., "http_req_duration:p95 < 500"
	pattern := regexp.MustCompile(`^([a-z_]+):([a-z0-9]+)\s*([<>=!]+)\s*([0-9.]+)$`)
	matches := pattern.FindStringSubmatch(s)
	if matches == nil {
		return Threshold{}, fmt.Errorf("invalid threshold format: %q (expected format: metric:aggregate operator value, e.g., 'http_req_duration:p95 < 500')", s)
	}

	metric := matches[1]
	aggregate := matches[2]
	operator := matches[3]
	valueStr := matches[4]

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return Threshold{}, fmt.Errorf("invalid threshold value %q: %v", valueStr, err)
	}

	// Validate metric
	if !isValidMetric(metric) {
		return Threshold{}, fmt.Errorf("unsupported metric: %q (supported: http_req_duration, http_req_failed, http_requests)", metric)
	}

	// Validate aggregate
	if !isValidAggregate(aggregate) {
		return Threshold{}, fmt.Errorf("unsupported aggregate: %q (supported: p50, p90, p95, p99, avg, min, max, rate, count)", aggregate)
	}

	// Validate operator
	if !isValidOperator(operator) {
		return Threshold{}, fmt.Errorf("unsupported operator: %q (supported: <, <=, >, >=, ==)", operator)
	}

	return Threshold{
		Metric:    metric,
		Aggregate: aggregate,
		Operator:  operator,
		Value:     value,
		Raw:       s,
	}, nil
}

// ParseMultiple parses multiple threshold strings.
func ParseMultiple(thresholds []string) ([]Threshold, error) {
	if len(thresholds) == 0 {
		return nil, nil
	}

	result := make([]Threshold, 0, len(thresholds))
	var errors []string

	for i, s := range thresholds {
		t, err := Parse(s)
		if err != nil {
			errors = append(errors, fmt.Sprintf("threshold[%d]: %v", i, err))
			continue
		}
		result = append(result, t)
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("threshold parsing errors: %s", strings.Join(errors, "; "))
	}

	return result, nil
}

func isValidMetric(metric string) bool {
	valid := []string{"http_req_duration", "http_req_failed", "http_requests"}
	for _, v := range valid {
		if metric == v {
			return true
		}
	}
	return false
}

func isValidAggregate(aggregate string) bool {
	valid := []string{"p50", "p90", "p95", "p99", "avg", "min", "max", "rate", "count"}
	for _, v := range valid {
		if aggregate == v {
			return true
		}
	}
	return false
}

func isValidOperator(operator string) bool {
	valid := []string{"<", "<=", ">", ">=", "=="}
	for _, v := range valid {
		if operator == v {
			return true
		}
	}
	return false
}

func extractMetricValue(t Threshold, stats metrics.Stats) (float64, error) {
	switch t.Metric {
	case "http_req_duration":
		return extractLatencyMetric(t.Aggregate, stats)
	case "http_req_failed":
		return extractFailureMetric(t.Aggregate, stats)
	case "http_requests":
		return extractRequestMetric(t.Aggregate, stats)
	default:
		return 0, fmt.Errorf("unknown metric: %s", t.Metric)
	}
}

func extractLatencyMetric(aggregate string, stats metrics.Stats) (float64, error) {
	switch aggregate {
	case "p50":
		return stats.P50LatencyMs, nil
	case "p90":
		return stats.P90LatencyMs, nil
	case "p95":
		// Approximate p95 from p90 and p99
		return (stats.P90LatencyMs + stats.P99LatencyMs) / 2, nil
	case "p99":
		return stats.P99LatencyMs, nil
	case "avg", "mean":
		return stats.MeanLatencyMs, nil
	case "min":
		return stats.MinLatencyMs, nil
	case "max":
		return stats.MaxLatencyMs, nil
	default:
		return 0, fmt.Errorf("unsupported aggregate %q for http_req_duration", aggregate)
	}
}

func extractFailureMetric(aggregate string, stats metrics.Stats) (float64, error) {
	switch aggregate {
	case "count":
		return float64(stats.Failures), nil
	case "rate":
		if stats.Total == 0 {
			return 0, nil
		}
		return float64(stats.Failures) / float64(stats.Total), nil
	default:
		return 0, fmt.Errorf("unsupported aggregate %q for http_req_failed (use 'count' or 'rate')", aggregate)
	}
}

func extractRequestMetric(aggregate string, stats metrics.Stats) (float64, error) {
	switch aggregate {
	case "count":
		return float64(stats.Total), nil
	case "rate":
		return stats.RequestsPerSec, nil
	default:
		return 0, fmt.Errorf("unsupported aggregate %q for http_requests (use 'count' or 'rate')", aggregate)
	}
}

func compareValues(actual float64, operator string, expected float64) bool {
	// Handle floating point comparison with small epsilon
	epsilon := 1e-9

	switch operator {
	case "<":
		return actual < expected
	case "<=":
		return actual <= expected || math.Abs(actual-expected) < epsilon
	case ">":
		return actual > expected
	case ">=":
		return actual >= expected || math.Abs(actual-expected) < epsilon
	case "==":
		return math.Abs(actual-expected) < epsilon
	default:
		return false
	}
}
