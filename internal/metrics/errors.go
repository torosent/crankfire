package metrics

import (
	"fmt"
	"strings"
	"unicode"
)

var friendlyAliases = map[string]string{
	"*runner.HTTPError":              "HTTP error response",
	"runner.HTTPError":               "HTTP error response",
	"*url.Error":                     "Request URL error",
	"url.Error":                      "Request URL error",
	"*context.deadlineExceededError": "Context deadline exceeded",
	"context.deadlineExceededError":  "Context deadline exceeded",
	"context.deadlineExceeded":       "Context deadline exceeded",
	"*context.deadlineExceeded":      "Context deadline exceeded",
}

// FriendlyErrorName returns a human-friendly label for a Go error type.
func FriendlyErrorName(typeName string) string {
	cleaned := strings.TrimSpace(typeName)
	if cleaned == "" {
		return "Unknown error"
	}

	if alias, ok := friendlyAliases[cleaned]; ok {
		return alias
	}

	cleaned = strings.TrimPrefix(cleaned, "*")
	if alias, ok := friendlyAliases[cleaned]; ok {
		return alias
	}
	if idx := strings.LastIndex(cleaned, "/"); idx != -1 {
		cleaned = cleaned[idx+1:]
	}

	pkg := ""
	name := cleaned
	if idx := strings.Index(name, "."); idx != -1 {
		pkg = name[:idx]
		name = name[idx+1:]
	}

	pretty := humanizeTypeName(name)
	if pretty == "" {
		pretty = name
	}

	lowerPkg := strings.ToLower(pkg)
	lowerPretty := strings.ToLower(pretty)

	switch {
	case lowerPkg == "context" && strings.Contains(lowerPretty, "deadline"):
		return "Context deadline exceeded"
	case lowerPkg == "runner" && strings.Contains(lowerPretty, "http error"):
		return "HTTP error response"
	case lowerPkg == "url" && strings.Contains(lowerPretty, "error"):
		return "Request URL error"
	}

	if pkg != "" && pkg != "main" {
		return fmt.Sprintf("%s (%s)", pretty, pkg)
	}
	return pretty
}

func humanizeTypeName(name string) string {
	if name == "" {
		return ""
	}

	var words []string
	var current []rune
	runes := []rune(name)

	appendWord := func() {
		if len(current) == 0 {
			return
		}
		word := string(current)
		if isAllUpper(word) {
			words = append(words, word)
		} else {
			words = append(words, capitalize(word))
		}
		current = current[:0]
	}

	for i, r := range runes {
		if i > 0 {
			prev := runes[i-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsUpper(r) && (unicode.IsLower(prev) || (unicode.IsUpper(prev) && nextLower)) {
				appendWord()
			} else if unicode.IsDigit(r) && !unicode.IsDigit(prev) {
				appendWord()
			}
		}
		current = append(current, r)
	}
	appendWord()

	return strings.Join(words, " ")
}

func isAllUpper(s string) bool {
	hasLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
			if !unicode.IsUpper(r) {
				return false
			}
		}
	}
	return hasLetter
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	runes := []rune(lower)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
