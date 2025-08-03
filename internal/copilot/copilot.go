// Package copilot provides a client for GitHub Copilot's API.
package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/charmbracelet/mods/internal/cache"
	"github.com/charmbracelet/mods/internal/oauth"
)

const (
	copilotAuthDeviceCodeURL = "https://github.com/login/device/code"
	copilotAuthTokenURL      = "https://github.com/login/oauth/access_token" // #nosec G101
	copilotChatAuthURL       = "https://api.github.com/copilot_internal/v2/token"
	copilotEditorVersion     = "vscode/1.95.3"
	copilotUserAgent         = "curl/7.81.0" // Necessary to bypass the user-agent check
	copilotClientID          = "Iv1.b507a08c87ecfe98"
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

type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type DeviceTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error,omitempty"`
}

type FailedRequestResponse struct {
	DocumentationURL string `json:"documentation_url"`
	Message          string `json:"message"`
}

type OAuthTokenWrapper struct {
	User        string `json:"user"`
	OAuthToken  string `json:"oauth_token"`
	GithubAppID string `json:"githubAppId"`
}

type OAuthToken struct {
	GithubWrapper OAuthTokenWrapper `json:"github.com:Iv1.b507a08c87ecfe98"`
}

type Client struct {
	client      *http.Client
	cache       string
	AccessToken *AccessToken
}

func New(cacheDir string) *Client {
	return &Client{
		client: &http.Client{},
		cache:  cacheDir,
	}
}

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

func Login(client *http.Client, configPath string) (string, error) {
	oauthConfig := oauth.Config{
		Name:            "copilot",
		DeviceCodeURL:   copilotAuthDeviceCodeURL,
		TokenURL:        copilotAuthTokenURL,
		ClientID:        copilotClientID,
		Scopes:          []string{"copilot"},
		HTTPClient:      client,
		TokenSerializer: NewCopilotTokenSerializer("copilot"),
	}

	oauthClient := oauth.New(oauthConfig)
	token, err := oauthClient.GetToken()
	if err != nil {
		return "", err
	}

	return token.AccessToken, nil
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

	oauthConfig := oauth.Config{
		Name:            "copilot",
		DeviceCodeURL:   copilotAuthDeviceCodeURL,
		TokenURL:        copilotAuthTokenURL,
		ClientID:        copilotClientID,
		Scopes:          []string{"copilot"},
		HTTPClient:      c.client,
		UserAgent:       copilotUserAgent,
		CachePath:       c.cache,
		TokenSerializer: NewCopilotTokenSerializer("copilot"),
	}

	oauthClient := oauth.New(oauthConfig)
	oauthToken, err := oauthClient.GetToken()
	if err != nil {
		return AccessToken{}, fmt.Errorf("failed to get oAuth token: %w", err)
	}

	tokenReq, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, copilotChatAuthURL, nil)
	if err != nil {
		return AccessToken{}, fmt.Errorf("failed to create token request: %w", err)
	}

	tokenReq.Header.Set("Authorization", "token "+oauthToken.AccessToken)
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
