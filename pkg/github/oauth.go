// Package github implements the GitHub OAuth device flow for authenticating
// users and obtaining access tokens for repository operations.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultClientID is the public client ID for the Vibespace GitHub App.
// Override via config.yaml github.client_id for self-hosted installations.
const DefaultClientID = "Ov23lih5HRh3ytJVCxp0"

// DeviceCodeResponse contains the device code and user instructions from GitHub.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenResponse contains the OAuth token pair from GitHub.
type TokenResponse struct {
	AccessToken           string `json:"access_token"`
	RefreshToken          string `json:"refresh_token"`
	TokenType             string `json:"token_type"`
	ExpiresIn             int    `json:"expires_in"`
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
	Scope                 string `json:"scope"`
	Error                 string `json:"error,omitempty"`
	ErrorDescription      string `json:"error_description,omitempty"`
}

// BaseURL is the GitHub base URL. Exported for testing.
var BaseURL = "https://github.com"

// RequestDeviceCode starts the OAuth device flow by requesting a device code.
// For OAuth Apps, scope should be "repo" to get repository access.
// For GitHub Apps, scope is ignored (permissions come from app settings).
func RequestDeviceCode(ctx context.Context, clientID, scope string) (*DeviceCodeResponse, error) {
	form := url.Values{
		"client_id": {clientID},
	}
	if scope != "" {
		form.Set("scope", scope)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		BaseURL+"/login/device/code",
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting device code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}

	var dcResp DeviceCodeResponse
	if err := json.Unmarshal(body, &dcResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if dcResp.DeviceCode == "" {
		return nil, fmt.Errorf("empty device code in response: %s", body)
	}

	return &dcResp, nil
}

// PollForToken polls GitHub until the user authorizes the device or the context expires.
// It handles authorization_pending (retry), slow_down (+5s interval),
// and expired_token/access_denied (error).
func PollForToken(ctx context.Context, clientID, deviceCode string, interval int) (*TokenResponse, error) {
	if interval < 1 {
		interval = 5
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			token, err := exchangeDeviceCode(ctx, clientID, deviceCode)
			if err != nil {
				return nil, err
			}

			switch token.Error {
			case "":
				return token, nil
			case "authorization_pending":
				continue
			case "slow_down":
				interval += 5
				ticker.Reset(time.Duration(interval) * time.Second)
				continue
			case "expired_token":
				return nil, fmt.Errorf("device code expired — please restart the authorization")
			case "access_denied":
				return nil, fmt.Errorf("authorization denied by user")
			default:
				return nil, fmt.Errorf("github oauth error: %s — %s", token.Error, token.ErrorDescription)
			}
		}
	}
}

// RefreshAccessToken exchanges a refresh token for a new access + refresh token pair.
// No client_secret is needed for tokens obtained via the device flow.
func RefreshAccessToken(ctx context.Context, clientID, refreshToken string) (*TokenResponse, error) {
	form := url.Values{
		"client_id":     {clientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		BaseURL+"/login/oauth/access_token",
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}

	var token TokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if token.Error != "" {
		return nil, fmt.Errorf("refresh failed: %s — %s", token.Error, token.ErrorDescription)
	}

	if token.AccessToken == "" {
		return nil, fmt.Errorf("empty access token in refresh response")
	}

	return &token, nil
}

// exchangeDeviceCode makes a single token exchange attempt.
func exchangeDeviceCode(ctx context.Context, clientID, deviceCode string) (*TokenResponse, error) {
	form := url.Values{
		"client_id":   {clientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		BaseURL+"/login/oauth/access_token",
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchanging device code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var token TokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &token, nil
}
