package setrunner_test

import (
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/setrunner"
	"github.com/torosent/crankfire/internal/store"
)

func ptr[T any](v T) *T { return &v }

func TestApplyOverridesNilIsNoop(t *testing.T) {
	base := config.Config{TargetURL: "https://x", Total: 100}
	got := setrunner.ApplyOverrides(base, store.Override{})
	if got.TargetURL != base.TargetURL || got.Total != base.Total {
		t.Errorf("expected unchanged, got %+v", got)
	}
}

func TestApplyOverridesSetsScalars(t *testing.T) {
	base := config.Config{TargetURL: "https://old", Total: 10, Concurrency: 1}
	o := store.Override{
		TargetURL:     ptr("https://new"),
		TotalRequests: ptr(99),
		Concurrency:   ptr(8),
		Duration:      ptr(5 * time.Second),
	}
	got := setrunner.ApplyOverrides(base, o)
	if got.TargetURL != "https://new" || got.Total != 99 || got.Concurrency != 8 || got.Duration != 5*time.Second {
		t.Errorf("scalars not applied: %+v", got)
	}
}

func TestApplyOverridesMergesHeaders(t *testing.T) {
	base := config.Config{Headers: map[string]string{"X-A": "1", "X-B": "2"}}
	o := store.Override{Headers: map[string]string{"X-B": "B2", "X-C": "3"}}
	got := setrunner.ApplyOverrides(base, o)
	want := map[string]string{"X-A": "1", "X-B": "B2", "X-C": "3"}
	for k, v := range want {
		if got.Headers[k] != v {
			t.Errorf("header %s: got %q want %q", k, got.Headers[k], v)
		}
	}
	// base must not be mutated
	if base.Headers["X-B"] != "2" {
		t.Errorf("base mutated: %v", base.Headers)
	}
}

func TestApplyOverridesExpandsEnvInAuthToken(t *testing.T) {
	t.Setenv("CRANKFIRE_TEST_TOK", "secret123")
	o := store.Override{AuthToken: ptr("Bearer ${CRANKFIRE_TEST_TOK}")}
	got := setrunner.ApplyOverrides(config.Config{}, o)
	if got.Auth.StaticToken != "Bearer secret123" {
		t.Errorf("auth token: got %q", got.Auth.StaticToken)
	}
}

func TestApplyOverridesMissingEnvIsLiteral(t *testing.T) {
	o := store.Override{AuthToken: ptr("Bearer ${MISSING_VAR_XYZ}")}
	got := setrunner.ApplyOverrides(config.Config{}, o)
	if got.Auth.StaticToken != "Bearer " {
		t.Errorf("missing env should expand to empty: got %q", got.Auth.StaticToken)
	}
}
