// Package copilot provides a client for GitHub Copilot's API.
package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/mods/internal/cache"
)

const (
	copilotChatAuthURL   = "https://api.github.com/copilot_internal/v2/token"
	copilotEditorVersion = "vscode/1.95.3"
	copilotUserAgent     = "curl/7.81.0" // Necessay to bypass the user-agent check
)

// AccessToken response from GitHub Copilot's token endpoint.
type AccessToken struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	Endpoints struct {
		API           string `json:"api"` // Can change in Github Enterprise instances
		OriginTracker string `json:"origin-tracker"`
		Proxy         string `json:"proxy"`
		Telemetry     string `json:"telemetry"`
	} `json:"endpoints"`
	ErrorDetails *struct {
		URL            string `json:"url,omitempty"`
		Message        string `json:"message,omitempty"`
		Title          string `json:"title,omitempty"`
		NotificationID string `json:"notification_id,omitempty"`
	} `json:"error_details,omitempty"`
}

// Client copilot client.
type Client struct {
	client      *http.Client
	cache       string
	AccessToken *AccessToken
}

// New new copilot client.
func New(cacheDir string) *Client {
	return &Client{
		client: &http.Client{},
		cache:  cacheDir,
	}
}

// Do does the request.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Editor-Version", copilotEditorVersion)
	req.Header.Set("User-Agent", copilotUserAgent)

	isTokenExpired := c.AccessToken != nil && c.AccessToken.ExpiresAt < time.Now().Unix()

	if c.AccessToken == nil || isTokenExpired {
		accessToken, err := c.Auth()
		if err != nil {
			return nil, fmt.Errorf("failed to get access token: %w", err)
		}
		c.AccessToken = &accessToken
	}

	if c.AccessToken != nil {
		req.Header.Set("Authorization", "Bearer "+c.AccessToken.Token)
	}

	httpResp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	return httpResp, nil
}

func getCopilotRefreshToken() (string, error) {
	configPath := filepath.Join(os.Getenv("HOME"), ".config/github-copilot")
	if runtime.GOOS == "windows" {
		configPath = filepath.Join(os.Getenv("LOCALAPPDATA"), "github-copilot")
	}

	// Check both possible config file locations
	configFiles := []string{
		filepath.Join(configPath, "hosts.json"),
		filepath.Join(configPath, "apps.json"),
	}

	// Try to get token from config files
	for _, path := range configFiles {
		token, err := extractCopilotTokenFromFile(path)
		if err == nil && token != "" {
			return token, nil
		}
	}

	return "", fmt.Errorf("no token found in %s", strings.Join(configFiles, ", "))
}

func extractCopilotTokenFromFile(path string) (string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read Copilot configuration file at %s: %w", path, err)
	}

	var config map[string]json.RawMessage
	if err := json.Unmarshal(bytes, &config); err != nil {
		return "", fmt.Errorf("failed to parse Copilot configuration file at %s: %w", path, err)
	}

	for key, value := range config {
		if key == "github.com" || strings.HasPrefix(key, "github.com:") {
			var tokenData map[string]string
			if err := json.Unmarshal(value, &tokenData); err != nil {
				continue
			}
			if token, exists := tokenData["oauth_token"]; exists {
				return token, nil
			}
		}
	}

	return "", fmt.Errorf("no token found in %s", path)
}

// Auth authenticates the user and retrieves an access token.
func (c *Client) Auth() (AccessToken, error) {
	cache, err := cache.NewExpiring[AccessToken](c.cache)
	if err == nil {
		var token AccessToken
		err = cache.Read("copilot", func(r io.Reader) error {
			return json.NewDecoder(r).Decode(&token)
		})
		if err == nil && token.ExpiresAt > time.Now().Unix() {
			return token, nil
		}
	}

	refreshToken, err := getCopilotRefreshToken()
	if err != nil {
		return AccessToken{}, fmt.Errorf("failed to get refresh token: %w", err)
	}

	tokenReq, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, copilotChatAuthURL, nil)
	if err != nil {
		return AccessToken{}, fmt.Errorf("failed to create token request: %w", err)
	}

	tokenReq.Header.Set("Authorization", "token "+refreshToken)
	tokenReq.Header.Set("Accept", "application/json")
	tokenReq.Header.Set("Editor-Version", copilotEditorVersion)
	tokenReq.Header.Set("User-Agent", copilotUserAgent)

	tokenResp, err := c.client.Do(tokenReq)
	if err != nil {
		return AccessToken{}, fmt.Errorf("failed to get access token: %w", err)
	}
	defer func() {
		if closeErr := tokenResp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("error closing response body: %w", closeErr)
		}
	}()

	var tokenResponse AccessToken
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokenResponse); err != nil {
		return AccessToken{}, fmt.Errorf("failed to decode token response: %w", err)
	}

	if tokenResponse.ErrorDetails != nil {
		return AccessToken{}, fmt.Errorf("token error: %s", tokenResponse.ErrorDetails.Message)
	}

	if cache != nil {
		if err := cache.Write("copilot", tokenResponse.ExpiresAt, func(w io.Writer) error {
			return json.NewEncoder(w).Encode(tokenResponse)
		}); err != nil {
			return AccessToken{}, fmt.Errorf("failed to cache token: %w", err)
		}
	}

	return tokenResponse, nil
}
