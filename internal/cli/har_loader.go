package cli

import (
	"fmt"
	"io"
	"net/url"
	"sort"
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
	opts, err := parseHARFilterToOptions(cfg.HARFilter)
	if err != nil {
		return err
	}

	// Convert HAR to endpoints
	endpoints, err := har.Convert(harData, opts)
	if err != nil {
		return fmt.Errorf("failed to convert HAR: %w", err)
	}

	// Append to existing endpoints
	cfg.Endpoints = append(cfg.Endpoints, endpoints...)
	// Loaded: a second call (BuildRunner also calls this) must not add them twice.
	cfg.HARFile = ""

	return nil
}

// printEndpointHosts says which hosts are about to get load. A browser
// recording also holds third-party hosts (analytics, CDNs).
func printEndpointHosts(w io.Writer, endpoints []config.Endpoint) {
	seen := map[string]bool{}
	var hosts []string
	for _, ep := range endpoints {
		if u, err := url.Parse(ep.URL); err == nil && u.Host != "" && !seen[u.Host] {
			seen[u.Host] = true
			hosts = append(hosts, u.Host)
		}
	}
	sort.Strings(hosts)
	fmt.Fprintf(w, "HAR: replaying %d endpoints to %d host(s): %s\n", len(endpoints), len(hosts), strings.Join(hosts, ", "))
}

// parseHARFilterToOptions converts a filter string to har.ConvertOptions.
// Format examples:
//   - "host:example.com"
//   - "host:api.example.com,cdn.example.com"
//   - "method:GET"
//   - "method:GET,POST"
//   - "host:example.com;method:GET,POST"
func parseHARFilterToOptions(filter string) (har.ConvertOptions, error) {
	opts := har.DefaultOptions()

	if filter == "" {
		return opts, nil
	}

	parts := strings.Split(filter, ";")
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			return opts, fmt.Errorf("invalid HAR filter %q (expected host:... or method:...)", part)
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
		default:
			// A typo here used to mean "no filter": every recorded host got load.
			return opts, fmt.Errorf("unknown HAR filter key %q (supported: host, method)", key)
		}
	}

	return opts, nil
}
