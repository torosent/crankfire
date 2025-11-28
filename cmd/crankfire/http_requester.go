package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/torosent/crankfire/internal/auth"
	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/extractor"
	"github.com/torosent/crankfire/internal/httpclient"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/runner"
)

const (
	maxLoggedBodyBytes = 1024
	maxBodyReadSize    = 1024 * 1024
)

// httpRequester implements runner.Requester for HTTP protocol.
type httpRequester struct {
	client    *http.Client
	builder   *httpclient.RequestBuilder
	collector *metrics.Collector
	helper    baseRequesterHelper
}

// Do executes an HTTP request and records metrics.
func (r *httpRequester) Do(ctx context.Context) error {
	ctx, start, meta := r.helper.initRequest(ctx, "http")
	builder := r.builder
	var tmpl *endpointTemplate
	if endpoint := endpointFromContext(ctx); endpoint != nil {
		tmpl = endpoint
		if endpoint.builder != nil {
			builder = endpoint.builder
		}
		meta.Endpoint = endpoint.name
	}
	if builder == nil {
		err := fmt.Errorf("request builder is not configured")
		meta = annotateStatus(meta, "http", fallbackStatusCode(err))
		r.collector.RecordRequest(time.Since(start), err, meta)
		return err
	}
	req, err := builder.Build(ctx)
	if err != nil {
		meta = annotateStatus(meta, "http", fallbackStatusCode(err))
		r.collector.RecordRequest(time.Since(start), err, meta)
		return err
	}

	resp, err := r.client.Do(req)
	latency := time.Since(start)
	if err != nil {
		meta = annotateStatus(meta, "http", fallbackStatusCode(err))
		r.collector.RecordRequest(latency, err, meta)
		return err
	}
	defer resp.Body.Close()

	// Read response body for extraction and error logging (up to 1MB limit).
	// Body read errors are non-fatal; we continue with an empty body.
	body, bodyErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyReadSize))
	if bodyErr != nil {
		body = nil // Ensure body is nil on error for consistent behavior
	}

	var resultErr error
	if resp.StatusCode >= 400 {
		snippet := body
		if len(snippet) > maxLoggedBodyBytes {
			snippet = snippet[:maxLoggedBodyBytes]
		}
		resultErr = &runner.HTTPError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(snippet)),
		}
		meta = annotateStatus(meta, "http", strconv.Itoa(resp.StatusCode))
	}

	// Extract values if applicable
	if tmpl != nil && len(tmpl.extractors) > 0 {
		shouldExtract := resp.StatusCode < 400
		if !shouldExtract {
			// For error responses, check if any extractor has OnError=true
			for _, ext := range tmpl.extractors {
				if ext.OnError {
					shouldExtract = true
					break
				}
			}
		}

		if shouldExtract {
			// Filter extractors based on OnError flag for error responses
			extractorsToUse := tmpl.extractors
			if resp.StatusCode >= 400 {
				extractorsToUse = filterExtractorsOnError(tmpl.extractors)
			}

			if len(extractorsToUse) > 0 {
				logger := &stderrLogger{}
				extracted := extractor.ExtractAll(body, extractorsToUse, logger)
				storeExtractedValues(ctx, extracted)
			}
		}
	}

	if resultErr != nil && meta.StatusCode == "" {
		meta = annotateStatus(meta, "http", httpStatusCodeFromError(resultErr))
	}
	r.collector.RecordRequest(latency, resultErr, meta)
	return resultErr
}

// filterExtractorsOnError returns only extractors with OnError=true.
func filterExtractorsOnError(extractors []extractor.Extractor) []extractor.Extractor {
	result := make([]extractor.Extractor, 0, len(extractors))
	for _, ext := range extractors {
		if ext.OnError {
			result = append(result, ext)
		}
	}
	return result
}

// storeExtractedValues stores extracted key-value pairs in the variable store from context.
func storeExtractedValues(ctx context.Context, values map[string]string) {
	if len(values) == 0 {
		return
	}
	store := variableStoreFromContext(ctx)
	if store == nil {
		return
	}
	for key, value := range values {
		store.Set(key, value)
	}
}

// httpStatusCodeFromError extracts the HTTP status code from an error if available.
func httpStatusCodeFromError(err error) string {
	if err == nil {
		return ""
	}
	var httpErr *runner.HTTPError
	if errors.As(err, &httpErr) && httpErr.StatusCode > 0 {
		return strconv.Itoa(httpErr.StatusCode)
	}
	return fallbackStatusCode(err)
}

// newHTTPRequestBuilder creates a new RequestBuilder with the appropriate
// auth provider and feeder based on configuration.
func newHTTPRequestBuilder(cfg *config.Config, provider auth.Provider, feeder httpclient.Feeder) (*httpclient.RequestBuilder, error) {
	switch {
	case provider != nil && feeder != nil:
		return httpclient.NewRequestBuilderWithAuthAndFeeder(cfg, provider, feeder)
	case provider != nil:
		return httpclient.NewRequestBuilderWithAuth(cfg, provider)
	case feeder != nil:
		return httpclient.NewRequestBuilderWithFeeder(cfg, feeder)
	default:
		return httpclient.NewRequestBuilder(cfg)
	}
}
