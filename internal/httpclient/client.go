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
)

type RequestBuilder struct {
	method  string
	target  string
	headers http.Header
	body    BodySource
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

func (b *RequestBuilder) Build(ctx context.Context) (*http.Request, error) {
	if b == nil {
		return nil, errors.New("builder cannot be nil")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	reader, err := b.body.NewReader()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, b.method, b.target, reader)
	if err != nil {
		_ = reader.Close()
		return nil, err
	}

	if b.headers != nil {
		req.Header = make(http.Header, len(b.headers))
		for key, values := range b.headers {
			for _, val := range values {
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
