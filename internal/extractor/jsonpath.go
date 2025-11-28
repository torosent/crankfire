package extractor

import (
	"github.com/tidwall/gjson"
)

// findJSONPath extracts a value from JSON using gjson with support for $.field and field syntax.
func findJSONPath(body []byte, path string, logger Logger) string {
	// Strip leading $. if present, or handle bare $ to return entire JSON
	if len(path) > 0 && path[0] == '$' {
		if len(path) > 1 && path[1] == '.' {
			path = path[2:]
		} else if len(path) == 1 {
			// Bare "$" means entire JSON - use @this in gjson
			path = "@this"
		}
	}

	result := gjson.GetBytes(body, path)

	if !result.Exists() {
		if logger != nil {
			logger.Warn("JSONPath not found: %s", path)
		}
		return ""
	}

	return result.String()
}
