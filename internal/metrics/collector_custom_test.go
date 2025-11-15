package metrics

import (
	"testing"
	"time"
)

func TestCollectorCustomMetrics(t *testing.T) {
	collector := NewCollector()
	collector.Start()

	// Record requests with custom metrics
	collector.RecordRequest(50*time.Millisecond, nil, &RequestMetadata{
		Endpoint: "ws-endpoint",
		Protocol: "websocket",
		CustomMetrics: map[string]interface{}{
			"messages_sent":     int64(10),
			"messages_received": int64(8),
			"bytes_sent":        int64(1024),
		},
	})

	collector.RecordRequest(60*time.Millisecond, nil, &RequestMetadata{
		Endpoint: "ws-endpoint",
		Protocol: "websocket",
		CustomMetrics: map[string]interface{}{
			"messages_sent":     int64(15),
			"messages_received": int64(12),
			"bytes_sent":        int64(2048),
		},
	})

	stats := collector.Stats(1 * time.Second)

	if len(stats.ProtocolMetrics) == 0 {
		t.Fatal("Expected ProtocolMetrics to be populated")
	}

	wsMetrics, ok := stats.ProtocolMetrics["websocket"]
	if !ok {
		t.Fatal("Expected websocket protocol metrics")
	}

	// Check aggregated values
	if msgSent, ok := wsMetrics["messages_sent"].(int64); !ok || msgSent != 25 {
		t.Errorf("Expected messages_sent=25, got %v", wsMetrics["messages_sent"])
	}

	if msgRecv, ok := wsMetrics["messages_received"].(int64); !ok || msgRecv != 20 {
		t.Errorf("Expected messages_received=20, got %v", wsMetrics["messages_received"])
	}

	if bytesSent, ok := wsMetrics["bytes_sent"].(int64); !ok || bytesSent != 3072 {
		t.Errorf("Expected bytes_sent=3072, got %v", wsMetrics["bytes_sent"])
	}
}

func TestCollectorMultipleProtocols(t *testing.T) {
	collector := NewCollector()
	collector.Start()

	// WebSocket metrics
	collector.RecordRequest(50*time.Millisecond, nil, &RequestMetadata{
		Protocol: "websocket",
		CustomMetrics: map[string]interface{}{
			"connections": int64(5),
		},
	})

	// gRPC metrics
	collector.RecordRequest(40*time.Millisecond, nil, &RequestMetadata{
		Protocol: "grpc",
		CustomMetrics: map[string]interface{}{
			"calls":       int64(10),
			"status_code": "OK",
		},
	})

	// SSE metrics
	collector.RecordRequest(30*time.Millisecond, nil, &RequestMetadata{
		Protocol: "sse",
		CustomMetrics: map[string]interface{}{
			"events": int64(100),
		},
	})

	stats := collector.Stats(1 * time.Second)

	if len(stats.ProtocolMetrics) != 3 {
		t.Errorf("Expected 3 protocols, got %d", len(stats.ProtocolMetrics))
	}

	if _, ok := stats.ProtocolMetrics["websocket"]; !ok {
		t.Error("Expected websocket metrics")
	}
	if _, ok := stats.ProtocolMetrics["grpc"]; !ok {
		t.Error("Expected grpc metrics")
	}
	if _, ok := stats.ProtocolMetrics["sse"]; !ok {
		t.Error("Expected sse metrics")
	}
}

func TestCollectorCustomMetricsAggregation(t *testing.T) {
	collector := NewCollector()
	collector.Start()

	// Test int aggregation
	collector.RecordRequest(10*time.Millisecond, nil, &RequestMetadata{
		Protocol: "test",
		CustomMetrics: map[string]interface{}{
			"counter": int(5),
		},
	})
	collector.RecordRequest(10*time.Millisecond, nil, &RequestMetadata{
		Protocol: "test",
		CustomMetrics: map[string]interface{}{
			"counter": int(3),
		},
	})

	stats := collector.Stats(1 * time.Second)
	if counter, ok := stats.ProtocolMetrics["test"]["counter"].(int); !ok || counter != 8 {
		t.Errorf("Expected counter=8, got %v", stats.ProtocolMetrics["test"]["counter"])
	}
}

func TestCollectorCustomMetricsFloatAggregation(t *testing.T) {
	collector := NewCollector()
	collector.Start()

	collector.RecordRequest(10*time.Millisecond, nil, &RequestMetadata{
		Protocol: "test",
		CustomMetrics: map[string]interface{}{
			"duration_ms": float64(123.5),
		},
	})
	collector.RecordRequest(10*time.Millisecond, nil, &RequestMetadata{
		Protocol: "test",
		CustomMetrics: map[string]interface{}{
			"duration_ms": float64(456.7),
		},
	})

	stats := collector.Stats(1 * time.Second)
	if duration, ok := stats.ProtocolMetrics["test"]["duration_ms"].(float64); !ok || duration != 580.2 {
		t.Errorf("Expected duration_ms=580.2, got %v", stats.ProtocolMetrics["test"]["duration_ms"])
	}
}

func TestCollectorCustomMetricsNonNumeric(t *testing.T) {
	collector := NewCollector()
	collector.Start()

	// Non-numeric values should keep latest value
	collector.RecordRequest(10*time.Millisecond, nil, &RequestMetadata{
		Protocol: "test",
		CustomMetrics: map[string]interface{}{
			"status": "CONNECTING",
		},
	})
	collector.RecordRequest(10*time.Millisecond, nil, &RequestMetadata{
		Protocol: "test",
		CustomMetrics: map[string]interface{}{
			"status": "CONNECTED",
		},
	})

	stats := collector.Stats(1 * time.Second)
	if status, ok := stats.ProtocolMetrics["test"]["status"].(string); !ok || status != "CONNECTED" {
		t.Errorf("Expected status=CONNECTED, got %v", stats.ProtocolMetrics["test"]["status"])
	}
}

func TestCollectorNoCustomMetrics(t *testing.T) {
	collector := NewCollector()
	collector.Start()

	collector.RecordRequest(50*time.Millisecond, nil, nil)
	collector.RecordRequest(60*time.Millisecond, nil, &RequestMetadata{
		Endpoint: "test",
	})

	stats := collector.Stats(1 * time.Second)

	if stats.ProtocolMetrics != nil && len(stats.ProtocolMetrics) > 0 {
		t.Error("Expected no protocol metrics when none provided")
	}
}
