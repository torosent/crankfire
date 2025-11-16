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

// OAuth2ClientCredentialsProvider implements the OAuth2 client credentials flow.
type OAuth2ClientCredentialsProvider struct {
	tokenURL            string
	clientID            string
	clientSecret        string
	scopes              []string
	refreshBeforeExpiry time.Duration
	httpClient          *http.Client
	mu                  sync.Mutex
	cachedToken         string
	tokenExpiry         time.Time
	fetchInProgress     bool
	fetchCond           *sync.Cond
}

// OAuth2ResourceOwnerProvider implements the OAuth2 resource owner password credentials flow.
type OAuth2ResourceOwnerProvider struct {
	tokenURL            string
	clientID            string
	clientSecret        string
	username            string
	password            string
	scopes              []string
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

// NewOAuth2ClientCredentialsProvider creates a new OAuth2 client credentials provider.
func NewOAuth2ClientCredentialsProvider(
	tokenURL string,
	clientID string,
	clientSecret string,
	scopes []string,
	refreshBeforeExpiry time.Duration,
) (*OAuth2ClientCredentialsProvider, error) {
	p := &OAuth2ClientCredentialsProvider{
		tokenURL:            tokenURL,
		clientID:            clientID,
		clientSecret:        clientSecret,
		scopes:              scopes,
		refreshBeforeExpiry: refreshBeforeExpiry,
		httpClient:          &http.Client{Timeout: 30 * time.Second},
	}
	p.fetchCond = sync.NewCond(&p.mu)
	return p, nil
}

// Token retrieves a valid OAuth2 access token, using cache when available.
func (p *OAuth2ClientCredentialsProvider) Token(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if cached token is still valid
	if p.cachedToken != "" && time.Now().Before(p.tokenExpiry) {
		return p.cachedToken, nil
	}

	// If another goroutine is already fetching, wait for it
	for p.fetchInProgress {
		p.fetchCond.Wait()
		// After waking up, check if we now have a valid token
		if p.cachedToken != "" && time.Now().Before(p.tokenExpiry) {
			return p.cachedToken, nil
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
		return "", err
	}

	// Cache the token
	p.cachedToken = token
	p.tokenExpiry = time.Now().Add(time.Duration(expiresIn)*time.Second - p.refreshBeforeExpiry)

	return p.cachedToken, nil
}

func (p *OAuth2ClientCredentialsProvider) fetchToken(ctx context.Context) (string, int, error) {
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	if len(p.scopes) > 0 {
		data.Set("scope", strings.Join(p.scopes, " "))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.tokenURL, strings.NewReader(data.Encode()))
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
func (p *OAuth2ClientCredentialsProvider) InjectHeader(ctx context.Context, req *http.Request) error {
	token, err := p.Token(ctx)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	return nil
}

// Close releases resources held by the provider.
func (p *OAuth2ClientCredentialsProvider) Close() error {
	p.httpClient.CloseIdleConnections()
	return nil
}

// NewOAuth2ResourceOwnerProvider creates a new OAuth2 resource owner password credentials provider.
func NewOAuth2ResourceOwnerProvider(
	tokenURL string,
	clientID string,
	clientSecret string,
	username string,
	password string,
	scopes []string,
	refreshBeforeExpiry time.Duration,
) (*OAuth2ResourceOwnerProvider, error) {
	p := &OAuth2ResourceOwnerProvider{
		tokenURL:            tokenURL,
		clientID:            clientID,
		clientSecret:        clientSecret,
		username:            username,
		password:            password,
		scopes:              scopes,
		refreshBeforeExpiry: refreshBeforeExpiry,
		httpClient:          &http.Client{Timeout: 30 * time.Second},
	}
	p.fetchCond = sync.NewCond(&p.mu)
	return p, nil
}

// Token retrieves a valid OAuth2 access token, using cache when available.
func (p *OAuth2ResourceOwnerProvider) Token(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if cached token is still valid
	if p.cachedToken != "" && time.Now().Before(p.tokenExpiry) {
		return p.cachedToken, nil
	}

	// If another goroutine is already fetching, wait for it
	for p.fetchInProgress {
		p.fetchCond.Wait()
		// After waking up, check if we now have a valid token
		if p.cachedToken != "" && time.Now().Before(p.tokenExpiry) {
			return p.cachedToken, nil
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
		return "", err
	}

	// Cache the token
	p.cachedToken = token
	p.tokenExpiry = time.Now().Add(time.Duration(expiresIn)*time.Second - p.refreshBeforeExpiry)

	return p.cachedToken, nil
}

func (p *OAuth2ResourceOwnerProvider) fetchToken(ctx context.Context) (string, int, error) {
	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("username", p.username)
	data.Set("password", p.password)
	if len(p.scopes) > 0 {
		data.Set("scope", strings.Join(p.scopes, " "))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.tokenURL, strings.NewReader(data.Encode()))
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
func (p *OAuth2ResourceOwnerProvider) InjectHeader(ctx context.Context, req *http.Request) error {
	token, err := p.Token(ctx)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	return nil
}

// Close releases resources held by the provider.
func (p *OAuth2ResourceOwnerProvider) Close() error {
	p.httpClient.CloseIdleConnections()
	return nil
}
