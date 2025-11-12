package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/torosent/crankfire/internal/metrics"
)

// PrintReport outputs a human-readable summary report.
func PrintReport(w io.Writer, stats metrics.Stats) {
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
	if len(stats.Errors) > 0 {
		fmt.Fprintln(w, "\nErrors:")
		for k, v := range stats.Errors {
			fmt.Fprintf(w, "  %s: %d\n", k, v)
		}
	}
}

// PrintJSONReport outputs a JSON-formatted report.
func PrintJSONReport(w io.Writer, stats metrics.Stats) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(stats)
}
