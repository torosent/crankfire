package main

import (
	"regexp"
	"strings"

	"github.com/torosent/crankfire/internal/variables"
)

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

// applyPlaceholdersWithVariables applies placeholders with support for:
// 1. Variable store (highest priority)
// 2. Feeder record
// 3. Default value syntax: {{key|default}}
// 4. Empty default: {{key|}}
func applyPlaceholdersWithVariables(template string, record map[string]string, store variables.Store) string {
	if store == nil {
		store = variables.NewStore()
	}

	// Compile regex to find placeholders with optional defaults: {{key}} or {{key|default}}
	placeholderRegex := regexp.MustCompile(`\{\{([^}|]+)(?:\|([^}]*))?\}\}`)

	return placeholderRegex.ReplaceAllStringFunc(template, func(match string) string {
		parts := placeholderRegex.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}

		key := parts[1]
		defaultValue := ""
		if len(parts) > 2 {
			defaultValue = parts[2]
		}

		// Priority 1: Variable Store
		if store != nil {
			if val, ok := store.Get(key); ok {
				return val
			}
		}

		// Priority 2: Feeder Record
		if record != nil {
			if val, ok := record[key]; ok {
				return val
			}
		}

		// Priority 3: Default Value (if specified with |)
		if len(parts) > 2 {
			return defaultValue
		}

		// No match: keep the placeholder as-is
		return match
	})
}

// applyPlaceholdersToMapWithVariables applies placeholders to all values in a map,
// using variables and defaults.
func applyPlaceholdersToMapWithVariables(values map[string]string, record map[string]string, store variables.Store) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = applyPlaceholdersWithVariables(value, record, store)
	}
	return out
}
