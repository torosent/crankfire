package main

import (
	"errors"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/runner"
)

func TestMakeHeaders(t *testing.T) {
	input := map[string]string{
		"Content-Type": "application/json",
		"X-Custom":     "value",
	}
	got := makeHeaders(input)
	if got.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got.Get("Content-Type"))
	}
	if got.Get("X-Custom") != "value" {
		t.Errorf("X-Custom = %q, want value", got.Get("X-Custom"))
	}
}

func TestHttpStatusCodeFromError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"http error", &runner.HTTPError{StatusCode: 404}, "404"},
		{"generic error", errors.New("oops"), "ERRORSTRING"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := httpStatusCodeFromError(tt.err)
			if got != tt.want {
				t.Errorf("httpStatusCodeFromError() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToRunnerArrivalModel(t *testing.T) {
	tests := []struct {
		input config.ArrivalModel
		want  runner.ArrivalModel
	}{
		{config.ArrivalModelUniform, runner.ArrivalModelUniform},
		{config.ArrivalModelPoisson, runner.ArrivalModelPoisson},
		{"unknown", runner.ArrivalModelUniform}, // Default fallback
	}

	for _, tt := range tests {
		got := toRunnerArrivalModel(tt.input)
		if got != tt.want {
			t.Errorf("toRunnerArrivalModel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestToRunnerLoadPatterns(t *testing.T) {
	input := []config.LoadPattern{
		{
			Name:     "ramp",
			Type:     "ramp",
			FromRPS:  10,
			ToRPS:    100,
			Duration: time.Minute,
		},
	}
	got := toRunnerLoadPatterns(input)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].Name != "ramp" {
		t.Errorf("Name = %q, want ramp", got[0].Name)
	}
	if got[0].Type != runner.LoadPatternTypeRamp {
		t.Errorf("Type = %q, want ramp", got[0].Type)
	}
}

func TestToRunnerLoadSteps(t *testing.T) {
	input := []config.LoadStep{
		{RPS: 10, Duration: time.Second},
	}
	got := toRunnerLoadSteps(input)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].RPS != 10 {
		t.Errorf("RPS = %d, want 10", got[0].RPS)
	}
}
