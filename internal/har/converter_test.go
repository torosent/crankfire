package har

import (
	"net/url"
	"testing"
)

func TestConvert_BasicRequest(t *testing.T) {
	har := &HAR{
		Log: &Log{
			Entries: []*Entry{
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/users",
					},
				},
			},
		},
	}

	endpoints, err := Convert(har, DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}

	ep := endpoints[0]
	if ep.Method != "GET" {
		t.Errorf("expected method GET, got %s", ep.Method)
	}
	if ep.URL != "https://api.example.com/users" {
		t.Errorf("expected URL https://api.example.com/users, got %s", ep.URL)
	}
	if ep.Weight != 1 {
		t.Errorf("expected weight 1, got %d", ep.Weight)
	}
}

func TestConvert_WithHeaders(t *testing.T) {
	har := &HAR{
		Log: &Log{
			Entries: []*Entry{
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/users",
						Headers: []*Header{
							{Name: "User-Agent", Value: "Mozilla/5.0"},
							{Name: "Accept", Value: "application/json"},
						},
					},
				},
			},
		},
	}

	endpoints, err := Convert(har, DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ep := endpoints[0]
	if ep.Headers == nil {
		t.Fatal("expected headers to be non-nil")
	}

	if ep.Headers["User-Agent"] != "Mozilla/5.0" {
		t.Errorf("expected User-Agent: Mozilla/5.0, got %s", ep.Headers["User-Agent"])
	}

	if ep.Headers["Accept"] != "application/json" {
		t.Errorf("expected Accept: application/json, got %s", ep.Headers["Accept"])
	}
}

func TestConvert_WithPostData(t *testing.T) {
	har := &HAR{
		Log: &Log{
			Entries: []*Entry{
				{
					Request: &Request{
						Method: "POST",
						URL:    "https://api.example.com/users",
						PostData: &PostData{
							Text: `{"name": "Jane"}`,
						},
					},
				},
			},
		},
	}

	endpoints, err := Convert(har, DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ep := endpoints[0]
	if ep.Body != `{"name": "Jane"}` {
		t.Errorf("expected body {\"name\": \"Jane\"}, got %s", ep.Body)
	}
}

func TestConvert_FilterByHost_Include(t *testing.T) {
	har := &HAR{
		Log: &Log{
			Entries: []*Entry{
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/users",
					},
				},
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://other.example.com/products",
					},
				},
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/products",
					},
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.IncludeHosts = []string{"api.example.com"}

	endpoints, err := Convert(har, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(endpoints))
	}

	for _, ep := range endpoints {
		parsedURL, _ := url.Parse(ep.URL)
		if parsedURL.Host != "api.example.com" {
			t.Errorf("expected host api.example.com, got %s", parsedURL.Host)
		}
	}
}

func TestConvert_FilterByHost_Exclude(t *testing.T) {
	har := &HAR{
		Log: &Log{
			Entries: []*Entry{
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/users",
					},
				},
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://other.example.com/products",
					},
				},
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/products",
					},
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.ExcludeHosts = []string{"other.example.com"}

	endpoints, err := Convert(har, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(endpoints))
	}

	for _, ep := range endpoints {
		parsedURL, _ := url.Parse(ep.URL)
		if parsedURL.Host == "other.example.com" {
			t.Errorf("expected to exclude other.example.com, but found it")
		}
	}
}

func TestConvert_FilterByMethod_Include(t *testing.T) {
	har := &HAR{
		Log: &Log{
			Entries: []*Entry{
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/users",
					},
				},
				{
					Request: &Request{
						Method: "POST",
						URL:    "https://api.example.com/users",
					},
				},
				{
					Request: &Request{
						Method: "DELETE",
						URL:    "https://api.example.com/users/1",
					},
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.IncludeMethods = []string{"GET", "POST"}

	endpoints, err := Convert(har, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(endpoints))
	}

	for _, ep := range endpoints {
		if ep.Method != "GET" && ep.Method != "POST" {
			t.Errorf("expected GET or POST, got %s", ep.Method)
		}
	}
}

func TestConvert_ExcludeStaticAssets_DefaultBehavior(t *testing.T) {
	har := &HAR{
		Log: &Log{
			Entries: []*Entry{
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/users",
					},
				},
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/styles.css",
					},
				},
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/app.js",
					},
				},
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/image.png",
					},
				},
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/logo.svg",
					},
				},
			},
		},
	}

	opts := DefaultOptions()
	// ExcludeStatic should be true by default

	endpoints, err := Convert(har, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint (excluding static assets), got %d", len(endpoints))
	}

	if endpoints[0].URL != "https://api.example.com/users" {
		t.Errorf("expected /users endpoint, got %s", endpoints[0].URL)
	}
}

