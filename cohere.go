package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	cohere "github.com/cohere-ai/cohere-go/v2"
	"github.com/cohere-ai/cohere-go/v2/client"
	"github.com/cohere-ai/cohere-go/v2/core"
	"github.com/cohere-ai/cohere-go/v2/option"
	openai "github.com/sashabaranov/go-openai"
)

// CohereClientConfig represents the configuration for the Cohere API client.
type CohereClientConfig struct {
	AuthToken          string
	BaseURL            string
	HTTPClient         *http.Client
	EmptyMessagesLimit uint
}

// DefaultCohereConfig returns the default configuration for the Cohere API client.
func DefaultCohereConfig(authToken string) CohereClientConfig {
	return CohereClientConfig{
		AuthToken:  authToken,
		BaseURL:    "",
		HTTPClient: &http.Client{},
	}
}

// CohereClient is a client for the Cohere API.
type CohereClient struct {
	*client.Client
}

// NewCohereClient creates a new [client.Client] with the given configuration.
func NewCohereClientWithConfig(config CohereClientConfig) *CohereClient {
	opts := []option.RequestOption{
		client.WithToken(config.AuthToken),
		client.WithHTTPClient(config.HTTPClient),
	}

	if config.BaseURL != "" {
		opts = append(opts, client.WithBaseURL(config.BaseURL))
	}

	return &CohereClient{
		Client: client.NewClient(opts...),
	}
}

// CohereChatCompletionStream represents a stream for chat completion.
type CohereChatCompletionStream struct {
	*cohereStreamReader
}

type cohereStreamReader struct {
	*core.Stream[cohere.StreamedChatResponse]
}

// Recv reads the next response from the stream.
func (stream *cohereStreamReader) Recv() (response openai.ChatCompletionStreamResponse, err error) {
	return stream.processMessages()
}

// Close closes the stream.
func (stream *cohereStreamReader) Close() error {
	if err := stream.Stream.Close(); err != nil {
		return fmt.Errorf("cohere: %w", err)
	}
	return nil
}

func (stream *cohereStreamReader) processMessages() (openai.ChatCompletionStreamResponse, error) {
	for {
		message, err := stream.Stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return *new(openai.ChatCompletionStreamResponse), io.EOF
			}
			return *new(openai.ChatCompletionStreamResponse), fmt.Errorf("cohere: %w", err)
		}

		if message.EventType != "text-generation" {
			continue
		}

		// NOTE: Leverage the existing logic based on OpenAI ChatCompletionStreamResponse by
		//       converting the Cohere events into them.
		response := openai.ChatCompletionStreamResponse{
			Choices: []openai.ChatCompletionStreamChoice{
				{
					Index: 0,
					Delta: openai.ChatCompletionStreamChoiceDelta{
						Content: message.TextGeneration.Text,
						Role:    "assistant",
					},
				},
			},
		}

		return response, nil
	}
}

// CreateChatCompletionStream — API call to create a chat completion w/ streaming
// support.
func (c *CohereClient) CreateChatCompletionStream(
	ctx context.Context,
	request *cohere.ChatStreamRequest,
) (stream *CohereChatCompletionStream, err error) {
	resp, err := c.ChatStream(ctx, request)
	if err != nil {
		return
	}

	stream = &CohereChatCompletionStream{
		cohereStreamReader: &cohereStreamReader{
			Stream: resp,
		},
	}
	return
}

// CohereToOpenAIAPIError attempts to convert a Cohere API error into
// an OpenAI API error to later reuse the existing error handling logic.
func CohereToOpenAIAPIError(err error) error {
	ce := &core.APIError{}
	if !errors.As(err, &ce) {
		return err
	}

	unwrapped := ce.Unwrap()
	if unwrapped == nil {
		unwrapped = err
	}

	var message string
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(unwrapped.Error()), &body); err == nil {
		message, _ = body["message"].(string)
	}

	if message == "" {
		message = unwrapped.Error()
	}

	return &openai.APIError{
		HTTPStatusCode: ce.StatusCode,
		Message:        message,
	}
}
