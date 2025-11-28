package extractor

import (
	"regexp"
)

// findRegex extracts a value from text using a regex pattern.
// If the regex has a capture group, returns the first capture group.
// If no capture group, returns the full match.
// Returns empty string if no match.
func findRegex(body []byte, pattern string, logger Logger) string {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		if logger != nil {
			logger.Warn("Invalid regex pattern: %s (error: %v)", pattern, err)
		}
		return ""
	}

	match := regex.FindSubmatch(body)
	if match == nil {
		if logger != nil {
			logger.Warn("Regex pattern not found: %s", pattern)
		}
		return ""
	}

	// If there are capture groups, return the first capture group (index 1)
	// Otherwise, return the full match (index 0)
	if len(match) > 1 {
		return string(match[1])
	}

	return string(match[0])
}
