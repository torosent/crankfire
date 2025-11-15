package feeder

import (
	"strings"
)

// SubstitutePlaceholders replaces all occurrences of {{field_name}} in the template
// with the corresponding value from the record.
// If a placeholder's field is not found in the record, it is left unchanged.
func SubstitutePlaceholders(template string, record Record) string {
	result := template
	for key, value := range record {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}
