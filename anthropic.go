package main

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	openai "github.com/sashabaranov/go-openai"
)

// AnthropicClientConfig represents the configuration for the Anthropic API client.
type AnthropicClientConfig struct {
	AuthToken          string
	BaseURL            string
	HTTPClient         *http.Client
	EmptyMessagesLimit uint
}

// DefaultAnthropicConfig returns the default configuration for the Anthropic API client.
func DefaultAnthropicConfig(authToken string) AnthropicClientConfig {
	return AnthropicClientConfig{
		AuthToken:  authToken,
		HTTPClient: &http.Client{},
	}
}

// AnthropicClient is a client for the Anthropic API.
type AnthropicClient struct {
	*anthropic.Client
}

// NewAnthropicClientWithConfig creates a new [client.Client] with the given configuration.
func NewAnthropicClientWithConfig(config AnthropicClientConfig) *AnthropicClient {
	opts := []option.RequestOption{
		option.WithAPIKey(config.AuthToken),
		option.WithHTTPClient(config.HTTPClient),
	}
	if config.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(config.BaseURL))
	}
	client := anthropic.NewClient(opts...)
	return &AnthropicClient{
		Client: &client,
	}
}

// CreateChatCompletionStream — API call to create a chat completion w/
// streaming support.
func (c *AnthropicClient) CreateChatCompletionStream(
	ctx context.Context,
	request anthropic.MessageNewParams,
) (*AnthropicChatCompletionStream, error) {
	return &AnthropicChatCompletionStream{
		anthropicStreamReader: &anthropicStreamReader{
			Stream: c.Messages.NewStreaming(ctx, request),
		},
	}, nil
}

func makeAnthropicSystem(system string) []anthropic.TextBlockParam {
	if system == "" {
		return nil
	}
	return []anthropic.TextBlockParam{
		{
			Text: system,
		},
	}
}

// AnthropicChatCompletionStream represents a stream for chat completion.
type AnthropicChatCompletionStream struct {
	*anthropicStreamReader
}

type anthropicStreamReader struct {
	*ssestream.Stream[anthropic.MessageStreamEventUnion]
}

// Recv reads the next response from the stream.
func (r *anthropicStreamReader) Recv() (response openai.ChatCompletionStreamResponse, err error) {
	if err := r.Err(); err != nil {
		return openai.ChatCompletionStreamResponse{}, fmt.Errorf("anthropic: %w", err)
	}
	for r.Next() {
		event := r.Current()
		switch eventVariant := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			switch deltaVariant := eventVariant.Delta.AsAny().(type) {
			case anthropic.TextDelta:
				return openai.ChatCompletionStreamResponse{
					Choices: []openai.ChatCompletionStreamChoice{
						{
							Index: 0,
							Delta: openai.ChatCompletionStreamChoiceDelta{
								Content: deltaVariant.Text,
								Role:    "assistant",
							},
						},
					},
				}, nil
			}
		}
	}
	return openai.ChatCompletionStreamResponse{}, io.EOF
}

// Close closes the stream.
func (r *anthropicStreamReader) Close() error {
	if err := r.Stream.Close(); err != nil {
		return fmt.Errorf("anthropic: %w", err)
	}
	return nil
}
