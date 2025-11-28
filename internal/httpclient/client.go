package httpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/placeholders"
	"github.com/torosent/crankfire/internal/variables"
)

// AuthProvider supplies authentication tokens and injects them into HTTP requests.
type AuthProvider interface {
	Token(ctx context.Context) (string, error)
	InjectHeader(ctx context.Context, req *http.Request) error
	Close() error
}

// Feeder provides per-request data records for placeholder substitution.
type Feeder interface {
	Next(ctx context.Context) (map[string]string, error)
	Close() error
	Len() int
}

type RequestBuilder struct {
	method       string
	target       string
	headers      http.Header
	body         BodySource
	authProvider AuthProvider
	feeder       Feeder
}

func NewRequestBuilder(cfg *config.Config) (*RequestBuilder, error) {
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}

	target := strings.TrimSpace(cfg.TargetURL)
	if target == "" {
		return nil, errors.New("target URL is required")
	}

	method := strings.TrimSpace(cfg.Method)
	if method == "" {
		method = http.MethodGet
	}
	method = strings.ToUpper(method)

	bodySource, err := NewBodySource(cfg)
	if err != nil {
		return nil, err
	}
	if bodySource == nil {
		bodySource = emptyBodySource{}
	}

	headers := http.Header{}
	for key, value := range cfg.Headers {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			return nil, fmt.Errorf("invalid header key %q", key)
		}
		if strings.ContainsAny(trimmedKey, "\r\n") {
			return nil, fmt.Errorf("invalid header key %q", key)
		}
		canonicalKey := http.CanonicalHeaderKey(trimmedKey)
		if canonicalKey == "" {
			return nil, fmt.Errorf("invalid header key %q", key)
		}

		if strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("invalid header value for %s", canonicalKey)
		}

		headers.Set(canonicalKey, value)
	}

	return &RequestBuilder{
		method:  method,
		target:  target,
		headers: headers,
		body:    bodySource,
	}, nil
}

// NewRequestBuilderWithAuth creates a RequestBuilder with an auth provider for automatic token injection.
func NewRequestBuilderWithAuth(cfg *config.Config, provider AuthProvider) (*RequestBuilder, error) {
	builder, err := NewRequestBuilder(cfg)
	if err != nil {
		return nil, err
	}
	builder.authProvider = provider
	return builder, nil
}

// NewRequestBuilderWithFeeder creates a RequestBuilder with a feeder for per-request data injection.
func NewRequestBuilderWithFeeder(cfg *config.Config, feeder Feeder) (*RequestBuilder, error) {
	builder, err := NewRequestBuilder(cfg)
	if err != nil {
		return nil, err
	}
	builder.feeder = feeder
	return builder, nil
}

// NewRequestBuilderWithAuthAndFeeder creates a RequestBuilder with both auth and feeder.
func NewRequestBuilderWithAuthAndFeeder(cfg *config.Config, provider AuthProvider, feeder Feeder) (*RequestBuilder, error) {
	builder, err := NewRequestBuilder(cfg)
	if err != nil {
		return nil, err
	}
	builder.authProvider = provider
	builder.feeder = feeder
	return builder, nil
}

func (b *RequestBuilder) Build(ctx context.Context) (*http.Request, error) {
	if b == nil {
		return nil, errors.New("builder cannot be nil")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	// Get feeder record if feeder is present
	var record map[string]string
	if b.feeder != nil {
		var err error
		record, err = b.feeder.Next(ctx)
		if err != nil {
			return nil, fmt.Errorf("feeder exhausted: %w", err)
		}
	}

	// Get variable store from context
	store := variables.FromContext(ctx)

	// Apply placeholder substitution to target URL
	target := b.target
	target = placeholders.Apply(target, record, store)

	reader, err := b.body.NewReader()
	if err != nil {
		return nil, err
	}

	// If feeder or store is present, apply substitution to body
	if record != nil || store != nil {
		bodyBytes, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			return nil, fmt.Errorf("read body for substitution: %w", err)
		}
		bodyStr := placeholders.Apply(string(bodyBytes), record, store)
		reader = io.NopCloser(strings.NewReader(bodyStr))
	}

	req, err := http.NewRequestWithContext(ctx, b.method, target, reader)
	if err != nil {
		_ = reader.Close()
		return nil, err
	}

	if b.headers != nil {
		req.Header = make(http.Header, len(b.headers))
		for key, values := range b.headers {
			for _, val := range values {
				// Apply placeholder substitution to header values
				val = placeholders.Apply(val, record, store)
				req.Header.Add(key, val)
			}
		}
	}

	if length, ok := b.body.ContentLength(); ok {
		req.ContentLength = length
	}

	req.GetBody = func() (io.ReadCloser, error) {
		return b.body.NewReader()
	}

	// Inject auth header if provider is present
	if b.authProvider != nil {
		if err := b.authProvider.InjectHeader(ctx, req); err != nil {
			return nil, fmt.Errorf("auth provider inject header: %w", err)
		}
	}

	return req, nil
}

func NewClient(timeout time.Duration) *http.Client {
	if timeout < 0 {
		timeout = 0
	}

	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          256,
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}
