package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	// CopilotClientID is the OAuth client ID for GitHub Copilot device flow.
	CopilotClientID = "Iv1.b507a08c87ecfe98"
	// DeviceCodeURL is the GitHub OAuth device code endpoint.
	DeviceCodeURL = "https://github.com/login/device/code"
	// AccessTokenURL is the GitHub OAuth access token endpoint.
	AccessTokenURL = "https://github.com/login/oauth/access_token"
)

// DeviceCodeResponse represents the response from the device code request.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// AccessTokenResponse represents the response from the access token request.
type AccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error,omitempty"`
	ErrorDesc   string `json:"error_description,omitempty"`
}

// RequestDeviceCode initiates the device flow by requesting a device code.
// The user must then visit the verification URI and enter the user code.
func RequestDeviceCode(ctx context.Context) (*DeviceCodeResponse, error) {
	// Prepare form data
	data := url.Values{}
	data.Set("client_id", CopilotClientID)
	data.Set("scope", "copilot")

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, DeviceCodeURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create device code request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request device code: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read device code response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code request failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var dcResp DeviceCodeResponse
	if err := json.Unmarshal(body, &dcResp); err != nil {
		return nil, fmt.Errorf("failed to parse device code response: %w", err)
	}

	return &dcResp, nil
}

// PollForAccessToken polls the OAuth endpoint until the user completes
// authentication or the request expires/fails.
func PollForAccessToken(ctx context.Context, deviceCode string, interval int) (string, error) {
	if interval < 1 {
		interval = 5 // Default to 5 seconds if not specified
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	client := &http.Client{Timeout: 30 * time.Second}

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			token, err := requestAccessToken(ctx, client, deviceCode)
			if err != nil {
				return "", err
			}
			if token != "" {
				return token, nil
			}
			// Continue polling if token is empty (authorization pending)
		}
	}
}

// requestAccessToken makes a single request to exchange the device code for an access token.
func requestAccessToken(ctx context.Context, client *http.Client, deviceCode string) (string, error) {
	// Prepare form data
	data := url.Values{}
	data.Set("client_id", CopilotClientID)
	data.Set("device_code", deviceCode)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, AccessTokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create access token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request access token: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read access token response: %w", err)
	}

	// Parse response
	var atResp AccessTokenResponse
	if err := json.Unmarshal(body, &atResp); err != nil {
		return "", fmt.Errorf("failed to parse access token response: %w", err)
	}

	// Check for errors
	switch atResp.Error {
	case "":
		// Success - return the access token
		if atResp.AccessToken == "" {
			return "", fmt.Errorf("received empty access token")
		}
		return atResp.AccessToken, nil

	case "authorization_pending":
		// User hasn't completed authorization yet, continue polling
		return "", nil

	case "slow_down":
		// We're polling too fast, the caller should increase the interval
		// For simplicity, we just return empty to continue polling
		return "", nil

	case "expired_token":
		return "", fmt.Errorf("device code expired, please restart authentication")

	case "access_denied":
		return "", fmt.Errorf("access denied by user")

	default:
		desc := atResp.ErrorDesc
		if desc == "" {
			desc = atResp.Error
		}
		return "", fmt.Errorf("authentication error: %s", desc)
	}
}

// GetCopilotAccessToken retrieves a fresh access token from GitHub.
// This exchanges the OAuth token for a Copilot-specific access token.
func GetCopilotAccessToken(ctx context.Context, oauthToken string) (string, error) {
	// Request a Copilot-specific token from GitHub
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/copilot_internal/v2/token", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create copilot token request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", oauthToken))
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request copilot token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read copilot token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("copilot token request failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse the response to get the token
	var tokenResp struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse copilot token response: %w", err)
	}

	if tokenResp.Token == "" {
		return "", fmt.Errorf("received empty copilot token")
	}

	return tokenResp.Token, nil
}
