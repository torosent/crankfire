// Package template renders Go text/template over raw YAML bytes.
//
// Used by Crankfire to materialize set templates into real sets.
// Output is NOT validated as YAML/Set here — callers do that.
package template

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// Render evaluates rawYAML as a Go text/template with the given params.
// Missing params render as the empty string (option missingkey=zero).
// Available funcs: default, lower, upper.
func Render(rawYAML []byte, params map[string]string) ([]byte, error) {
	tmpl, err := template.New("set").
		Option("missingkey=zero").
		Funcs(template.FuncMap{
			"default": func(d, v string) string {
				if v == "" {
					return d
				}
				return v
			},
			"lower": strings.ToLower,
			"upper": strings.ToUpper,
		}).
		Parse(string(rawYAML))
	if err != nil {
		return nil, fmt.Errorf("template parse: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return nil, fmt.Errorf("template exec: %w", err)
	}
	return buf.Bytes(), nil
}
