package auth

import (
	"context"
	"fmt"
	"net/http"
)

// StaticTokenProvider implements a provider that returns a pre-configured
// static token. This is typically used for OIDC tokens that are obtained
// outside of the application.
type StaticTokenProvider struct {
	token string
}

// NewStaticTokenProvider creates a new static token provider with the given token.
func NewStaticTokenProvider(token string) *StaticTokenProvider {
	return &StaticTokenProvider{
		token: token,
	}
}

// Token returns the static token immediately without any network calls.
func (p *StaticTokenProvider) Token(ctx context.Context) (string, error) {
	return p.token, nil
}

// InjectHeader injects the static token into the Authorization header.
func (p *StaticTokenProvider) InjectHeader(ctx context.Context, req *http.Request) error {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.token))
	return nil
}

// Close is a no-op for static token providers.
func (p *StaticTokenProvider) Close() error {
	return nil
}
