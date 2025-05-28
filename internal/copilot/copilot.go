// Package copilot provides a client for GitHub Copilot's API.
package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/mods/internal/cache"
)

const (
	copilotAuthDeviceCodeURL = "https://github.com/login/device/code"
	copilotAuthTokenURL      = "https://github.com/login/oauth/access_token" // #nosec G101
	copilotChatAuthURL       = "https://api.github.com/copilot_internal/v2/token"
	copilotEditorVersion     = "vscode/1.95.3"
	copilotUserAgent         = "curl/7.81.0" // Necessay to bypass the user-agent check

	// if you change this, don't forget to update the
	// `copilotOAuthToken` json struct tag
	copilotClientID = "Iv1.b507a08c87ecfe98"
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

type copilotDeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type copilotDeviceTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error,omitempty"`
}

type copilotFailedRequestResponse struct {
	DocumentationURL string `json:"documentation_url"`
	Message          string `json:"message"`
}

type copilotGithubOAuthTokenWrapper struct {
	User        string `json:"user"`
	OAuthToken  string `json:"oauth_token"`
	GithubAppID string `json:"githubAppId"`
}

type copilotOAuthToken struct {
	GithubWrapper copilotGithubOAuthTokenWrapper `json:"github.com:Iv1.b507a08c87ecfe98"`
}

type copilotHTTPClient struct {
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

func copilotLogin(client *http.Client, configPath string) (string, error) {
	data := strings.NewReader(fmt.Sprintf("client_id=%s&scope=copilot", copilotClientID))
	req, err := http.NewRequest("POST", copilotAuthDeviceCodeURL, data)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get device code: %w", err)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to decode device code response: %w", err)
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("error closing response body: %w", closeErr)
		}
	}()

	deviceCodeResp := copilotDeviceCodeResponse{}

	parsedData, err := url.ParseQuery(string(responseBody))
	if err != nil {
		return "", fmt.Errorf("failed to parse device code response: %w", err)
	}

	deviceCodeResp.UserCode = parsedData.Get("user_code")
	deviceCodeResp.ExpiresIn, _ = strconv.Atoi(parsedData.Get("expires_in"))
	deviceCodeResp.Interval, _ = strconv.Atoi(parsedData.Get("interval"))
	deviceCodeResp.DeviceCode = parsedData.Get("device_code")
	deviceCodeResp.VerificationURI = parsedData.Get("verification_uri")

	fmt.Printf("Please go to %s and enter the code %s\n", deviceCodeResp.VerificationURI, deviceCodeResp.UserCode)
	oAuthToken, err := fetchCopilotRefreshToken(client, deviceCodeResp.DeviceCode, deviceCodeResp.Interval, deviceCodeResp.ExpiresIn)

	if err != nil {
		return "", err
	}

	err = saveCopilotOAuthToken(
		copilotOAuthToken{
			GithubWrapper: copilotGithubOAuthTokenWrapper{
				User:        "",
				OAuthToken:  oAuthToken.AccessToken,
				GithubAppID: copilotClientID,
			},
		},
		configPath,
	)

	if err != nil {
		return "", err
	}

	return oAuthToken.AccessToken, nil
}

