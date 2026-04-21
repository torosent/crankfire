// internal/tui/runview/requestcontext.go
package runview

type ContextParam struct {
	Label string
	Value string
}

type RequestContext struct {
	RawURL       string
	Method       string
	Protocol     string
	TargetRPS    float64
	LatencySLOMs float64
	Params       []ContextParam
	QueryParams  []ContextParam
}

func (c RequestContext) HasContent() bool {
	return c.RawURL != "" ||
		c.Method != "" ||
		c.Protocol != "" ||
		c.TargetRPS > 0 ||
		c.LatencySLOMs > 0 ||
		len(c.Params) > 0 ||
		len(c.QueryParams) > 0
}
