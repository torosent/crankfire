package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/extractor"
	"github.com/torosent/crankfire/internal/httpclient"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/runner"
	"github.com/torosent/crankfire/internal/variables"
)

func TestMakeHeaders(t *testing.T) {
	input := map[string]string{
		"Content-Type": "application/json",
		"X-Custom":     "value",
	}
	got := makeHeaders(input)
	if got.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got.Get("Content-Type"))
	}
	if got.Get("X-Custom") != "value" {
		t.Errorf("X-Custom = %q, want value", got.Get("X-Custom"))
	}
}

func TestHttpStatusCodeFromError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"http error", &runner.HTTPError{StatusCode: 404}, "404"},
		{"generic error", errors.New("oops"), "ERRORSTRING"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := httpStatusCodeFromError(tt.err)
			if got != tt.want {
				t.Errorf("httpStatusCodeFromError() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToRunnerArrivalModel(t *testing.T) {
	tests := []struct {
		input config.ArrivalModel
		want  runner.ArrivalModel
	}{
		{config.ArrivalModelUniform, runner.ArrivalModelUniform},
		{config.ArrivalModelPoisson, runner.ArrivalModelPoisson},
		{"unknown", runner.ArrivalModelUniform}, // Default fallback
	}

	for _, tt := range tests {
		got := toRunnerArrivalModel(tt.input)
		if got != tt.want {
			t.Errorf("toRunnerArrivalModel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestToRunnerLoadPatterns(t *testing.T) {
	input := []config.LoadPattern{
		{
			Name:     "ramp",
			Type:     "ramp",
			FromRPS:  10,
			ToRPS:    100,
			Duration: time.Minute,
		},
	}
	got := toRunnerLoadPatterns(input)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].Name != "ramp" {
		t.Errorf("Name = %q, want ramp", got[0].Name)
	}
	if got[0].Type != runner.LoadPatternTypeRamp {
		t.Errorf("Type = %q, want ramp", got[0].Type)
	}
}

func TestToRunnerLoadSteps(t *testing.T) {
	input := []config.LoadStep{
		{RPS: 10, Duration: time.Second},
	}
	got := toRunnerLoadSteps(input)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].RPS != 10 {
		t.Errorf("RPS = %d, want 10", got[0].RPS)
	}
}

func TestHTTPRequester_ExtractsJSONPath(t *testing.T) {
	// Test extracting a JSON path from a successful response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": 123, "name": "Alice"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		TargetURL: server.URL,
		Method:    http.MethodGet,
	}

	builder, err := createTestRequestBuilder(cfg)
	if err != nil {
		t.Fatalf("failed to create request builder: %v", err)
	}

	collector := metrics.NewCollector()
	requester := &httpRequester{
		client:    http.DefaultClient,
		builder:   builder,
		collector: collector,
	}

	// Create a variable store and attach to context
	store := variables.NewStore()
	ctx := variables.NewContext(context.Background(), store)

	// Create endpoint template with extractor
	tmpl := &endpointTemplate{
		name:    "test",
		weight:  1,
		builder: builder,
		extractors: []extractor.Extractor{
			{
				JSONPath: "id",
				Variable: "user_id",
			},
		},
	}
	ctx = context.WithValue(ctx, endpointContextKey, tmpl)

	err = requester.Do(ctx)
	if err != nil {
		t.Fatalf("Do() returned error: %v", err)
	}

	// Check that the value was extracted and stored
	value, ok := store.Get("user_id")
	if !ok {
		t.Fatal("variable 'user_id' not found in store")
	}
	if value != "123" {
		t.Errorf("user_id = %q, want 123", value)
	}
}

func TestHTTPRequester_ExtractsRegex(t *testing.T) {
	// Test extracting using regex from a successful response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`Response: ID=789, Status=OK`))
	}))
	defer server.Close()

	cfg := &config.Config{
		TargetURL: server.URL,
		Method:    http.MethodGet,
	}

	builder, err := createTestRequestBuilder(cfg)
	if err != nil {
		t.Fatalf("failed to create request builder: %v", err)
	}

	collector := metrics.NewCollector()
	requester := &httpRequester{
		client:    http.DefaultClient,
		builder:   builder,
		collector: collector,
	}

	store := variables.NewStore()
	ctx := variables.NewContext(context.Background(), store)

	tmpl := &endpointTemplate{
		name:    "test",
		weight:  1,
		builder: builder,
		extractors: []extractor.Extractor{
			{
				Regex:    `ID=(\d+)`,
				Variable: "response_id",
			},
		},
	}
	ctx = context.WithValue(ctx, endpointContextKey, tmpl)

	err = requester.Do(ctx)
	if err != nil {
		t.Fatalf("Do() returned error: %v", err)
	}

	value, ok := store.Get("response_id")
	if !ok {
		t.Fatal("variable 'response_id' not found in store")
	}
	if value != "789" {
		t.Errorf("response_id = %q, want 789", value)
	}
}

