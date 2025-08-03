package oauth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/mods/internal/cache"
	"github.com/cli/oauth"
)

// Config holds OAuth configuration settings.
type Config struct {
	Name            string
	DeviceCodeURL   string
	TokenURL        string
	ClientID        string
	Scopes          []string
	Audience        string
	UserAgent       string
	CachePath       string
	HTTPClient      *http.Client
	TokenSerializer TokenSerializer
}

// Token represents an OAuth access token with metadata.
type Token struct {
	AccessToken string            `json:"access_token"`
	TokenType   string            `json:"token_type,omitempty"`
	ExpiresIn   int               `json:"expires_in,omitempty"`
	ExpiresAt   int64             `json:"expires_at,omitempty"`
	Scope       string            `json:"scope,omitempty"`
	Audience    string            `json:"audience,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// TokenSerializer defines the interface for token serialization.
type TokenSerializer interface {
	Serialize(token Token) ([]byte, error)
	Deserialize(data []byte) (Token, error)
	GetTokenPath() string
}

// Client represents an OAuth client for device flow authentication.
type Client struct {
	config      Config
	httpClient  *http.Client
	cacheClient *cache.ExpiringCache[Token]
	token       *Token
}

// New creates a new OAuth client with the given configuration.
func New(config Config) *Client {
	httpClient := &http.Client{}
	if config.HTTPClient != nil {
		httpClient = config.HTTPClient
	}

	client := &Client{
		config:     config,
		httpClient: httpClient,
	}

	if config.CachePath != "" {
		cache, err := cache.NewExpiring[Token](config.CachePath)
		if err == nil {
			client.cacheClient = cache
		}
	}

	return client
}

// Do executes an HTTP request with OAuth authentication.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if c.config.UserAgent != "" {
		req.Header.Set("User-Agent", c.config.UserAgent)
	}
	req.Header.Set("Accept", "application/json")

	isTokenExpired := c.token != nil && c.token.ExpiresAt > 0 && c.token.ExpiresAt < time.Now().Unix()

	if c.token == nil || isTokenExpired {
		token, err := c.Auth()
		if err != nil {
			return nil, fmt.Errorf("failed to authenticate: %w", err)
		}
		c.token = &token
	}

	if c.token != nil {
		req.Header.Set("Authorization", c.token.TokenType+" "+c.token.AccessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	return resp, nil
}

// Auth performs OAuth device flow authentication and returns a token.
func (c *Client) Auth() (Token, error) {
	if c.cacheClient != nil {
		var token Token
		err := c.cacheClient.Read(c.config.Name, func(r io.Reader) error {
			return json.NewDecoder(r).Decode(&token)
		})
		if err == nil && (token.ExpiresAt == 0 || token.ExpiresAt > time.Now().Unix()) {
			return token, nil
		}
	}

	configPath := c.getTokenPath()
	existingToken, err := c.loadToken(configPath)

	tokenExists := err == nil && existingToken.AccessToken != ""
	tokenNotExpired := existingToken.ExpiresAt == 0 || existingToken.ExpiresAt > time.Now().Unix()

	if tokenExists && tokenNotExpired {
		// Token exists and is valid, cache it and return
		if c.cacheClient != nil && existingToken.ExpiresAt > 0 {
			if cacheErr := c.cacheClient.Write(c.config.Name, existingToken.ExpiresAt, func(w io.Writer) error {
				return json.NewEncoder(w).Encode(existingToken)
			}); cacheErr != nil {
				return Token{}, fmt.Errorf("failed to cache token: %w", cacheErr)
			}
		}
		return existingToken, nil
	}
	flow := &oauth.Flow{
		Host: &oauth.Host{
			DeviceCodeURL: c.config.DeviceCodeURL,
			TokenURL:      c.config.TokenURL,
		},
		ClientID:   c.config.ClientID,
		Scopes:     c.config.Scopes,
		Audience:   c.config.Audience,
		HTTPClient: c.httpClient,
		DisplayCode: func(code, url string) error {
			fmt.Fprintf(os.Stdout, "\nCopy code %s and visit %s to authenticate\n\n", code, url)
			return nil
		},
	}

	accessToken, err := flow.DeviceFlow()
	if err != nil {
		return Token{}, fmt.Errorf("oauth device flow failed: %w", err)
	}

	token := Token{
		AccessToken: accessToken.Token,
		TokenType:   accessToken.Type,
		Scope:       accessToken.Scope,
		Audience:    c.config.Audience,
	}

	if token.ExpiresAt == 0 {
		token.ExpiresAt = time.Now().Add(time.Hour).Unix()
	}

	// Save access token to config file
	if token.AccessToken != "" {
		if c.config.TokenSerializer != nil {
			if saveErr := c.saveTokenWithSerializer(token, configPath); saveErr != nil {
				// Log error but don't fail the auth process
				fmt.Fprintf(os.Stderr, "Warning: failed to save token: %v\n", saveErr)
			}
		} else {
			if saveErr := SaveToken(c.config.Name, token, configPath); saveErr != nil {
				// Log error but don't fail the auth process
				fmt.Fprintf(os.Stderr, "Warning: failed to save token: %v\n", saveErr)
			}
		}
	}

	// Cache the token if cache client is available
	if c.cacheClient != nil && token.ExpiresAt > 0 {
		if err := c.cacheClient.Write(c.config.Name, token.ExpiresAt, func(w io.Writer) error {
			return json.NewEncoder(w).Encode(token)
		}); err != nil {
			return Token{}, fmt.Errorf("failed to cache token: %w", err)
		}
	}

	return token, nil
}

func (c *Client) getTokenPath() string {
	if c.config.TokenSerializer != nil {
		return c.config.TokenSerializer.GetTokenPath()
	}
	return GetDefaultConfigPath()
}

func (c *Client) loadToken(configPath string) (Token, error) {
	if c.config.TokenSerializer != nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return Token{}, fmt.Errorf("failed to read token configuration file at %s: %w", configPath, err)
		}

		token, err := c.config.TokenSerializer.Deserialize(data)
		if err != nil {
			return Token{}, fmt.Errorf("failed to deserialize token: %w", err)
		}

		return token, nil
	}

	return LoadToken(c.config.Name, configPath)
}

// GetToken retrieves a valid OAuth token, performing authentication if necessary.
func (c *Client) GetToken() (Token, error) {
	return c.Auth()
}

// SetToken sets the current token for the client.
func (c *Client) SetToken(token Token) {
	c.token = &token
}

func (c *Client) saveTokenWithSerializer(token Token, configPath string) error {
	data, err := c.config.TokenSerializer.Serialize(token)
	if err != nil {
		return fmt.Errorf("failed to serialize token: %w", err)
	}

	configDir := filepath.Dir(configPath)
	if err = os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	err = os.WriteFile(configPath, data, 0o600)
	if err != nil {
		return fmt.Errorf("error writing token to %s: %w", configPath, err)
	}

	return nil
}

// ClearToken removes the current token and clears it from cache.
func (c *Client) ClearToken() error {
	c.token = nil
	if c.cacheClient != nil {
		return c.cacheClient.Delete(c.config.Name)
	}
	return nil
}

// Login performs the OAuth device flow login and saves the token.
func Login(config Config) (Token, error) {
	client := New(config)
	return client.Auth()
}
