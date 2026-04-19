package template_test

import (
	"strings"
	"testing"

	"github.com/torosent/crankfire/internal/template"
)

func TestRenderHappyPath(t *testing.T) {
	src := []byte(`name: api-{{ .Env }}-{{ .Rate }}rps
description: rate {{ .Rate }}
`)
	got, err := template.Render(src, map[string]string{"Env": "prod", "Rate": "500"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "name: api-prod-500rps\ndescription: rate 500\n"
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderMissingKeyRendersEmpty(t *testing.T) {
	src := []byte(`name: api-{{ .Missing }}-x`)
	got, err := template.Render(src, map[string]string{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if string(got) != "name: api--x" {
		t.Errorf("got %q, want %q", got, "name: api--x")
	}
}

func TestRenderDefaultFunc(t *testing.T) {
	src := []byte(`rate: {{ default "100" .Rate }}`)
	got, err := template.Render(src, map[string]string{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if string(got) != "rate: 100" {
		t.Errorf("got %q", got)
	}
	got, _ = template.Render(src, map[string]string{"Rate": "500"})
	if string(got) != "rate: 500" {
		t.Errorf("got %q", got)
	}
}

func TestRenderLowerUpper(t *testing.T) {
	src := []byte(`{{ lower .Env }}-{{ upper .Region }}`)
	got, _ := template.Render(src, map[string]string{"Env": "PROD", "Region": "us-east"})
	if string(got) != "prod-US-EAST" {
		t.Errorf("got %q", got)
	}
}

func TestRenderInvalidTemplateReturnsError(t *testing.T) {
	src := []byte(`{{ .Bad`)
	_, err := template.Render(src, nil)
	if err == nil {
		t.Fatal("expected error for unterminated action")
	}
	if !strings.Contains(err.Error(), "template:") {
		t.Errorf("error should mention template parser: %v", err)
	}
}
