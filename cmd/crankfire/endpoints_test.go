package main

import (
	"context"
	"errors"
	"math/rand"
	"net/http"
	"testing"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/runner"
)

func TestBuildEndpointTemplateOverrides(t *testing.T) {
	cfg := &config.Config{
		TargetURL: "https://api.example.com",
		Method:    http.MethodPost,
		Headers: map[string]string{
			"Authorization": "Bearer base",
		},
		Body: "{}",
	}

	ep := config.Endpoint{
		Name:   "list",
		Weight: 2,
		Path:   "/users",
		Method: http.MethodGet,
		Headers: map[string]string{
			"X-Feature": "beta",
		},
	}

	tmpl, err := buildEndpointTemplate(cfg, ep)
	if err != nil {
		t.Fatalf("buildEndpointTemplate error: %v", err)
	}

	req, err := tmpl.builder.Build(context.Background())
	if err != nil {
		t.Fatalf("builder.Build error: %v", err)
	}

	if req.Method != http.MethodGet {
		t.Fatalf("expected GET, got %s", req.Method)
	}
	if req.URL.String() != "https://api.example.com/users" {
		t.Fatalf("unexpected url: %s", req.URL.String())
	}
	if req.Header.Get("Authorization") != "Bearer base" {
		t.Fatalf("base header missing")
	}
	if req.Header.Get("X-Feature") != "beta" {
		t.Fatalf("endpoint header missing")
	}
}

func TestEndpointSelectionWrapperReusesChoiceAcrossRetries(t *testing.T) {
	selector := &endpointSelector{
		templates:   []*endpointTemplate{{name: "only", weight: 1}},
		totalWeight: 1,
		rnd:         randFixed(),
	}

	recorder := &retryRecorder{failures: 1}
	policy := runner.RetryPolicy{MaxAttempts: 2, ShouldRetry: func(error) bool { return true }}
	wrapped := selector.Wrap(runner.WithRetry(recorder, policy))

	if err := wrapped.Do(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if recorder.attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", recorder.attempts)
	}
	if len(recorder.templates) != 1 {
		t.Fatalf("expected single template recorded, got %d", len(recorder.templates))
	}
	for tmpl := range recorder.templates {
		if tmpl != selector.templates[0] {
			t.Fatalf("unexpected template pointer")
		}
	}
}

type retryRecorder struct {
	attempts  int
	failures  int
	templates map[*endpointTemplate]struct{}
}

func (r *retryRecorder) Do(ctx context.Context) error {
	if r.templates == nil {
		r.templates = make(map[*endpointTemplate]struct{})
	}
	tmpl := endpointFromContext(ctx)
	if tmpl == nil {
		panic("no endpoint in context")
	}
	r.templates[tmpl] = struct{}{}
	r.attempts++
	if r.attempts <= r.failures {
		return errors.New("fail")
	}
	return nil
}

func randFixed() *rand.Rand {
	src := rand.NewSource(1)
	return rand.New(src)
}
