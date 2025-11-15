package auth

import (
	"context"
	"net/http"
)

// Provider defines the interface for authentication providers that can
// obtain tokens and inject them into HTTP requests.
type Provider interface {
	// Token retrieves a valid authentication token, using cached values
	// when available and valid.
	Token(ctx context.Context) (string, error)

	// InjectHeader injects the authentication token into the Authorization
	// header of the provided HTTP request.
	InjectHeader(ctx context.Context, req *http.Request) error

	// Close releases any resources held by the provider.
	Close() error
}
