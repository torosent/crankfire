package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/threshold"
)

// PrintReport outputs a human-readable summary report.
func PrintReport(w io.Writer, stats metrics.Stats, thresholdResults []threshold.Result) {
	fmt.Fprintln(w, "\n--- Load Test Results ---")
	fmt.Fprintf(w, "Total Requests:    %d\n", stats.Total)
	fmt.Fprintf(w, "Successful:        %d\n", stats.Successes)
	fmt.Fprintf(w, "Failed:            %d\n", stats.Failures)
	fmt.Fprintf(w, "Duration:          %s\n", stats.Duration)
	fmt.Fprintf(w, "Requests/sec:      %.2f\n", stats.RequestsPerSec)
	fmt.Fprintln(w, "\nLatency:")
	fmt.Fprintf(w, "  Min:             %s\n", stats.MinLatency)
	fmt.Fprintf(w, "  Max:             %s\n", stats.MaxLatency)
	fmt.Fprintf(w, "  Mean:            %s\n", stats.MeanLatency)
	fmt.Fprintf(w, "  P50:             %s\n", stats.P50Latency)
	fmt.Fprintf(w, "  P90:             %s\n", stats.P90Latency)
	fmt.Fprintf(w, "  P99:             %s\n", stats.P99Latency)
	if len(stats.StatusBuckets) > 0 {
		fmt.Fprintln(w, "\nStatus Buckets:")
		writeStatusBuckets(w, stats.StatusBuckets, "  ")
	}

	if len(thresholdResults) > 0 {
		fmt.Fprintln(w, "\nThresholds:")
		passCount := 0
		for _, result := range thresholdResults {
			fmt.Fprintf(w, "  %s\n", result.Message)
			if result.Pass {
				passCount++
			}
		}
		fmt.Fprintf(w, "\nThreshold Summary: %d/%d passed\n", passCount, len(thresholdResults))
	}

	if len(stats.Endpoints) > 0 {
		fmt.Fprintln(w, "\nEndpoint Breakdown:")
		names := make([]string, 0, len(stats.Endpoints))
		for name := range stats.Endpoints {
			names = append(names, name)
		}
		sort.Slice(names, func(i, j int) bool {
			return stats.Endpoints[names[i]].Total > stats.Endpoints[names[j]].Total
		})
		for _, name := range names {
			endpoint := stats.Endpoints[name]
			share := 0.0
			if stats.Total > 0 {
				share = (float64(endpoint.Total) / float64(stats.Total)) * 100
			}

			fmt.Fprintf(
				w,
				"  - %s: total=%d (%.1f%%), successes=%d, failures=%d, rps=%.2f, p99=%s\n",
				name,
				endpoint.Total,
				share,
				endpoint.Successes,
				endpoint.Failures,
				endpoint.RequestsPerSec,
				endpoint.P99Latency,
			)
			if len(endpoint.StatusBuckets) > 0 {
				fmt.Fprintln(w, "    Status Buckets:")
				writeStatusBuckets(w, endpoint.StatusBuckets, "      ")
			}
		}
	}

	if len(stats.ProtocolMetrics) > 0 {
		fmt.Fprintln(w, "\nProtocol Metrics:")
		protocols := make([]string, 0, len(stats.ProtocolMetrics))
		for protocol := range stats.ProtocolMetrics {
			protocols = append(protocols, protocol)
		}
		sort.Strings(protocols)
		for _, protocol := range protocols {
			metrics := stats.ProtocolMetrics[protocol]
			fmt.Fprintf(w, "  %s:\n", protocol)
			keys := make([]string, 0, len(metrics))
			for key := range metrics {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				fmt.Fprintf(w, "    %s: %v\n", key, metrics[key])
			}
		}
	}
}

// ThresholdResultJSON represents a threshold result in JSON format.
type ThresholdResultJSON struct {
	Threshold string  `json:"threshold"`
	Metric    string  `json:"metric"`
	Aggregate string  `json:"aggregate"`
	Operator  string  `json:"operator"`
	Expected  float64 `json:"expected"`
	Actual    float64 `json:"actual"`
	Pass      bool    `json:"pass"`
}

// JSONReport wraps stats and threshold results for JSON output.
type JSONReport struct {
	metrics.Stats
	Thresholds *ThresholdSummary `json:"thresholds,omitempty"`
}

// ThresholdSummary contains threshold evaluation results.
type ThresholdSummary struct {
	Total   int                   `json:"total"`
	Passed  int                   `json:"passed"`
	Failed  int                   `json:"failed"`
	Results []ThresholdResultJSON `json:"results"`
}

// PrintJSONReport outputs a JSON-formatted report.
func PrintJSONReport(w io.Writer, stats metrics.Stats, thresholdResults []threshold.Result) error {
	report := JSONReport{
		Stats: stats,
	}

	if len(thresholdResults) > 0 {
		summary := &ThresholdSummary{
			Total:   len(thresholdResults),
			Results: make([]ThresholdResultJSON, len(thresholdResults)),
		}
		for i, tr := range thresholdResults {
			summary.Results[i] = ThresholdResultJSON{
				Threshold: tr.Threshold.Raw,
				Metric:    tr.Threshold.Metric,
				Aggregate: tr.Threshold.Aggregate,
				Operator:  tr.Threshold.Operator,
				Expected:  tr.Threshold.Value,
				Actual:    tr.Actual,
				Pass:      tr.Pass,
			}
			if tr.Pass {
				summary.Passed++
			} else {
				summary.Failed++
			}
		}
		report.Thresholds = summary
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func writeStatusBuckets(w io.Writer, buckets map[string]map[string]int, indent string) {
	rows := metrics.FlattenStatusBuckets(buckets)
	if len(rows) == 0 {
		fmt.Fprintf(w, "%sNone\n", indent)
		return
	}
	for _, row := range rows {
		fmt.Fprintf(
			w,
			"%s%s %s: %d\n",
			indent,
			strings.ToUpper(row.Protocol),
			row.Code,
			row.Count,
		)
	}
}