func TestHTTPRequester_ExtractorChaining(t *testing.T) {
	// Test that extracted values can be used in subsequent requests
	// First request extracts an ID from JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"session_id": "sess-12345"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		TargetURL: server.URL,
		Method:    http.MethodGet,
	}

	builder, err := createTestRequestBuilder(cfg)
	if err != nil {
		t.Fatalf("failed to create request builder: %v", err)
	}

	collector := metrics.NewCollector()
	requester := &httpRequester{
		client:    http.DefaultClient,
		builder:   builder,
		collector: collector,
	}

	store := variables.NewStore()
	ctx := variables.NewContext(context.Background(), store)

	tmpl := &endpointTemplate{
		name:    "test",
		weight:  1,
		builder: builder,
		extractors: []extractor.Extractor{
			{
				JSONPath: "session_id",
				Variable: "sid",
			},
		},
	}
	ctx = context.WithValue(ctx, endpointContextKey, tmpl)

	err = requester.Do(ctx)
	if err != nil {
		t.Fatalf("Do() returned error: %v", err)
	}

	// Verify the extracted value
	value, ok := store.Get("sid")
	if !ok {
		t.Fatal("variable 'sid' not found in store")
	}
	if value != "sess-12345" {
		t.Errorf("sid = %q, want sess-12345", value)
	}
}

func TestHTTPRequester_ExtractorNoMatch_Continues(t *testing.T) {
	// Test that missing extractors don't fail the request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": 123}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		TargetURL: server.URL,
		Method:    http.MethodGet,
	}

	builder, err := createTestRequestBuilder(cfg)
	if err != nil {
		t.Fatalf("failed to create request builder: %v", err)
	}

	collector := metrics.NewCollector()
	requester := &httpRequester{
		client:    http.DefaultClient,
		builder:   builder,
		collector: collector,
	}

	store := variables.NewStore()
	ctx := variables.NewContext(context.Background(), store)

	// Extractor looking for a missing field
	tmpl := &endpointTemplate{
		name:    "test",
		weight:  1,
		builder: builder,
		extractors: []extractor.Extractor{
			{
				JSONPath: "missing_field",
				Variable: "result",
			},
		},
	}
	ctx = context.WithValue(ctx, endpointContextKey, tmpl)

	err = requester.Do(ctx)
	if err != nil {
		t.Fatalf("Do() returned error: %v", err)
	}

	// Value should be empty but variable should exist
	value, ok := store.Get("result")
	if !ok {
		t.Fatal("variable 'result' not found in store")
	}
	if value != "" {
		t.Errorf("result = %q, want empty string", value)
	}
}

func TestHTTPRequester_ExtractOnError_WhenEnabled(t *testing.T) {
	// Test extraction from error response when OnError=true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error_code": "INVALID_REQUEST"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		TargetURL: server.URL,
		Method:    http.MethodGet,
	}

	builder, err := createTestRequestBuilder(cfg)
	if err != nil {
		t.Fatalf("failed to create request builder: %v", err)
	}

	collector := metrics.NewCollector()
	requester := &httpRequester{
		client:    http.DefaultClient,
		builder:   builder,
		collector: collector,
	}

	store := variables.NewStore()
	ctx := variables.NewContext(context.Background(), store)

	// Extractor with OnError=true
	tmpl := &endpointTemplate{
		name:    "test",
		weight:  1,
		builder: builder,
		extractors: []extractor.Extractor{
			{
				JSONPath: "error_code",
				Variable: "err_code",
				OnError:  true,
			},
		},
	}
	ctx = context.WithValue(ctx, endpointContextKey, tmpl)

	err = requester.Do(ctx)
	if err == nil {
		t.Fatal("expected error for 400 status, got nil")
	}

	// Value should still be extracted
	value, ok := store.Get("err_code")
	if !ok {
		t.Fatal("variable 'err_code' not found in store")
	}
	if value != "INVALID_REQUEST" {
		t.Errorf("err_code = %q, want INVALID_REQUEST", value)
	}
}

func TestHTTPRequester_NoExtractOnError_WhenDisabled(t *testing.T) {
	// Test that extraction is skipped from error response when OnError=false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error_code": "INVALID_REQUEST"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		TargetURL: server.URL,
		Method:    http.MethodGet,
	}

	builder, err := createTestRequestBuilder(cfg)
	if err != nil {
		t.Fatalf("failed to create request builder: %v", err)
	}

	collector := metrics.NewCollector()
	requester := &httpRequester{
		client:    http.DefaultClient,
		builder:   builder,
		collector: collector,
	}

	store := variables.NewStore()
	ctx := variables.NewContext(context.Background(), store)

	// Extractor with OnError=false (default)
	tmpl := &endpointTemplate{
		name:    "test",
		weight:  1,
		builder: builder,
		extractors: []extractor.Extractor{
			{
				JSONPath: "error_code",
				Variable: "err_code",
				OnError:  false,
			},
		},
	}
	ctx = context.WithValue(ctx, endpointContextKey, tmpl)

	err = requester.Do(ctx)
	if err == nil {
		t.Fatal("expected error for 400 status, got nil")
	}

	// Value should NOT be extracted
	_, ok := store.Get("err_code")
	if ok {
		t.Fatal("variable 'err_code' should not exist in store for error response with OnError=false")
	}
}

// createTestRequestBuilder is a helper to create a RequestBuilder for testing
func createTestRequestBuilder(cfg *config.Config) (*httpclient.RequestBuilder, error) {
	return httpclient.NewRequestBuilder(cfg)
}
