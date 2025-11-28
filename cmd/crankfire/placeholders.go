package main

import (
	"regexp"
	"strings"

	"github.com/torosent/crankfire/internal/variables"
)

// applyPlaceholders applies placeholders with optional variable store support.
// Supports priority: 1. Variable Store, 2. Feeder Record, 3. Default values ({{key|default}})
func applyPlaceholders(template string, record map[string]string, stores ...variables.Store) string {
	// If no record and no store, return template as-is
	if len(record) == 0 && len(stores) == 0 {
		return template
	}

	var store variables.Store
	if len(stores) > 0 && stores[0] != nil {
		store = stores[0]
		// Use regex-based replacement for advanced features
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
			if val, ok := store.Get(key); ok {
				return val
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

	// Simple replacement without variable store
	result := template
	for key, value := range record {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

func applyPlaceholdersToMap(values map[string]string, record map[string]string, stores ...variables.Store) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = applyPlaceholders(value, record, stores...)
	}
	return out
}

// Legacy aliases for backward compatibility - these use the new consolidated functions
func applyPlaceholdersWithVariables(template string, record map[string]string, store variables.Store) string {
	return applyPlaceholders(template, record, store)
}

func applyPlaceholdersToMapWithVariables(values map[string]string, record map[string]string, store variables.Store) map[string]string {
	return applyPlaceholdersToMap(values, record, store)
}
