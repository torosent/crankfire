// internal/cli/dashboard_context.go
package cli

import (
	"net/url"
	"sort"
	"strconv"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/threshold"
	"github.com/torosent/crankfire/internal/tui/runview"
)

func BuildDashboardContext(targetURL string, cfg config.Config) runview.RequestContext {
	if targetURL == "" {
		targetURL = resolveDashboardTargetURL(cfg)
	}
	ctx := runview.RequestContext{
		RawURL:    targetURL,
		Method:    cfg.Method,
		Protocol:  string(cfg.Protocol),
		TargetRPS: float64(cfg.Rate),
	}

	appendParam := func(label, value string) {
		if value == "" {
			return
		}
		ctx.Params = append(ctx.Params, runview.ContextParam{Label: label, Value: value})
	}

	appendParam("Workers", strconv.Itoa(cfg.Concurrency))
	if cfg.Rate > 0 {
		appendParam("Rate", strconv.Itoa(cfg.Rate)+"/s")
	} else {
		appendParam("Rate", "unlimited")
	}
	if cfg.Duration > 0 {
		appendParam("Duration", cfg.Duration.String())
	}
	if cfg.Total > 0 {
		appendParam("Total", strconv.Itoa(cfg.Total))
	}
	if cfg.Timeout > 0 {
		appendParam("Timeout", cfg.Timeout.String())
	}
	if cfg.Retries > 0 {
		appendParam("Retries", strconv.Itoa(cfg.Retries))
	}
	if cfg.ConfigFile != "" {
		appendParam("Config", cfg.ConfigFile)
	}

	u, err := url.Parse(targetURL)
	if err != nil {
		return ctx
	}

	keys := make([]string, 0, len(u.Query()))
	for key := range u.Query() {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, value := range u.Query()[key] {
			ctx.QueryParams = append(ctx.QueryParams, runview.ContextParam{Label: key, Value: value})
		}
	}

	if parsed, err := threshold.ParseMultiple(cfg.Thresholds); err == nil {
		for _, candidate := range parsed {
			if candidate.Metric == "http_req_duration" && candidate.Aggregate == "p95" && (candidate.Operator == "<" || candidate.Operator == "<=") {
				ctx.LatencySLOMs = candidate.Value
				break
			}
		}
	}

	return ctx
}

func resolveDashboardTargetURL(cfg config.Config) string {
	if cfg.TargetURL != "" {
		return cfg.TargetURL
	}
	if len(cfg.Endpoints) == 0 {
		return ""
	}
	if cfg.Endpoints[0].URL != "" {
		return cfg.Endpoints[0].URL
	}
	return cfg.Endpoints[0].Path
}
