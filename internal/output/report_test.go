package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/metrics"
)

func TestPrintReportBasic(t *testing.T) {
	stats := metrics.Stats{
		EndpointStats: metrics.EndpointStats{
			Total:          100,
			Successes:      95,
			Failures:       5,
			RequestsPerSec: 50.0,
		},
		Duration: 2 * time.Second,
	}

	var buf bytes.Buffer
	PrintReport(&buf, stats)

	output := buf.String()
	if !strings.Contains(output, "Total Requests") {
		t.Errorf("Expected total requests in output")
	}
	if !strings.Contains(output, "95") {
		t.Errorf("Expected successes in output")
	}
}

func TestPrintReportIncludesProtocolMetrics(t *testing.T) {
	stats := metrics.Stats{
		EndpointStats: metrics.EndpointStats{
			Total:          100,
			Successes:      100,
			RequestsPerSec: 50.0,
		},
		Duration: 2 * time.Second,
		ProtocolMetrics: map[string]map[string]interface{}{
			"websocket": {
				"messages_sent": int64(500),
				"bytes_sent":    int64(102400),
			},
			"grpc": {
				"status_code": "OK",
			},
		},
	}

	var buf bytes.Buffer
	PrintReport(&buf, stats)

	output := buf.String()
	if !strings.Contains(output, "Protocol Metrics:") {
		t.Errorf("Expected Protocol Metrics section in output")
	}
	if !strings.Contains(output, "websocket:") {
		t.Errorf("Expected websocket protocol in output")
	}
	if !strings.Contains(output, "messages_sent") {
		t.Errorf("Expected messages_sent metric in output")
	}
	if !strings.Contains(output, "grpc:") {
		t.Errorf("Expected grpc protocol in output")
	}
}

func TestPrintJSONReportWithProtocolMetrics(t *testing.T) {
	stats := metrics.Stats{
		EndpointStats: metrics.EndpointStats{
			Total:          100,
			Successes:      100,
			RequestsPerSec: 50.0,
		},
		DurationMs: 2000.0,
		ProtocolMetrics: map[string]map[string]interface{}{
			"sse": {
				"events_received": int64(250),
			},
		},
	}

	var buf bytes.Buffer
	err := PrintJSONReport(&buf, stats)
	if err != nil {
		t.Fatalf("PrintJSONReport failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"protocol_metrics"`) {
		t.Errorf("Expected protocol_metrics in JSON output")
	}
	if !strings.Contains(output, `"sse"`) {
		t.Errorf("Expected sse protocol in JSON output")
	}
}
