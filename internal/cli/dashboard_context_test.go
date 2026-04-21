// internal/cli/dashboard_context_test.go
package cli

import (
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/tui/runview"
)

func TestBuildDashboardContextIncludesExecutionAndQueryParams(t *testing.T) {
	cfg := config.Config{
		TargetURL:   "https://example.com/run?orchestrationcount=1&payloadsizekb=100",
		Method:      "POST",
		Protocol:    config.ProtocolHTTP,
		Concurrency: 5,
		Rate:        5,
		Duration:    10 * time.Minute,
		Timeout:     30 * time.Second,
		Retries:     2,
		ConfigFile:  "scenario.yaml",
		Thresholds:  []string{"http_req_duration:p95 < 250"},
	}

	ctx := BuildDashboardContext(cfg.TargetURL, cfg)

	if ctx.RawURL != cfg.TargetURL {
		t.Fatalf("RawURL = %q, want %q", ctx.RawURL, cfg.TargetURL)
	}
	if ctx.Method != "POST" {
		t.Fatalf("Method = %q, want POST", ctx.Method)
	}
	if !containsParam(ctx.Params, "Workers", "5") {
		t.Fatalf("missing Workers param in %+v", ctx.Params)
	}
	if !containsParam(ctx.Params, "Duration", "10m0s") {
		t.Fatalf("missing Duration param in %+v", ctx.Params)
	}
	if !containsParam(ctx.QueryParams, "payloadsizekb", "100") {
		t.Fatalf("missing payloadsizekb query param in %+v", ctx.QueryParams)
	}
	if ctx.TargetRPS != 5 {
		t.Fatalf("TargetRPS = %.2f, want 5", ctx.TargetRPS)
	}
	if ctx.LatencySLOMs != 250 {
		t.Fatalf("LatencySLOMs = %.2f, want 250", ctx.LatencySLOMs)
	}
}

func containsParam(params []runview.ContextParam, label, value string) bool {
	for _, param := range params {
		if param.Label == label && param.Value == value {
			return true
		}
	}
	return false
}