func TestConvert_ExcludeStaticAssets_AllExtensions(t *testing.T) {
	staticAssets := []string{
		"https://api.example.com/script.js",
		"https://api.example.com/style.css",
		"https://api.example.com/image.png",
		"https://api.example.com/photo.jpg",
		"https://api.example.com/picture.jpeg",
		"https://api.example.com/animation.gif",
		"https://api.example.com/graphic.svg",
		"https://api.example.com/font.woff",
		"https://api.example.com/font2.woff2",
		"https://api.example.com/font3.ttf",
		"https://api.example.com/font4.eot",
		"https://api.example.com/favicon.ico",
		"https://api.example.com/map.map",
	}

	entries := make([]*Entry, 0, len(staticAssets))
	for _, url := range staticAssets {
		entries = append(entries, &Entry{
			Request: &Request{
				Method: "GET",
				URL:    url,
			},
		})
	}

	har := &HAR{
		Log: &Log{
			Entries: entries,
		},
	}

	opts := DefaultOptions()
	endpoints, err := Convert(har, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpoints) != 0 {
		t.Fatalf("expected 0 endpoints (all are static assets), got %d", len(endpoints))
	}
}

func TestConvert_IncludeStaticAssets_WhenDisabled(t *testing.T) {
	har := &HAR{
		Log: &Log{
			Entries: []*Entry{
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/users",
					},
				},
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/styles.css",
					},
				},
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/app.js",
					},
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.ExcludeStatic = false

	endpoints, err := Convert(har, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpoints) != 3 {
		t.Fatalf("expected 3 endpoints (including static assets), got %d", len(endpoints))
	}
}

func TestConvert_EmptyHAR(t *testing.T) {
	har := &HAR{
		Log: &Log{
			Entries: []*Entry{},
		},
	}

	endpoints, err := Convert(har, DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpoints) != 0 {
		t.Fatalf("expected 0 endpoints, got %d", len(endpoints))
	}
}

func TestConvert_NilLog(t *testing.T) {
	har := &HAR{
		Log: nil,
	}

	_, err := Convert(har, DefaultOptions())
	if err == nil {
		t.Fatal("expected error for nil Log")
	}
}

func TestConvert_ExtractsNameFromPath(t *testing.T) {
	har := &HAR{
		Log: &Log{
			Entries: []*Entry{
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/users/123",
					},
				},
				{
					Request: &Request{
						Method: "POST",
						URL:    "https://api.example.com/api/v1/products",
					},
				},
			},
		},
	}

	endpoints, err := Convert(har, DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if endpoints[0].Name != "/users/123" {
		t.Errorf("expected name /users/123, got %s", endpoints[0].Name)
	}

	if endpoints[1].Name != "/api/v1/products" {
		t.Errorf("expected name /api/v1/products, got %s", endpoints[1].Name)
	}
}

func TestConvert_FiltersHopByHopHeaders(t *testing.T) {
	har := &HAR{
		Log: &Log{
			Entries: []*Entry{
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/users",
						Headers: []*Header{
							{Name: "User-Agent", Value: "Mozilla/5.0"},
							{Name: "Connection", Value: "keep-alive"},
							{Name: "Keep-Alive", Value: "timeout=5"},
							{Name: "Accept", Value: "application/json"},
							{Name: "Transfer-Encoding", Value: "chunked"},
						},
					},
				},
			},
		},
	}

	endpoints, err := Convert(har, DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ep := endpoints[0]
	if ep.Headers == nil {
		t.Fatal("expected headers to be non-nil")
	}

	// Should include
	if ep.Headers["User-Agent"] != "Mozilla/5.0" {
		t.Error("expected User-Agent header to be included")
	}
	if ep.Headers["Accept"] != "application/json" {
		t.Error("expected Accept header to be included")
	}

	// Should exclude hop-by-hop headers
	if _, ok := ep.Headers["Connection"]; ok {
		t.Error("expected Connection header to be filtered out")
	}
	if _, ok := ep.Headers["Keep-Alive"]; ok {
		t.Error("expected Keep-Alive header to be filtered out")
	}
	if _, ok := ep.Headers["Transfer-Encoding"]; ok {
		t.Error("expected Transfer-Encoding header to be filtered out")
	}
}

