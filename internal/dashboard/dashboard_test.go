package dashboard

import (
	"strings"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/metrics"
)

func TestFormatMetricValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"int", int(42), "42"},
		{"int64", int64(1000), "1000"},
		{"float64 small", float64(12.345), "12.35"},
		{"float64 large", float64(1234.5), "1234"},
		{"string", "OK", "OK"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMetricValue(tt.value)
			if result != tt.expected {
				t.Errorf("formatMetricValue(%v) = %s, expected %s", tt.value, result, tt.expected)
			}
		})
	}
}

func TestJoinLines(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		expected string
	}{
		{"empty", []string{}, ""},
		{"single", []string{"line1"}, "line1"},
		{"multiple", []string{"line1", "line2", "line3"}, "line1\nline2\nline3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinLines(tt.lines)
			if result != tt.expected {
				t.Errorf("joinLines() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestCollectorWithProtocolMetrics(t *testing.T) {
	collector := metrics.NewCollector()
	collector.Start()

	// Record some requests with protocol metrics
	collector.RecordRequest(50*time.Millisecond, nil, &metrics.RequestMetadata{
		Protocol: "websocket",
		CustomMetrics: map[string]interface{}{
			"messages_sent": int64(10),
			"bytes_sent":    int64(1024),
		},
	})

	collector.RecordRequest(60*time.Millisecond, nil, &metrics.RequestMetadata{
		Protocol: "grpc",
		CustomMetrics: map[string]interface{}{
			"calls":       int64(5),
			"status_code": "OK",
		},
	})

	stats := collector.Stats(1 * time.Second)

	if len(stats.ProtocolMetrics) != 2 {
		t.Errorf("Expected 2 protocols, got %d", len(stats.ProtocolMetrics))
	}

	if _, ok := stats.ProtocolMetrics["websocket"]; !ok {
		t.Error("Expected websocket metrics")
	}
	if _, ok := stats.ProtocolMetrics["grpc"]; !ok {
		t.Error("Expected grpc metrics")
	}
}

func TestFormatStatusListRows(t *testing.T) {
	rows := formatStatusListRows(map[string]map[string]int{
		"http": {
			"404": 3,
			"500": 1,
		},
		"grpc": {
			"UNAVAILABLE": 2,
		},
	})
	if len(rows) == 0 {
		t.Fatal("expected status rows to be populated")
	}
	if !strings.Contains(rows[0], "HTTP") {
		t.Fatalf("expected HTTP protocol in formatted row, got %s", rows[0])
	}
}

func TestSummarizeStatusBuckets(t *testing.T) {
	summary := summarizeStatusBuckets(map[string]map[string]int{
		"http": {
			"404": 2,
			"500": 1,
		},
	}, 1)
	if summary == "" {
		t.Fatal("expected summary output")
	}
	if !strings.Contains(summary, "404") {
		t.Fatalf("expected 404 in summary, got %s", summary)
	}
}
