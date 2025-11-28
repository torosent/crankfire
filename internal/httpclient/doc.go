// Package httpclient provides HTTP client utilities for the crankfire load testing tool.
//
// The httpclient package handles HTTP request construction and execution with support for:
//   - Configurable timeouts and connection pooling
//   - Authentication integration (OAuth2, static tokens)
//   - Data feeders for dynamic request parameterization
//   - Body loading from inline content or files
//
// # Request Building
//
// Use [NewRequestBuilder] to create a new request builder from configuration:
//
//	builder, err := httpclient.NewRequestBuilder(cfg)
//	if err != nil {
//		return err
//	}
//	req, err := builder.Build(ctx)
//
// For requests requiring authentication, use [NewRequestBuilderWithAuth]:
//
//	builder, err := httpclient.NewRequestBuilderWithAuth(cfg, authProvider)
//
// For dynamic data injection from CSV/JSON files, use [NewRequestBuilderWithFeeder]:
//
//	builder, err := httpclient.NewRequestBuilderWithFeeder(cfg, dataFeeder)
//
// # HTTP Client
//
// The [NewClient] function creates an HTTP client optimized for load testing with
// configurable timeouts and connection reuse:
//
//	client := httpclient.NewClient(30 * time.Second)
//	resp, err := client.Do(req)
//
// # Integration
//
// This package integrates with:
//   - [github.com/torosent/crankfire/internal/auth] for authentication
//   - [github.com/torosent/crankfire/internal/feeder] for data injection
//   - [github.com/torosent/crankfire/internal/config] for configuration
package httpclient
