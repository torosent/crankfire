package dashboard

import (
	"strings"
	"testing"
	"time"

	"github.com/gizak/termui/v3/widgets"
	"github.com/torosent/crankfire/internal/metrics"
)

func TestFormatPatternDuration(t *testing.T) {
	tests := []struct {
		name     string
		d        time.Duration
		expected string
	}{
		{"zero", 0, "0s"},
		{"seconds", 20 * time.Second, "20s"},
		{"minutes only", 60 * time.Second, "1m"},
		{"minutes and seconds", 7*time.Minute + 50*time.Second, "7m50s"},
		{"whole hours", 2 * time.Hour, "2h"},
		{"mixed", 1*time.Minute + 30*time.Second, "1m30s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPatternDuration(tt.d)
			if got != tt.expected {
				t.Errorf("formatPatternDuration(%v) = %q, want %q", tt.d, got, tt.expected)
			}
		})
	}
}

func TestUpdateLoadPattern(t *testing.T) {
	makeStep := func(label string, rps int, dur time.Duration, start time.Duration) PatternStep {
		_ = rps // label encodes this already
		return PatternStep{Label: label, Duration: dur, Start: start}
	}

	steps := []PatternStep{
		makeStep("10 RPS", 10, 20*time.Second, 0),
		makeStep("50 RPS", 50, 50*time.Second, 20*time.Second),
		makeStep("100 RPS", 100, 60*time.Second, 70*time.Second),
	}
	total := 130 * time.Second

	tests := []struct {
		name          string
		elapsed       time.Duration
		wantInTitle   string
		wantCurrent   string
		wantCompleted string
		wantPending   string
	}{
		{
			name:        "at start – first step active",
			elapsed:     5 * time.Second,
			wantInTitle: "Load Pattern: step-up",
			wantCurrent: "► 10 RPS",
			wantPending: "· 50 RPS",
		},
		{
			name:          "second step active",
			elapsed:       30 * time.Second,
			wantCurrent:   "► 50 RPS",
			wantCompleted: "✓ 10 RPS",
			wantPending:   "· 100 RPS",
		},
		{
			name:          "after all steps",
			elapsed:       200 * time.Second,
			wantCompleted: "✓ 10 RPS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dashboard{
				protocolPara: widgets.NewParagraph(),
				testConfig: TestConfig{
					LoadPatternName:  "step-up",
					LoadPatternSteps: steps,
					LoadPatternTotal: total,
				},
			}
			d.updateLoadPattern(tt.elapsed)

			if tt.wantInTitle != "" && !strings.Contains(d.protocolPara.Title, tt.wantInTitle) {
				t.Errorf("title %q does not contain %q", d.protocolPara.Title, tt.wantInTitle)
			}
			if tt.wantCurrent != "" && !strings.Contains(d.protocolPara.Text, tt.wantCurrent) {
				t.Errorf("text %q does not contain current marker %q", d.protocolPara.Text, tt.wantCurrent)
			}
			if tt.wantCompleted != "" && !strings.Contains(d.protocolPara.Text, tt.wantCompleted) {
				t.Errorf("text %q does not contain completed marker %q", d.protocolPara.Text, tt.wantCompleted)
			}
			if tt.wantPending != "" && !strings.Contains(d.protocolPara.Text, tt.wantPending) {
				t.Errorf("text %q does not contain pending marker %q", d.protocolPara.Text, tt.wantPending)
			}
			// Progress bar should always be present (bar characters or percentage).
			if !strings.Contains(d.protocolPara.Text, "%") {
				t.Error("expected progress percentage in text")
			}
		})
	}
}

func TestTestConfigLoadPatternFields(t *testing.T) {
	// Verify that TestConfig correctly propagates LoadPatternSteps and LoadPatternTotal.
	steps := []PatternStep{
		{Label: "10 RPS", Duration: 20 * time.Second, Start: 0},
		{Label: "50 RPS", Duration: 50 * time.Second, Start: 20 * time.Second},
	}
	cfg := TestConfig{
		LoadPatternName:  "my-pattern",
		LoadPatternSteps: steps,
		LoadPatternTotal: 70 * time.Second,
	}
	if cfg.LoadPatternTotal != 70*time.Second {
		t.Errorf("LoadPatternTotal = %v, want 70s", cfg.LoadPatternTotal)
	}
	if len(cfg.LoadPatternSteps) != 2 {
		t.Errorf("LoadPatternSteps len = %d, want 2", len(cfg.LoadPatternSteps))
	}
	if cfg.LoadPatternSteps[1].Start != 20*time.Second {
		t.Errorf("step[1].Start = %v, want 20s", cfg.LoadPatternSteps[1].Start)
	}
}

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