func TestConvert_NoHeadersWhenDisabled(t *testing.T) {
	har := &HAR{
		Log: &Log{
			Entries: []*Entry{
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/users",
						Headers: []*Header{
							{Name: "User-Agent", Value: "Mozilla/5.0"},
							{Name: "Accept", Value: "application/json"},
						},
					},
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.IncludeHeaders = false

	endpoints, err := Convert(har, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ep := endpoints[0]
	if len(ep.Headers) > 0 {
		t.Errorf("expected no headers when IncludeHeaders=false, got %v", ep.Headers)
	}
}

func TestConvert_MultipleFiltersApplied(t *testing.T) {
	har := &HAR{
		Log: &Log{
			Entries: []*Entry{
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/users",
					},
				},
				{
					Request: &Request{
						Method: "POST",
						URL:    "https://api.example.com/users",
					},
				},
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/styles.css",
					},
				},
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://other.com/data",
					},
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.IncludeHosts = []string{"api.example.com"}
	opts.IncludeMethods = []string{"GET"}
	// ExcludeStatic is true by default

	endpoints, err := Convert(har, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint after applying all filters, got %d", len(endpoints))
	}

	if endpoints[0].URL != "https://api.example.com/users" {
		t.Errorf("expected /users endpoint, got %s", endpoints[0].URL)
	}
}

func TestConvert_SetPath(t *testing.T) {
	har := &HAR{
		Log: &Log{
			Entries: []*Entry{
				{
					Request: &Request{
						Method: "GET",
						URL:    "https://api.example.com/users/123?limit=10",
					},
				},
			},
		},
	}

	endpoints, err := Convert(har, DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ep := endpoints[0]
	if ep.Path != "/users/123" {
		t.Errorf("expected path /users/123, got %s", ep.Path)
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if !opts.ExcludeStatic {
		t.Error("expected ExcludeStatic to be true by default")
	}

	if !opts.IncludeHeaders {
		t.Error("expected IncludeHeaders to be true by default")
	}

	if len(opts.IncludeHosts) != 0 {
		t.Errorf("expected empty IncludeHosts by default, got %v", opts.IncludeHosts)
	}

	if len(opts.ExcludeHosts) != 0 {
		t.Errorf("expected empty ExcludeHosts by default, got %v", opts.ExcludeHosts)
	}

	if len(opts.IncludeMethods) != 0 {
		t.Errorf("expected empty IncludeMethods by default, got %v", opts.IncludeMethods)
	}
}

func TestIsStaticAsset(t *testing.T) {
	testCases := []struct {
		url      string
		expected bool
	}{
		{"https://api.example.com/script.js", true},
		{"https://api.example.com/style.css", true},
		{"https://api.example.com/image.png", true},
		{"https://api.example.com/image.jpg", true},
		{"https://api.example.com/image.jpeg", true},
		{"https://api.example.com/image.gif", true},
		{"https://api.example.com/graphic.svg", true},
		{"https://api.example.com/font.woff", true},
		{"https://api.example.com/font.woff2", true},
		{"https://api.example.com/font.ttf", true},
		{"https://api.example.com/font.eot", true},
		{"https://api.example.com/favicon.ico", true},
		{"https://api.example.com/style.map", true},
		{"https://api.example.com/data.json", false},
		{"https://api.example.com/users", false},
		{"https://api.example.com/api/v1/users", false},
		{"https://api.example.com/users.html", false},
	}

	for _, tc := range testCases {
		result := isStaticAsset(tc.url)
		if result != tc.expected {
			t.Errorf("isStaticAsset(%s): expected %v, got %v", tc.url, tc.expected, result)
		}
	}
}

func TestExtractHeaders(t *testing.T) {
	headers := []*Header{
		{Name: "User-Agent", Value: "Mozilla/5.0"},
		{Name: "Accept", Value: "application/json"},
		{Name: "Content-Type", Value: "text/html"},
	}

	extracted := extractHeaders(headers)

	if extracted["User-Agent"] != "Mozilla/5.0" {
		t.Error("expected User-Agent header")
	}
	if extracted["Accept"] != "application/json" {
		t.Error("expected Accept header")
	}
	if extracted["Content-Type"] != "text/html" {
		t.Error("expected Content-Type header")
	}
	if len(extracted) != 3 {
		t.Errorf("expected 3 headers, got %d", len(extracted))
	}
}

func TestConvert_PreservesEndpointStructure(t *testing.T) {
	har := &HAR{
		Log: &Log{
			Entries: []*Entry{
				{
					Request: &Request{
						Method: "POST",
						URL:    "https://api.example.com/login",
						Headers: []*Header{
							{Name: "Content-Type", Value: "application/json"},
						},
						PostData: &PostData{
							Text: `{"username": "admin", "password": "secret"}`,
						},
					},
				},
			},
		},
	}

	endpoints, err := Convert(har, DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ep := endpoints[0]

	// Verify all fields are set
	if ep.Method == "" {
		t.Error("expected Method to be set")
	}
	if ep.URL == "" {
		t.Error("expected URL to be set")
	}
	if ep.Name == "" {
		t.Error("expected Name to be set")
	}
	if ep.Weight == 0 {
		t.Error("expected Weight to be set")
	}
	if ep.Path == "" {
		t.Error("expected Path to be set")
	}
	if ep.Body == "" {
		t.Error("expected Body to be set")
	}

	// Verify actual values
	if ep.Method != "POST" {
		t.Errorf("expected method POST, got %s", ep.Method)
	}
	if ep.Weight != 1 {
		t.Errorf("expected weight 1, got %d", ep.Weight)
	}
}
