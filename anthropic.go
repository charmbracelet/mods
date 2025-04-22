package main

import (
	"context"
	"fmt"
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
		BaseURL:    "",
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

// AnthropicChatCompletionStream represents a stream for chat completion.
type AnthropicChatCompletionStream struct {
	*anthropicStreamReader
}

type anthropicStreamReader struct {
	*ssestream.Stream[anthropic.MessageStreamEventUnion]
}

// Recv reads the next response from the stream.
func (stream *anthropicStreamReader) Recv() (response openai.ChatCompletionStreamResponse, err error) {
	return stream.processMessages()
}

// Close closes the stream.
func (stream *anthropicStreamReader) Close() error {
	if err := stream.Stream.Close(); err != nil {
		return fmt.Errorf("anthropic: %w", err)
	}
	return nil
}

func (stream *anthropicStreamReader) processMessages() (openai.ChatCompletionStreamResponse, error) {
	for stream.Next() {
		event := stream.Current()
		switch eventVariant := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			switch deltaVariant := eventVariant.Delta.AsAny().(type) {
			case anthropic.TextDelta:
				// NOTE: Leverage the existing logic based on OpenAI ChatCompletionStreamResponse by
				//       converting the Anthropic events into them.
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
	return openai.ChatCompletionStreamResponse{}, nil
}

// CreateChatCompletionStream — API call to create a chat completion w/ streaming
// support.
func (c *AnthropicClient) CreateChatCompletionStream(
	ctx context.Context,
	request anthropic.MessageNewParams,
) (*AnthropicChatCompletionStream, error) {
	return &AnthropicChatCompletionStream{
		anthropicStreamReader: &anthropicStreamReader{
			Stream: c.Client.Messages.NewStreaming(ctx, request),
		},
	}, nil
}

func makeAnthropicSystem(system string) []anthropic.TextBlockParam {
	return []anthropic.TextBlockParam{
		{
			Text: system,
		},
	}
}
