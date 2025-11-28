package main

import (
	"fmt"
	"strings"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/har"
)

// loadHAREndpoints loads a HAR file specified in config and appends the converted endpoints.
// This function should be called after config validation.
func loadHAREndpoints(cfg *config.Config) error {
	if strings.TrimSpace(cfg.HARFile) == "" {
		return nil
	}

	// Parse HAR file
	harData, err := har.ParseFile(cfg.HARFile)
	if err != nil {
		return fmt.Errorf("failed to parse HAR file: %w", err)
	}

	// Parse and apply filter
	opts := parseHARFilterToOptions(cfg.HARFilter)

	// Convert HAR to endpoints
	endpoints, err := har.Convert(harData, opts)
	if err != nil {
		return fmt.Errorf("failed to convert HAR: %w", err)
	}

	// Append to existing endpoints
	cfg.Endpoints = append(cfg.Endpoints, endpoints...)

	return nil
}

// parseHARFilterToOptions converts a filter string to har.ConvertOptions.
// Format examples:
//   - "host:example.com"
//   - "host:api.example.com,cdn.example.com"
//   - "method:GET"
//   - "method:GET,POST"
//   - "host:example.com;method:GET,POST"
func parseHARFilterToOptions(filter string) har.ConvertOptions {
	opts := har.DefaultOptions()

	if filter == "" {
		return opts
	}

	parts := strings.Split(filter, ";")
	for _, part := range parts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(kv[0]))
		value := strings.TrimSpace(kv[1])

		switch key {
		case "host":
			hosts := strings.Split(value, ",")
			for i := range hosts {
				hosts[i] = strings.TrimSpace(hosts[i])
			}
			opts.IncludeHosts = hosts
		case "method":
			methods := strings.Split(value, ",")
			for i := range methods {
				methods[i] = strings.TrimSpace(methods[i])
			}
			opts.IncludeMethods = methods
		}
	}

	return opts
}
