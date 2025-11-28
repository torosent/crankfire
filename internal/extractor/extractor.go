// Package extractor provides extraction logic for JSON path and regex patterns
// to extract values from HTTP response bodies.
package extractor

// Logger interface for warning output.
type Logger interface {
	Warn(format string, args ...interface{})
}

// Extractor defines extraction rules for a response body.
type Extractor struct {
	// JSONPath is a JSON path expression (e.g., "$.user.id", "user.id")
	JSONPath string

	// Regex is a regex pattern with optional capture group
	Regex string

	// Variable is the variable name to store the extracted value
	Variable string

	// OnError, if true, extracts even from error responses (4xx/5xx)
	OnError bool
}

// ExtractAll applies all extractors to the response body and returns extracted key-value pairs.
// It logs warnings for extraction failures but continues processing.
// The logger parameter can be nil to suppress warnings.
func ExtractAll(body []byte, extractors []Extractor, logger Logger) map[string]string {
	result := make(map[string]string)

	if len(extractors) == 0 {
		return result
	}

	for _, extractor := range extractors {
		var value string

		if extractor.JSONPath != "" {
			value = extractJSONPath(body, extractor.JSONPath, logger)
		} else if extractor.Regex != "" {
			value = extractRegex(body, extractor.Regex, logger)
		}

		result[extractor.Variable] = value
	}

	return result
}

// extractJSONPath extracts a value from JSON using a path expression.
func extractJSONPath(body []byte, path string, logger Logger) string {
	return findJSONPath(body, path, logger)
}

// extractRegex extracts a value from text using a regex pattern.
func extractRegex(body []byte, pattern string, logger Logger) string {
	return findRegex(body, pattern, logger)
}
