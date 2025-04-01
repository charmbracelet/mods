package main

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
)

const (
	copilotAuthDeviceCodeURL = "https://github.com/login/device/code"
	copilotAuthTokenURL      = "https://github.com/login/oauth/access_token"
	copilotChatAuthURL       = "https://api.github.com/copilot_internal/v2/token"
	copilotEditorVersion     = "vscode/1.95.3"
	copilotUserAgent         = "curl/7.81.0" // Necessay to bypass the user-agent check

	// if you change this, don't forget to update the
	// `copilotOAuthToken` json struct tag
	copilotClientID = "Iv1.b507a08c87ecfe98"
)

// Authentication response from GitHub Copilot's token endpoint.
type CopilotAccessToken struct {
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

type CopilotDeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationUri string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type CopilotDeviceTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error,omitempty"`
}

type CopilotFailedRequestResponse struct {
	DocumentationURL string `json:"documentation_url"`
	Message          string `json:"message"`
}

type copilotGithubOAuthTokenWrapper struct {
	User        string `json:"user"`
	OAuthToken  string `json:"oauth_token"`
	GithubAppId string `json:"githubAppId"`
}

type copilotOAuthToken struct {
	GithubWrapper copilotGithubOAuthTokenWrapper `json:"github.com:Iv1.b507a08c87ecfe98"`
}

type copilotHTTPClient struct {
	client      *http.Client
	AccessToken *CopilotAccessToken
}

func newCopilotHTTPClient() *copilotHTTPClient {
	return &copilotHTTPClient{
		client: &http.Client{},
	}
}

func (c *copilotHTTPClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Editor-Version", copilotEditorVersion)
	req.Header.Set("User-Agent", copilotUserAgent)

	isTokenExpired := c.AccessToken != nil && c.AccessToken.ExpiresAt < time.Now().Unix()

	if c.AccessToken == nil || isTokenExpired {
		accessToken, err := getCopilotAccessToken(c.client)
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

	var deviceCodeResp CopilotDeviceCodeResponse = CopilotDeviceCodeResponse{}

	parsedData, err := url.ParseQuery(string(responseBody))
	if err != nil {
		return "", fmt.Errorf("failed to parse device code response: %w", err)
	}

	deviceCodeResp.UserCode = parsedData.Get("user_code")
	deviceCodeResp.ExpiresIn, _ = strconv.Atoi(parsedData.Get("expires_in"))
	deviceCodeResp.Interval, _ = strconv.Atoi(parsedData.Get("interval"))
	deviceCodeResp.DeviceCode = parsedData.Get("device_code")
	deviceCodeResp.VerificationUri = parsedData.Get("verification_uri")

	fmt.Printf("Please go to %s and enter the code %s\n", deviceCodeResp.VerificationUri, deviceCodeResp.UserCode)
	oAuthToken, err := fetchCopilotRefreshToken(client, deviceCodeResp.DeviceCode, deviceCodeResp.Interval, deviceCodeResp.ExpiresIn)

	if err != nil {
		return "", err
	}

	err = saveCopilotOAuthToken(
		copilotOAuthToken{
			GithubWrapper: copilotGithubOAuthTokenWrapper{
				User:        "",
				OAuthToken:  oAuthToken.AccessToken,
				GithubAppId: copilotClientID,
			},
		},
		configPath,
	)

	if err != nil {
		return "", err
	}

	return oAuthToken.AccessToken, nil
}

func fetchCopilotRefreshToken(client *http.Client, deviceCode string, interval int, expiresIn int) (CopilotDeviceTokenResponse, error) {
	var accessTokenResp CopilotDeviceTokenResponse
	var errResp CopilotFailedRequestResponse

	endTime := time.Now().Add(time.Duration(expiresIn) * time.Second)
	ticker := time.NewTicker(time.Duration(interval) * time.Second)

	defer ticker.Stop()

	for range ticker.C {
		if time.Now().After(endTime) {
			return CopilotDeviceTokenResponse{}, fmt.Errorf("authorization polling timeout")
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
			return CopilotDeviceTokenResponse{}, err
		}
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return CopilotDeviceTokenResponse{}, err
		}
		defer resp.Body.Close()

		isRequestFailed := resp.StatusCode != 200

		if isRequestFailed {
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
				return CopilotDeviceTokenResponse{}, err
			}

			return CopilotDeviceTokenResponse{}, fmt.Errorf(
				"Failed to check refresh token\n\tMessage: %s\n\tDocumentation: %s",
				errResp.Message,
				errResp.DocumentationURL,
			)
		}

		if err := json.NewDecoder(resp.Body).Decode(&accessTokenResp); err != nil {
			return CopilotDeviceTokenResponse{}, err
		}

		if accessTokenResp.AccessToken != "" {
			// save to the new location

			return accessTokenResp, nil
		}

		if accessTokenResp.Error != "" {
			// Handle errors like "authorization_pending" or "expired_token" appropriately
			if accessTokenResp.Error != "authorization_pending" {
				return CopilotDeviceTokenResponse{}, fmt.Errorf("token error: %s", accessTokenResp.Error)
			}
		}
	}

	return CopilotDeviceTokenResponse{}, fmt.Errorf("authorization polling failed or timed out")
}

func saveCopilotOAuthToken(oAuthToken copilotOAuthToken, configPath string) error {
	fileContent, err := json.Marshal(oAuthToken)

	if err != nil {
		return fmt.Errorf("Error mashaling oAuthToken: %e", err)
	}

	err = os.WriteFile(configPath, fileContent, os.ModePerm)

	versionsPath := filepath.Join(filepath.Dir(configPath), "versions.json")
	copilotRegisterApp(versionsPath)

	if err != nil {
		return fmt.Errorf("Error writing oAuthToken to %s: %e", configPath, err)
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

	return os.WriteFile(versionsPath, updatedData, 0644)
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

	return "", fmt.Errorf(token)

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

func getCopilotAccessToken(client *http.Client) (CopilotAccessToken, error) {
	cache, err := NewExpiringCache[CopilotAccessToken]()
	if err == nil {
		var token CopilotAccessToken
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
		return CopilotAccessToken{}, fmt.Errorf("failed to create token request: %w", err)
	}

	tokenReq.Header.Set("Authorization", "token "+oAuthToken)
	tokenReq.Header.Set("Accept", "application/json")
	tokenReq.Header.Set("Editor-Version", copilotEditorVersion)
	tokenReq.Header.Set("User-Agent", copilotUserAgent)

	tokenResp, err := client.Do(tokenReq)
	if err != nil {
		return CopilotAccessToken{}, fmt.Errorf("failed to get access token: %w", err)
	}
	defer func() {
		if closeErr := tokenResp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("error closing response body: %w", closeErr)
		}
	}()

	var tokenResponse CopilotAccessToken
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokenResponse); err != nil {
		return CopilotAccessToken{}, fmt.Errorf("failed to decode token response: %w", err)
	}

	if tokenResponse.ErrorDetails != nil {
		return CopilotAccessToken{}, fmt.Errorf("token error: %s", tokenResponse.ErrorDetails.Message)
	}

	if cache != nil {
		if err := cache.Write("copilot", tokenResponse.ExpiresAt, func(w io.Writer) error {
			return json.NewEncoder(w).Encode(tokenResponse)
		}); err != nil {
			return CopilotAccessToken{}, fmt.Errorf("failed to cache token: %w", err)
		}
	}

	return tokenResponse, nil
}