func TestUpdateEndpointList(t *testing.T) {
	d := &Dashboard{
		endpointList: widgets.NewList(),
	}

	stats := metrics.Stats{
		EndpointStats: metrics.EndpointStats{
			Total: 100,
		},
		Endpoints: map[string]metrics.EndpointStats{
			"api/v1": {
				Total:          80,
				RequestsPerSec: 10.5,
				P99LatencyMs:   120.5,
				Failures:       2,
				StatusBuckets: map[string]map[string]int{
					"http": {"500": 2},
				},
			},
			"api/v2": {
				Total:          20,
				RequestsPerSec: 5.0,
				P99LatencyMs:   50.0,
				Failures:       0,
			},
		},
	}

	d.updateEndpointList(stats)

	if len(d.endpointList.Rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(d.endpointList.Rows))
	}

	// Check sorting (by total desc)
	if !strings.Contains(d.endpointList.Rows[0], "api/v1") {
		t.Error("Expected api/v1 to be first")
	}
	if !strings.Contains(d.endpointList.Rows[1], "api/v2") {
		t.Error("Expected api/v2 to be second")
	}

	// Check content formatting
	row1 := d.endpointList.Rows[0]
	if !strings.Contains(row1, "80.0%") {
		t.Error("Expected 80.0% share in row 1")
	}
	if !strings.Contains(row1, "Status HTTP 500 x2") {
		t.Error("Expected status summary in row 1")
	}
}

func TestUpdateProtocolMetrics(t *testing.T) {
	d := &Dashboard{
		protocolPara: widgets.NewParagraph(),
	}

	stats := metrics.Stats{
		ProtocolMetrics: map[string]map[string]interface{}{
			"http": {
				"connections": 10,
			},
			"websocket": {
				"messages": 100,
			},
		},
	}

	d.updateProtocolMetrics(stats)

	text := d.protocolPara.Text
	if !strings.Contains(text, "http:") {
		t.Error("Expected http section")
	}
	if !strings.Contains(text, "websocket:") {
		t.Error("Expected websocket section")
	}
	if !strings.Contains(text, "connections") {
		t.Error("Expected connections metric")
	}
}

func TestFormatTestParams(t *testing.T) {
	tests := []struct {
		name     string
		config   TestConfig
		contains []string
		excludes []string
	}{
		{
			name: "basic config",
			config: TestConfig{
				Concurrency: 10,
				Rate:        100,
				Duration:    30 * time.Second,
			},
			contains: []string{"Workers: 10", "Rate: 100/s", "Duration: 30s"},
			excludes: []string{"Protocol:", "Method:"},
		},
		{
			name: "unlimited rate",
			config: TestConfig{
				Concurrency: 5,
				Rate:        0,
			},
			contains: []string{"Workers: 5", "Rate: unlimited"},
		},
		{
			name: "websocket protocol",
			config: TestConfig{
				Protocol:    "websocket",
				Concurrency: 3,
			},
			contains: []string{"Protocol: websocket", "Workers: 3"},
		},
		{
			name: "http protocol not shown",
			config: TestConfig{
				Protocol:    "http",
				Concurrency: 3,
			},
			excludes: []string{"Protocol:"},
		},
		{
			name: "POST method shown",
			config: TestConfig{
				Method:      "POST",
				Concurrency: 3,
			},
			contains: []string{"Method: POST"},
		},
		{
			name: "GET method not shown",
			config: TestConfig{
				Method:      "GET",
				Concurrency: 3,
			},
			excludes: []string{"Method:"},
		},
		{
			name: "with retries",
			config: TestConfig{
				Concurrency: 5,
				Retries:     3,
			},
			contains: []string{"Retries: 3"},
		},
		{
			name: "with config file",
			config: TestConfig{
				Concurrency: 5,
				ConfigFile:  "test.yml",
			},
			contains: []string{"Config: test.yml"},
		},
		{
			name: "with total requests",
			config: TestConfig{
				Concurrency: 5,
				Total:       1000,
			},
			contains: []string{"Total: 1000"},
		},
		{
			name: "with timeout",
			config: TestConfig{
				Concurrency: 5,
				Timeout:     10 * time.Second,
			},
			contains: []string{"Timeout: 10s"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dashboard{testConfig: tt.config}
			result := d.formatTestParams()

			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("expected result to contain %q, got %q", s, result)
				}
			}

			for _, s := range tt.excludes {
				if strings.Contains(result, s) {
					t.Errorf("expected result NOT to contain %q, got %q", s, result)
				}
			}
		})
	}
}
