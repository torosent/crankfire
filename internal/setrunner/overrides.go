package setrunner

import (
	"os"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/store"
)

// ApplyOverrides returns a copy of base with non-nil fields of o applied.
// Headers are merged (override values win). AuthToken supports ${ENV}
// expansion via os.Expand (missing vars expand to empty string).
func ApplyOverrides(base config.Config, o store.Override) config.Config {
	out := base

	if o.TargetURL != nil {
		out.TargetURL = *o.TargetURL
	}
	if o.Method != nil {
		out.Method = *o.Method
	}
	if o.Body != nil {
		out.Body = *o.Body
	}
	if o.TotalRequests != nil {
		out.Total = *o.TotalRequests
	}
	if o.Rate != nil {
		out.Rate = *o.Rate
	}
	if o.Concurrency != nil {
		out.Concurrency = *o.Concurrency
	}
	if o.Duration != nil {
		out.Duration = *o.Duration
	}
	if o.Timeout != nil {
		out.Timeout = *o.Timeout
	}
	if o.AuthToken != nil {
		out.Auth.StaticToken = os.Expand(*o.AuthToken, os.Getenv)
	}

	if len(o.Headers) > 0 {
		merged := make(map[string]string, len(base.Headers)+len(o.Headers))
		for k, v := range base.Headers {
			merged[k] = v
		}
		for k, v := range o.Headers {
			merged[k] = v
		}
		out.Headers = merged
	}
	return out
}
