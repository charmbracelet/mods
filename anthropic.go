package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

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
		opts = append(opts, option.WithBaseURL(strings.TrimSuffix(config.BaseURL, "/v1")))
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
) *AnthropicChatCompletionStream {
	s := &AnthropicChatCompletionStream{
		stream:  c.Messages.NewStreaming(ctx, request),
		request: request,
	}

	s.factory = func() *ssestream.Stream[anthropic.MessageStreamEventUnion] {
		return c.Messages.NewStreaming(ctx, s.request)
	}

	return s
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
	stream  *ssestream.Stream[anthropic.MessageStreamEventUnion]
	request anthropic.MessageNewParams
	factory func() *ssestream.Stream[anthropic.MessageStreamEventUnion]
	message anthropic.Message
}

// Recv reads the next response from the stream.
func (r *AnthropicChatCompletionStream) Recv() (response openai.ChatCompletionStreamResponse, err error) {
	if r.stream == nil {
		r.stream = r.factory()
		r.message = anthropic.Message{}
	}

	if r.stream.Next() {
		event := r.stream.Current()
		if err := r.message.Accumulate(event); err != nil {
			return openai.ChatCompletionStreamResponse{}, fmt.Errorf("anthropic: %w", err)
		}
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
		return openai.ChatCompletionStreamResponse{}, errNoContent
	}
	if err := r.stream.Err(); err != nil {
		return openai.ChatCompletionStreamResponse{}, fmt.Errorf("anthropic: %w", err)
	}
	if err := r.stream.Close(); err != nil {
		return openai.ChatCompletionStreamResponse{}, fmt.Errorf("anthropic: %w", err)
	}
	r.request.Messages = append(r.request.Messages, r.message.ToParam())

	toolResults := []anthropic.ContentBlockParamUnion{}

	var sb strings.Builder
	_, _ = sb.WriteString("\n\n")
	for _, block := range r.message.Content {
		switch variant := block.AsAny().(type) {
		case anthropic.ToolUseBlock:
			content, err := toolCall(variant.Name, []byte(variant.JSON.Input.Raw()))
			toolResults = append(toolResults, anthropic.NewToolResultBlock(block.ID, content, err != nil))
			_, _ = sb.WriteString("> Called tool: `" + variant.Name + "`")
			if err != nil {
				_, _ = sb.WriteString(" (failed: `" + err.Error() + "`)")
			}
			_, _ = sb.WriteString("\n")
		}
	}
	_, _ = sb.WriteString("\n")

	if len(toolResults) == 0 {
		return openai.ChatCompletionStreamResponse{}, io.EOF
	}

	msg := anthropic.NewUserMessage(toolResults...)
	r.request.Messages = append(r.request.Messages, msg)
	r.stream = nil

	return openai.ChatCompletionStreamResponse{
		Choices: []openai.ChatCompletionStreamChoice{
			{
				Index: 0,
				Delta: openai.ChatCompletionStreamChoiceDelta{
					Content: sb.String(),
					Role:    openai.ChatMessageRoleTool,
				},
			},
		},
	}, nil
}

// Close closes the stream.
func (r *AnthropicChatCompletionStream) Close() error {
	if r.stream == nil {
		return nil
	}
	if err := r.stream.Close(); err != nil {
		return fmt.Errorf("anthropic: %w", err)
	}
	r.stream = nil
	return nil
}
