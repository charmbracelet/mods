// Package copilot provides token serialization for GitHub Copilot OAuth tokens.
package copilot

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/charmbracelet/mods/internal/oauth"
)

// TokenSerializer handles serialization of GitHub Copilot OAuth tokens.
type TokenSerializer struct {
	providerName string
}

// NewCopilotTokenSerializer creates a new token serializer for the given provider.
func NewCopilotTokenSerializer(providerName string) *TokenSerializer {
	return &TokenSerializer{
		providerName: providerName,
	}
}

// Serialize converts an OAuth token to GitHub Copilot's JSON format.
func (c *TokenSerializer) Serialize(token oauth.Token) ([]byte, error) {
	oAuthToken := OAuthToken{
		GithubWrapper: OAuthTokenWrapper{
			User:        "",
			OAuthToken:  token.AccessToken,
			GithubAppID: copilotClientID,
		},
	}

	data, err := json.Marshal(oAuthToken)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal copilot token: %w", err)
	}

	return data, nil
}

// Deserialize converts GitHub Copilot's JSON format back to an OAuth token.
func (c *TokenSerializer) Deserialize(data []byte) (oauth.Token, error) {
	var oAuthToken OAuthToken

	if err := json.Unmarshal(data, &oAuthToken); err != nil {
		return oauth.Token{}, fmt.Errorf("failed to unmarshal copilot token: %w", err)
	}

	token := oauth.Token{
		AccessToken: oAuthToken.GithubWrapper.OAuthToken,
	}

	return token, nil
}

// GetTokenPath returns the path where GitHub Copilot tokens are stored.
func (c *TokenSerializer) GetTokenPath() string {
	configPath := filepath.Join(xdg.ConfigHome, "github-copilot")
	return filepath.Join(configPath, "apps.json")
}
