// Package oauth provides token formatting and storage utilities for OAuth authentication.
package oauth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

// TokenFile represents the structure of the OAuth token configuration file.
type TokenFile struct {
	OAuthTokens map[string]TokenWrapper `json:"oauth_tokens,omitempty"`
}

// TokenWrapper wraps token data with additional metadata for storage.
type TokenWrapper struct {
	Token    string            `json:"token"`
	User     string            `json:"user,omitempty"`
	AppID    string            `json:"app_id,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// SaveToken saves an OAuth token to the specified configuration file.
func SaveToken(providerName string, token Token, configPath string) error {
	tokenFile := TokenFile{
		OAuthTokens: make(map[string]TokenWrapper),
	}

	if data, err := os.ReadFile(configPath); err == nil {
		if jsonErr := json.Unmarshal(data, &tokenFile); jsonErr != nil {
			tokenFile = TokenFile{
				OAuthTokens: make(map[string]TokenWrapper),
			}
		}
	}

	wrapper := TokenWrapper{
		Token:    token.AccessToken,
		User:     token.Metadata["user"],
		AppID:    token.Metadata["app_id"],
		Metadata: token.Metadata,
	}

	tokenFile.OAuthTokens[providerName] = wrapper

	fileContent, err := json.MarshalIndent(tokenFile, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling token file: %w", err)
	}

	configDir := filepath.Dir(configPath)
	if err = os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	err = os.WriteFile(configPath, fileContent, 0o600)
	if err != nil {
		return fmt.Errorf("error writing token to %s: %w", configPath, err)
	}

	return nil
}

// SaveRefreshToken saves a refresh token to the specified configuration file.
func SaveRefreshToken(providerName string, refreshToken string, configPath string) error {
	tokenFile := TokenFile{
		OAuthTokens: make(map[string]TokenWrapper),
	}

	if data, err := os.ReadFile(configPath); err == nil {
		if jsonErr := json.Unmarshal(data, &tokenFile); jsonErr != nil {
			tokenFile = TokenFile{
				OAuthTokens: make(map[string]TokenWrapper),
			}
		}
	}

	wrapper := TokenWrapper{
		Token: refreshToken,
	}

	tokenFile.OAuthTokens[providerName] = wrapper

	fileContent, err := json.MarshalIndent(tokenFile, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling refresh token file: %w", err)
	}

	configDir := filepath.Dir(configPath)
	if err = os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	err = os.WriteFile(configPath, fileContent, 0o600)
	if err != nil {
		return fmt.Errorf("error writing refresh token to %s: %w", configPath, err)
	}

	return nil
}

// LoadToken loads an OAuth token from the specified configuration file.
func LoadToken(providerName string, configPath string) (Token, error) {
	var tokenFile TokenFile

	data, err := os.ReadFile(configPath)
	if err != nil {
		return Token{}, fmt.Errorf("failed to read token configuration file at %s: %w", configPath, err)
	}

	if err := json.Unmarshal(data, &tokenFile); err != nil {
		return Token{}, fmt.Errorf("failed to parse token configuration file at %s: %w", configPath, err)
	}

	wrapper, exists := tokenFile.OAuthTokens[providerName]
	if !exists {
		return Token{}, fmt.Errorf("no token found for provider %s", providerName)
	}

	token := Token{
		AccessToken: wrapper.Token,
		Metadata:    wrapper.Metadata,
	}

	return token, nil
}

// GetDefaultConfigPath returns the default path for OAuth token storage.
func GetDefaultConfigPath() string {
	configPath := filepath.Join(xdg.ConfigHome, "mods")
	return filepath.Join(configPath, "oauth_tokens.json")
}
