package widgets_test

import (
	"strings"
	"testing"

	"github.com/torosent/crankfire/internal/tui/widgets"
)

func TestProtocolMetricsBlockRendersSortedAndLimited(t *testing.T) {
	in := map[string]map[string]interface{}{
		"websocket": {"connections": 12, "messages_sent": 3400, "messages_recv": 3399, "errors": 1, "extra": "x"},
		"http":      {"keepalive_reused": 0.92},
	}
	out := widgets.ProtocolMetricsBlock(in, 4)
	if !strings.Contains(out, "http:") || !strings.Contains(out, "websocket:") {
		t.Errorf("expected both protocols, got:\n%s", out)
	}
	if strings.Index(out, "http:") > strings.Index(out, "websocket:") {
		t.Errorf("expected http listed before websocket alphabetically:\n%s", out)
	}
	if strings.Contains(out, "extra") {
		t.Errorf("expected metric beyond limit to be dropped (limit=4):\n%s", out)
	}
	if !strings.Contains(out, "0.92") {
		t.Errorf("expected float value rendered, got:\n%s", out)
	}
	if !strings.Contains(out, "3400") {
		t.Errorf("expected int value rendered, got:\n%s", out)
	}
}

func TestProtocolMetricsBlockEmptyReturnsEmpty(t *testing.T) {
	if got := widgets.ProtocolMetricsBlock(nil, 4); got != "" {
		t.Errorf("expected empty for nil input, got %q", got)
	}
}
