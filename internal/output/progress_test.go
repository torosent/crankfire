package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/metrics"
)

func TestProgressReporterShowsProtocol(t *testing.T) {
	collector := metrics.NewCollector()
	collector.Start()

	// Record some requests with protocol metadata
	for i := 0; i < 10; i++ {
		collector.RecordRequest(50*time.Millisecond, nil, &metrics.RequestMetadata{
			Endpoint: "test-endpoint",
			Protocol: "websocket",
			CustomMetrics: map[string]interface{}{
				"messages_sent": int64(5),
			},
		})
	}

	elapsed := 1 * time.Second
	stats := collector.Stats(elapsed)

	// Verify protocol metrics are populated
	if len(stats.ProtocolMetrics) == 0 {
		t.Error("Expected protocol metrics to be populated")
	}

	if _, ok := stats.ProtocolMetrics["websocket"]; !ok {
		t.Error("Expected websocket protocol in stats")
	}
}

func TestProgressReporterBasic(t *testing.T) {
	collector := metrics.NewCollector()
	collector.Start()

	for i := 0; i < 5; i++ {
		collector.RecordRequest(30*time.Millisecond, nil, nil)
	}

	var buf bytes.Buffer
	reporter := NewProgressReporter(collector, 100*time.Millisecond, &buf)

	if reporter == nil {
		t.Fatal("Expected non-nil reporter")
	}

	reporter.Stop()
}

func TestProgressReporterFormatting(t *testing.T) {
	collector := metrics.NewCollector()
	collector.Start()

	collector.RecordRequest(50*time.Millisecond, nil, &metrics.RequestMetadata{
		Endpoint: "test",
		Protocol: "websocket",
		CustomMetrics: map[string]interface{}{
			"connections": int64(1),
		},
	})

	var buf bytes.Buffer
	reporter := NewProgressReporter(collector, 50*time.Millisecond, &buf)
	reporter.Start()

	time.Sleep(100 * time.Millisecond)
	reporter.Stop()

	output := buf.String()
	// Should contain basic progress info
	if !strings.Contains(output, "Requests:") {
		t.Error("Expected 'Requests:' in progress output")
	}
}
