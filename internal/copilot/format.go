package copilot

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/charmbracelet/mods/internal/oauth"
)

type CopilotTokenSerializer struct {
	providerName string
}

func NewCopilotTokenSerializer(providerName string) *CopilotTokenSerializer {
	return &CopilotTokenSerializer{
		providerName: providerName,
	}
}

func (c *CopilotTokenSerializer) Serialize(token oauth.Token) ([]byte, error) {
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

func (c *CopilotTokenSerializer) Deserialize(data []byte) (oauth.Token, error) {
	var oAuthToken OAuthToken

	if err := json.Unmarshal(data, &oAuthToken); err != nil {
		return oauth.Token{}, fmt.Errorf("failed to unmarshal copilot token: %w", err)
	}

	token := oauth.Token{
		AccessToken: oAuthToken.GithubWrapper.OAuthToken,
	}

	return token, nil
}

// GetTokenPath returns the path where GitHub Copilot tokens are stored
func (c *CopilotTokenSerializer) GetTokenPath() string {
	configPath := filepath.Join(xdg.ConfigHome, "github-copilot")
	return filepath.Join(configPath, "apps.json")
}
