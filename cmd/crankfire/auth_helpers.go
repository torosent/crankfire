package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/torosent/crankfire/internal/auth"
	"github.com/torosent/crankfire/internal/config"
)

const defaultAuthRefreshLeeway = 30 * time.Second

func buildAuthProvider(cfg *config.Config) (auth.Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	authCfg := cfg.Auth
	if strings.TrimSpace(string(authCfg.Type)) == "" {
		return nil, nil
	}

	refreshWindow := authCfg.RefreshBeforeExpiry
	if refreshWindow <= 0 {
		refreshWindow = defaultAuthRefreshLeeway
	}

	switch authCfg.Type {
	case config.AuthTypeOAuth2ClientCredentials:
		return auth.NewOAuth2ClientCredentialsProvider(
			authCfg.TokenURL,
			authCfg.ClientID,
			authCfg.ClientSecret,
			authCfg.Scopes,
			refreshWindow,
		)
	case config.AuthTypeOAuth2ResourceOwner:
		return auth.NewOAuth2ResourceOwnerProvider(
			authCfg.TokenURL,
			authCfg.ClientID,
			authCfg.ClientSecret,
			authCfg.Username,
			authCfg.Password,
			authCfg.Scopes,
			refreshWindow,
		)
	case config.AuthTypeOIDCImplicit, config.AuthTypeOIDCAuthCode:
		if strings.TrimSpace(authCfg.StaticToken) == "" {
			return nil, fmt.Errorf("static token is required for %s", authCfg.Type)
		}
		return auth.NewStaticTokenProvider(authCfg.StaticToken), nil
	default:
		return nil, fmt.Errorf("unsupported auth type %q", authCfg.Type)
	}
}

// GetBearerToken retrieves a token from the provider and formats it as a Bearer string.
func GetBearerToken(ctx context.Context, provider auth.Provider) (string, error) {
	if provider == nil {
		return "", nil
	}
	token, err := provider.Token(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Bearer %s", token), nil
}

func ensureAuthHeader(ctx context.Context, provider auth.Provider, headers http.Header) error {
	if provider == nil {
		return nil
	}
	if headers == nil {
		return fmt.Errorf("headers cannot be nil")
	}
	bearer, err := GetBearerToken(ctx, provider)
	if err != nil {
		return err
	}
	headers.Set("Authorization", bearer)
	return nil
}
