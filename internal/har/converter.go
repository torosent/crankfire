package har

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/torosent/crankfire/internal/config"
)

// Convert transforms HAR entries into Crankfire Endpoint structs with optional filtering.
func Convert(har *HAR, opts ConvertOptions) ([]config.Endpoint, error) {
	if har == nil || har.Log == nil {
		return nil, fmt.Errorf("HAR is nil or has nil Log")
	}

	var endpoints []config.Endpoint

	for _, entry := range har.Log.Entries {
		if entry == nil || entry.Request == nil {
			continue
		}

		if !shouldIncludeEntry(entry, opts) {
			continue
		}

		endpoint := entryToEndpoint(entry, opts)
		endpoints = append(endpoints, endpoint)
	}

	return endpoints, nil
}

// shouldIncludeEntry determines whether a HAR entry should be included in the conversion
// based on the provided filtering options.
func shouldIncludeEntry(entry *Entry, opts ConvertOptions) bool {
	if entry == nil || entry.Request == nil {
		return false
	}

	req := entry.Request

	// Filter by include hosts (if specified)
	if len(opts.IncludeHosts) > 0 {
		parsedURL, err := url.Parse(req.URL)
		if err != nil {
			return false
		}
		found := false
		for _, host := range opts.IncludeHosts {
			if parsedURL.Host == host {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by exclude hosts
	if len(opts.ExcludeHosts) > 0 {
		parsedURL, err := url.Parse(req.URL)
		if err != nil {
			return false
		}
		for _, host := range opts.ExcludeHosts {
			if parsedURL.Host == host {
				return false
			}
		}
	}

	// Filter by include methods (if specified)
	if len(opts.IncludeMethods) > 0 {
		found := false
		for _, method := range opts.IncludeMethods {
			if strings.EqualFold(req.Method, method) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter out static assets if enabled
	if opts.ExcludeStatic && isStaticAsset(req.URL) {
		return false
	}

	return true
}

// entryToEndpoint converts a single HAR Entry to a Crankfire Endpoint.
func entryToEndpoint(entry *Entry, opts ConvertOptions) config.Endpoint {
	req := entry.Request

	endpoint := config.Endpoint{
		Method: req.Method,
		URL:    req.URL,
		Weight: 1,
	}

	// Extract path from URL
	if parsedURL, err := url.Parse(req.URL); err == nil {
		endpoint.Path = parsedURL.Path
		endpoint.Name = parsedURL.Path
	}

	// Include headers if enabled
	if opts.IncludeHeaders && len(req.Headers) > 0 {
		endpoint.Headers = extractHeaders(req.Headers)
	}

	// Extract POST body if present
	if req.PostData != nil && req.PostData.Text != "" {
		endpoint.Body = req.PostData.Text
	}

	return endpoint
}

// isStaticAsset checks whether a URL points to a static asset
// based on file extensions.
func isStaticAsset(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	path := parsedURL.Path
	lowerPath := strings.ToLower(path)

	staticExtensions := []string{
		".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg",
		".woff", ".woff2", ".ttf", ".eot", ".ico", ".map",
	}

	for _, ext := range staticExtensions {
		if strings.HasSuffix(lowerPath, ext) {
			return true
		}
	}

	return false
}

// extractHeaders extracts request headers from HAR format,
// filtering out hop-by-hop headers.
func extractHeaders(headers []*Header) map[string]string {
	result := make(map[string]string)

	// List of hop-by-hop headers that should be filtered out
	hopByHopHeaders := map[string]bool{
		"connection":          true,
		"keep-alive":          true,
		"proxy-authenticate":  true,
		"proxy-authorization": true,
		"te":                  true,
		"trailers":            true,
		"transfer-encoding":   true,
		"upgrade":             true,
	}

	for _, header := range headers {
		if header == nil {
			continue
		}

		lowerName := strings.ToLower(header.Name)

		// Skip hop-by-hop headers
		if hopByHopHeaders[lowerName] {
			continue
		}

		result[header.Name] = header.Value
	}

	return result
}
