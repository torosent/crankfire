package main

import "strings"

func applyPlaceholders(template string, record map[string]string) string {
	if len(record) == 0 {
		return template
	}
	result := template
	for key, value := range record {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

func applyPlaceholdersToMap(values map[string]string, record map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		if len(record) > 0 {
			out[key] = applyPlaceholders(value, record)
		} else {
			out[key] = value
		}
	}
	return out
}
