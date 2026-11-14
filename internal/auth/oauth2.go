package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// defaultTokenLifetime is used when the token server sends no expires_in.
const defaultTokenLifetime = time.Hour

// OAuth2Provider fetches and caches an OAuth2 access token. The grant
// (client credentials or resource owner password) is fixed by its constructor.
type OAuth2Provider struct {
	tokenURL            string
	clientID            string
	clientSecret        string
	form                url.Values // grant_type and its fields, sent on every token request
	refreshBeforeExpiry time.Duration
	httpClient          *http.Client
	mu                  sync.Mutex
	cachedToken         string
	tokenExpiry         time.Time
	fetchInProgress     bool
	fetchCond           *sync.Cond
}

type oauth2TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Error       string `json:"error,omitempty"`
	ErrorDesc   string `json:"error_description,omitempty"`
}

func newOAuth2Provider(tokenURL, clientID, clientSecret string, scopes []string, refreshBeforeExpiry time.Duration, form url.Values) *OAuth2Provider {
	if len(scopes) > 0 {
		form.Set("scope", strings.Join(scopes, " "))
	}
	p := &OAuth2Provider{
		tokenURL:            tokenURL,
		clientID:            clientID,
		clientSecret:        clientSecret,
		form:                form,
		refreshBeforeExpiry: refreshBeforeExpiry,
		httpClient:          &http.Client{Timeout: 30 * time.Second},
	}
	p.fetchCond = sync.NewCond(&p.mu)
	return p
}

// NewOAuth2ClientCredentialsProvider creates a provider for the OAuth2 client credentials flow.
func NewOAuth2ClientCredentialsProvider(
	tokenURL string,
	clientID string,
	clientSecret string,
	scopes []string,
	refreshBeforeExpiry time.Duration,
) (*OAuth2Provider, error) {
	form := url.Values{"grant_type": {"client_credentials"}}
	return newOAuth2Provider(tokenURL, clientID, clientSecret, scopes, refreshBeforeExpiry, form), nil
}

// NewOAuth2ResourceOwnerProvider creates a provider for the OAuth2 resource owner password flow.
func NewOAuth2ResourceOwnerProvider(
	tokenURL string,
	clientID string,
	clientSecret string,
	username string,
	password string,
	scopes []string,
	refreshBeforeExpiry time.Duration,
) (*OAuth2Provider, error) {
	form := url.Values{"grant_type": {"password"}, "username": {username}, "password": {password}}
	return newOAuth2Provider(tokenURL, clientID, clientSecret, scopes, refreshBeforeExpiry, form), nil
}

// Token retrieves a valid OAuth2 access token, using cache when available.
func (p *OAuth2Provider) Token(ctx context.Context) (string, error) {
	p.mu.Lock()

	// Check if cached token is still valid
	if p.cachedToken != "" && time.Now().Before(p.tokenExpiry) {
		token := p.cachedToken
		p.mu.Unlock()
		return token, nil
	}

	// If another goroutine is already fetching, wait for it
	for p.fetchInProgress {
		p.fetchCond.Wait()
		// After waking up, check if we now have a valid token
		if p.cachedToken != "" && time.Now().Before(p.tokenExpiry) {
			token := p.cachedToken
			p.mu.Unlock()
			return token, nil
		}
	}

	// Mark that we're fetching
	p.fetchInProgress = true
	p.mu.Unlock()

	// Fetch the token (without holding the lock)
	token, expiresIn, err := p.fetchToken(ctx)

	p.mu.Lock()
	p.fetchInProgress = false
	p.fetchCond.Broadcast()

	if err != nil {
		p.mu.Unlock()
		return "", err
	}

	// Cache the token
	p.cachedToken = token
	p.tokenExpiry = time.Now().Add(cacheFor(expiresIn, p.refreshBeforeExpiry))
	p.mu.Unlock()

	return token, nil
}

// cacheFor returns how long a token may be served from the cache. It is never
// zero or negative: that would fetch a new token for every single request.
func cacheFor(expiresIn int, refreshBeforeExpiry time.Duration) time.Duration {
	lifetime := time.Duration(expiresIn) * time.Second
	if lifetime <= 0 {
		lifetime = defaultTokenLifetime // expires_in is optional in the OAuth2 spec
	}
	return lifetime - min(refreshBeforeExpiry, lifetime/2)
}

func (p *OAuth2Provider) fetchToken(ctx context.Context) (string, int, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", p.tokenURL, strings.NewReader(p.form.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(p.clientID, p.clientSecret)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to fetch token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("token request failed with status %d", resp.StatusCode)
	}

	var tokenResp oauth2TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", 0, fmt.Errorf("failed to decode token response: %w", err)
	}

	if tokenResp.Error != "" {
		return "", 0, fmt.Errorf("oauth2 error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	if tokenResp.AccessToken == "" {
		return "", 0, fmt.Errorf("no access token in response")
	}

	return tokenResp.AccessToken, tokenResp.ExpiresIn, nil
}

// InjectHeader injects the OAuth2 token into the Authorization header.
func (p *OAuth2Provider) InjectHeader(ctx context.Context, req *http.Request) error {
	token, err := p.Token(ctx)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	return nil
}

// Close releases resources held by the provider.
func (p *OAuth2Provider) Close() error {
	p.httpClient.CloseIdleConnections()
	return nil
}
