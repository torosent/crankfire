package main

import (
	"context"
	"errors"
	"math/rand"
	"net/http"
	"testing"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/runner"
	"github.com/torosent/crankfire/internal/variables"
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

func TestResolveEndpointURL(t *testing.T) {
	tests := []struct {
		name    string
		base    string
		ep      config.Endpoint
		want    string
		wantErr bool
	}{
		{
			name: "absolute endpoint url",
			base: "http://base.com",
			ep:   config.Endpoint{URL: "http://override.com"},
			want: "http://override.com",
		},
		{
			name: "relative path",
			base: "http://base.com/api",
			ep:   config.Endpoint{Path: "/users"},
			want: "http://base.com/users",
		},
		{
			name: "relative path with base slash",
			base: "http://base.com/api/",
			ep:   config.Endpoint{Path: "users"},
			want: "http://base.com/api/users",
		},
		{
			name: "no path no url",
			base: "http://base.com",
			ep:   config.Endpoint{},
			want: "http://base.com",
		},
		{
			name:    "empty base and no url",
			base:    "",
			ep:      config.Endpoint{Path: "/users"},
			wantErr: true,
		},
		{
			name:    "invalid base",
			base:    "://invalid",
			ep:      config.Endpoint{Path: "/users"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveEndpointURL(tt.base, tt.ep)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveEndpointURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("resolveEndpointURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMergeHeaders(t *testing.T) {
	tests := []struct {
		name      string
		base      map[string]string
		overrides map[string]string
		want      map[string]string
	}{
		{
			name:      "nil maps",
			base:      nil,
			overrides: nil,
			want:      nil,
		},
		{
			name:      "base only",
			base:      map[string]string{"A": "1"},
			overrides: nil,
			want:      map[string]string{"A": "1"},
		},
		{
			name:      "overrides only",
			base:      nil,
			overrides: map[string]string{"B": "2"},
			want:      map[string]string{"B": "2"},
		},
		{
			name:      "merge and override",
			base:      map[string]string{"A": "1", "B": "2"},
			overrides: map[string]string{"B": "3", "C": "4"},
			want:      map[string]string{"A": "1", "B": "3", "C": "4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeHeaders(tt.base, tt.overrides)
			// Check length
			if len(got) != len(tt.want) {
				t.Errorf("len(got) = %d, want %d", len(got), len(tt.want))
			}
			// Check values
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("got[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestNewEndpointSelector_Errors(t *testing.T) {
	t.Run("invalid weight", func(t *testing.T) {
		cfg := &config.Config{
			Endpoints: []config.Endpoint{
				{Name: "bad", Weight: 0},
			},
		}
		_, err := newEndpointSelector(cfg)
		if err == nil {
			t.Error("newEndpointSelector(weight=0) error = nil, want error")
		}
	})
}

func TestEndpointSelectionCreatesVariableStore(t *testing.T) {
	selector := &endpointSelector{
		templates:   []*endpointTemplate{{name: "only", weight: 1}},
		totalWeight: 1,
		rnd:         randFixed(),
	}

	storeRecorder := &variableStoreRecorder{}
	wrapped := selector.Wrap(storeRecorder)

	if err := wrapped.Do(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if storeRecorder.store == nil {
		t.Fatalf("expected variable store in context, got nil")
	}
}

func TestEndpointSelectionReusesVariableStore(t *testing.T) {
	selector := &endpointSelector{
		templates:   []*endpointTemplate{{name: "only", weight: 1}},
		totalWeight: 1,
		rnd:         randFixed(),
	}

	store := variables.NewStore()
	ctx := contextWithVariableStore(context.Background(), store)

	storeRecorder := &variableStoreRecorder{}
	wrapped := selector.Wrap(storeRecorder)

	if err := wrapped.Do(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if storeRecorder.store != store {
		t.Fatalf("expected same store to be reused")
	}
}

type variableStoreRecorder struct {
	store variables.Store
}

func (r *variableStoreRecorder) Do(ctx context.Context) error {
	r.store = variableStoreFromContext(ctx)
	return nil
}
