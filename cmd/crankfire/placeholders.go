package main

import (
	"github.com/torosent/crankfire/internal/placeholders"
	"github.com/torosent/crankfire/internal/variables"
)

// applyPlaceholders applies placeholders with optional variable store support.
// Supports priority: 1. Variable Store, 2. Feeder Record, 3. Default values ({{key|default}})
func applyPlaceholders(template string, record map[string]string, stores ...variables.Store) string {
	return placeholders.Apply(template, record, stores...)
}

func applyPlaceholdersToMap(values map[string]string, record map[string]string, stores ...variables.Store) map[string]string {
	return placeholders.ApplyToMap(values, record, stores...)
}