func fetchCopilotRefreshToken(client *http.Client, deviceCode string, interval int, expiresIn int) (copilotDeviceTokenResponse, error) {
	var accessTokenResp copilotDeviceTokenResponse
	var errResp copilotFailedRequestResponse

	// Adds a delay to give the user time to open
	// the browser and type the code
	time.Sleep(30 * time.Second)

	endTime := time.Now().Add(time.Duration(expiresIn) * time.Second)
	ticker := time.NewTicker(time.Duration(interval) * time.Second)

	defer ticker.Stop()

	for range ticker.C {
		if time.Now().After(endTime) {
			return copilotDeviceTokenResponse{}, fmt.Errorf("authorization polling timeout")
		}

		fmt.Println("Trying to fetch token...")
		data := strings.NewReader(
			fmt.Sprintf(
				"client_id=%s&device_code=%s&grant_type=urn:ietf:params:oauth:grant-type:device_code",
				copilotClientID,
				deviceCode,
			),
		)
		req, err := http.NewRequest("POST", copilotAuthTokenURL, data)
		if err != nil {
			return copilotDeviceTokenResponse{}, err
		}
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return copilotDeviceTokenResponse{}, err
		}

		defer func() {
			if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
				err = fmt.Errorf("error closing response body: %w", closeErr)
			}
		}()

		isRequestFailed := resp.StatusCode != 200

		if isRequestFailed {
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
				return copilotDeviceTokenResponse{}, err
			}

			return copilotDeviceTokenResponse{}, fmt.Errorf(
				"failed to check refresh token\n\tMessage: %s\n\tDocumentation: %s",
				errResp.Message,
				errResp.DocumentationURL,
			)
		}

		if err := json.NewDecoder(resp.Body).Decode(&accessTokenResp); err != nil {
			return copilotDeviceTokenResponse{}, err
		}

		if accessTokenResp.AccessToken != "" {
			return accessTokenResp, nil
		}

		if accessTokenResp.Error != "" {
			// Handle errors like "authorization_pending" or "expired_token" appropriately
			if accessTokenResp.Error != "authorization_pending" {
				return copilotDeviceTokenResponse{}, fmt.Errorf("token error: %s", accessTokenResp.Error)
			}
		}
	}

	return copilotDeviceTokenResponse{}, fmt.Errorf("authorization polling failed or timed out")
}

func saveCopilotOAuthToken(oAuthToken copilotOAuthToken, configPath string) error {
	fileContent, err := json.Marshal(oAuthToken)

	if err != nil {
		return fmt.Errorf("error mashaling oAuthToken: %e", err)
	}

	configDir := filepath.Dir(configPath)
	if err = os.MkdirAll(configDir, 0o600); err != nil {
		return fmt.Errorf("error creating config directory: %e", err)
	}

	err = os.WriteFile(configPath, fileContent, 0o600)
	if err != nil {
		return fmt.Errorf("error writing oAuthToken to %s: %e", configPath, err)
	}

	versionsPath := filepath.Join(filepath.Dir(configPath), "versions.json")
	err = copilotRegisterApp(versionsPath)
	if err != nil {
		return fmt.Errorf("error registering mods as copilot app %e", err)
	}

	return nil
}

func copilotRegisterApp(versionsPath string) error {
	versions := make(map[string]string)

	data, err := os.ReadFile(versionsPath)
	if err == nil {
		// File exists, unmarshal contents
		if err := json.Unmarshal(data, &versions); err != nil {
			return fmt.Errorf("error parsing versions file: %w", err)
		}
	}

	// Add/update our entry
	versions["mods"] = Version

	updatedData, err := json.Marshal(versions)
	if err != nil {
		return fmt.Errorf("error marshaling versions data: %w", err)
	}

	return os.WriteFile(versionsPath, updatedData, 0o600)
}

func getCopilotOAuthToken(client *http.Client) (string, error) {
	configPath := filepath.Join(os.Getenv("HOME"), ".config/github-copilot")
	if runtime.GOOS == "windows" {
		configPath = filepath.Join(os.Getenv("LOCALAPPDATA"), "github-copilot")
	}

	// Support both legacy and current config file locations
	legacyConfigPath := filepath.Join(configPath, "hosts.json")
	currentConfigPath := filepath.Join(configPath, "apps.json")

	// Check both possible config file locations
	configFiles := []string{
		legacyConfigPath,
		currentConfigPath,
	}

	// Try to get token from config files
	for _, path := range configFiles {
		token, err := extractCopilotTokenFromFile(path)
		if err == nil && token != "" {
			return token, nil
		}
	}

	// Try to login in into Copilot
	token, err := copilotLogin(client, currentConfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to login into Copilot: %w", err)
	}

	if token != "" {
		return token, nil
	}

	return "", fmt.Errorf("empty token")
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

	oAuthToken, err := getCopilotOAuthToken(client)
	if err != nil {
		return CopilotAccessToken{}, fmt.Errorf("failed to get oAuth token: %w", err)
	}

	tokenReq, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, copilotChatAuthURL, nil)
	if err != nil {
		return AccessToken{}, fmt.Errorf("failed to create token request: %w", err)
	}

	tokenReq.Header.Set("Authorization", "token "+oAuthToken)
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
