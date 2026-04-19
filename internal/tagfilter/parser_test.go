package tagfilter_test

import (
	"testing"

	"github.com/torosent/crankfire/internal/tagfilter"
)

func TestParseAndMatch(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		tags    []string
		want    bool
		wantErr bool
	}{
		{"empty matches all", "", []string{"prod"}, true, false},
		{"empty matches no tags", "", nil, true, false},
		{"single hit", "prod", []string{"prod"}, true, false},
		{"single miss", "prod", []string{"smoke"}, false, false},
		{"AND both present", "prod smoke", []string{"prod", "smoke"}, true, false},
		{"AND one missing", "prod smoke", []string{"prod"}, false, false},
		{"OR first hit", "prod,staging", []string{"prod"}, true, false},
		{"OR second hit", "prod,staging", []string{"staging"}, true, false},
		{"OR none hit", "prod,staging", []string{"dev"}, false, false},
		{"AND of OR matches", "prod smoke,regression", []string{"prod", "regression"}, true, false},
		{"AND of OR fails AND part", "prod smoke,regression", []string{"smoke"}, false, false},
		{"trims whitespace", "  prod  ,  staging  ", []string{"staging"}, true, false},
		{"empty token errors", "prod  ,  ,  staging", nil, false, true},
		{"invalid char errors", "prod!", nil, false, true},
		{"too long errors", "this-tag-name-is-way-too-long-and-should-be-rejected-by-the-validator-easily-now", nil, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, err := tagfilter.Parse(tc.expr)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Parse err=%v wantErr=%v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if got := m.Matches(tc.tags); got != tc.want {
				t.Errorf("Matches(%v) = %v, want %v", tc.tags, got, tc.want)
			}
		})
	}
}

func TestMatcherIsZeroValueSafe(t *testing.T) {
	var m tagfilter.Matcher
	if !m.Matches([]string{"any"}) {
		t.Error("zero Matcher should match anything")
	}
	if !m.Matches(nil) {
		t.Error("zero Matcher should match nil")
	}
}
