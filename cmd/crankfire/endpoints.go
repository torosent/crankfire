package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/extractor"
	"github.com/torosent/crankfire/internal/httpclient"
	"github.com/torosent/crankfire/internal/runner"
	"github.com/torosent/crankfire/internal/variables"
)

type endpointTemplate struct {
	name       string
	weight     int
	builder    *httpclient.RequestBuilder
	extractors []extractor.Extractor
}

type endpointSelector struct {
	templates   []*endpointTemplate
	totalWeight int
	rnd         *rand.Rand
	mu          sync.Mutex
}

func newEndpointSelector(cfg *config.Config) (*endpointSelector, error) {
	if len(cfg.Endpoints) == 0 {
		return nil, nil
	}

	templates := make([]*endpointTemplate, 0, len(cfg.Endpoints))
	total := 0
	for idx, ep := range cfg.Endpoints {
		tmpl, err := buildEndpointTemplate(cfg, ep)
		if err != nil {
			name := ep.Name
			if strings.TrimSpace(name) == "" {
				name = fmt.Sprintf("index %d", idx)
			}
			return nil, fmt.Errorf("endpoint %s: %w", name, err)
		}
		if tmpl.weight <= 0 {
			return nil, fmt.Errorf("endpoint %s: weight must be >= 1", ep.Name)
		}
		templates = append(templates, tmpl)
		total += tmpl.weight
	}
	if total <= 0 {
		return nil, fmt.Errorf("endpoint weights must sum to > 0")
	}

	return &endpointSelector{
		templates:   templates,
		totalWeight: total,
		rnd:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

func buildEndpointTemplate(cfg *config.Config, ep config.Endpoint) (*endpointTemplate, error) {
	weight := ep.Weight
	if weight <= 0 {
		weight = 1
	}

	method := strings.TrimSpace(ep.Method)
	if method == "" {
		method = cfg.Method
	}
	if method == "" {
		method = http.MethodGet
	}

	target, err := resolveEndpointURL(strings.TrimSpace(cfg.TargetURL), ep)
	if err != nil {
		return nil, err
	}

	headers := mergeHeaders(cfg.Headers, ep.Headers)

	body := cfg.Body
	bodyFile := cfg.BodyFile
	if strings.TrimSpace(ep.Body) != "" {
		body = ep.Body
		bodyFile = ""
	}
	if strings.TrimSpace(ep.BodyFile) != "" {
		bodyFile = ep.BodyFile
		body = ""
	}

	endpointCfg := &config.Config{
		TargetURL: target,
		Method:    method,
		Headers:   headers,
		Body:      body,
		BodyFile:  bodyFile,
	}

	builder, err := httpclient.NewRequestBuilder(endpointCfg)
	if err != nil {
		return nil, err
	}

	name := ep.Name
	if strings.TrimSpace(name) == "" {
		name = target
	}

	// Convert config.Extractor to extractor.Extractor
	extractors := convertExtractors(ep.Extractors)

	return &endpointTemplate{
		name:       name,
		weight:     weight,
		builder:    builder,
		extractors: extractors,
	}, nil
}

func resolveEndpointURL(base string, ep config.Endpoint) (string, error) {
	if trimmed := strings.TrimSpace(ep.URL); trimmed != "" {
		return trimmed, nil
	}
	if base == "" {
		return "", fmt.Errorf("endpoint %s: url is required when global target is empty", ep.Name)
	}
	if strings.TrimSpace(ep.Path) == "" {
		return base, nil
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid base target %q: %w", base, err)
	}
	rel, err := url.Parse(strings.TrimSpace(ep.Path))
	if err != nil {
		return "", fmt.Errorf("invalid endpoint path %q: %w", ep.Path, err)
	}
	resolved := baseURL.ResolveReference(rel)
	return resolved.String(), nil
}

func mergeHeaders(base map[string]string, overrides map[string]string) map[string]string {
	if len(base) == 0 && len(overrides) == 0 {
		return nil
	}
	merged := make(map[string]string, len(base)+len(overrides))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range overrides {
		key := http.CanonicalHeaderKey(strings.TrimSpace(k))
		if key == "" {
			continue
		}
		merged[key] = v
	}
	return merged
}

// convertExtractors converts config.Extractor to extractor.Extractor
func convertExtractors(configExtractors []config.Extractor) []extractor.Extractor {
	if len(configExtractors) == 0 {
		return nil
	}
	result := make([]extractor.Extractor, len(configExtractors))
	for i, ce := range configExtractors {
		result[i] = extractor.Extractor{
			JSONPath: ce.JSONPath,
			Regex:    ce.Regex,
			Variable: ce.Variable,
			OnError:  ce.OnError,
		}
	}
	return result
}

func (s *endpointSelector) pickTemplate() *endpointTemplate {
	if s == nil || len(s.templates) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.totalWeight <= 0 {
		return s.templates[0]
	}
	n := s.rnd.Intn(s.totalWeight)
	cumulative := 0
	for _, tmpl := range s.templates {
		cumulative += tmpl.weight
		if n < cumulative {
			return tmpl
		}
	}
	return s.templates[len(s.templates)-1]
}

func (s *endpointSelector) Wrap(next runner.Requester) runner.Requester {
	if s == nil || len(s.templates) == 0 || next == nil {
		return next
	}
	return &endpointSelectionRequester{next: next, selector: s}
}

type endpointSelectionRequester struct {
	next     runner.Requester
	selector *endpointSelector
}

func (e *endpointSelectionRequester) Do(ctx context.Context) error {
	if e.selector == nil {
		return e.next.Do(ctx)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if tmpl := endpointFromContext(ctx); tmpl != nil {
		return e.next.Do(ctx)
	}

	// Create a variable store for this worker if not already present
	if variables.FromContext(ctx) == nil {
		store := variables.NewStore()
		ctx = variables.NewContext(ctx, store)
	}

	tmpl := e.selector.pickTemplate()
	if tmpl == nil {
		return e.next.Do(ctx)
	}
	ctx = context.WithValue(ctx, endpointContextKey, tmpl)
	return e.next.Do(ctx)
}

type endpointCtxKey struct{}

var endpointContextKey = endpointCtxKey{}

func endpointFromContext(ctx context.Context) *endpointTemplate {
	if ctx == nil {
		return nil
	}
	if tmpl, ok := ctx.Value(endpointContextKey).(*endpointTemplate); ok {
		return tmpl
	}
	return nil
}
