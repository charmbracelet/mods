// Package openai implements [stream.Stream] for OpenAI.
package openai

import (
	"context"
	"net/http"
	"strings"

	"github.com/charmbracelet/mods/internal/proto"
	"github.com/charmbracelet/mods/internal/stream"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/shared"
)

var _ stream.Client = &Client{}

// Client is the openai client.
type Client struct {
	*openai.Client
}

// Config represents the configuration for the OpenAI API client.
type Config struct {
	AuthToken  string
	BaseURL    string
	HTTPClient interface {
		Do(*http.Request) (*http.Response, error)
	}
	APIType string
}

// DefaultConfig returns the default configuration for the OpenAI API client.
func DefaultConfig(authToken string) Config {
	return Config{
		AuthToken: authToken,
	}
}

// New creates a new [Client] with the given [Config].
func New(config Config) *Client {
	opts := []option.RequestOption{}

	if config.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(config.HTTPClient))
	}

	if config.APIType == "azure-ad" {
		opts = append(opts, azure.WithAPIKey(config.AuthToken))
		if config.BaseURL != "" {
			opts = append(opts, azure.WithEndpoint(config.BaseURL, "v1"))
		}
	} else {
		opts = append(opts, option.WithAPIKey(config.AuthToken))
		if config.BaseURL != "" {
			opts = append(opts, option.WithBaseURL(config.BaseURL))
		}
	}
	client := openai.NewClient(opts...)
	return &Client{
		Client: &client,
	}
}

// Request makes a new request and returns a stream.
func (c *Client) Request(ctx context.Context, request proto.Request) stream.Stream {
	body := openai.ChatCompletionNewParams{
		Model:    request.Model,
		User:     openai.String(request.User),
		Messages: fromProtoMessages(request.Messages),
		Tools:    fromMCPTools(request.Tools),
	}

	if request.API != "perplexity" || !strings.Contains(request.Model, "online") {
		if request.Temperature != nil {
			body.Temperature = openai.Float(*request.Temperature)
		}
		if request.TopP != nil {
			body.TopP = openai.Float(*request.TopP)
		}
		body.Stop = openai.ChatCompletionNewParamsStopUnion{
			OfStringArray: request.Stop,
		}
		if request.MaxTokens != nil {
			body.MaxTokens = openai.Int(*request.MaxTokens)
		}
		if request.API == "openai" && request.ResponseFormat != nil && *request.ResponseFormat == "json" {
			body.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONObject: &shared.ResponseFormatJSONObjectParam{},
			}
		}
	}

	s := &Stream{
		stream:   c.Chat.Completions.NewStreaming(ctx, body),
		request:  body,
		toolCall: request.ToolCaller,
		messages: request.Messages,
	}
	s.factory = func() *ssestream.Stream[openai.ChatCompletionChunk] {
		return c.Chat.Completions.NewStreaming(ctx, s.request)
	}
	return s
}

// Stream openai stream.
type Stream struct {
	done     bool
	request  openai.ChatCompletionNewParams
	stream   *ssestream.Stream[openai.ChatCompletionChunk]
	factory  func() *ssestream.Stream[openai.ChatCompletionChunk]
	message  openai.ChatCompletionAccumulator
	messages []proto.Message
	toolCall func(name string, data []byte) (string, error)
}

// CallTools implements stream.Stream.
func (s *Stream) CallTools() []proto.ToolCallStatus {
	calls := s.message.Choices[0].Message.ToolCalls
	statuses := make([]proto.ToolCallStatus, 0, len(calls))
	for _, call := range calls {
		msg, status := stream.CallTool(
			call.ID,
			call.Function.Name,
			[]byte(call.Function.Arguments),
			s.toolCall,
		)
		resp := openai.ToolMessage(
			msg.Content,
			call.ID,
		)
		s.request.Messages = append(s.request.Messages, resp)
		s.messages = append(s.messages, msg)
		statuses = append(statuses, status)
	}
	return statuses
}

// Close implements stream.Stream.
func (s *Stream) Close() error { return s.stream.Close() } //nolint:wrapcheck

// Current implements stream.Stream.
func (s *Stream) Current() (proto.Chunk, error) {
	event := s.stream.Current()
	s.message.AddChunk(event)
	if len(event.Choices) > 0 {
		return proto.Chunk{
			Content: event.Choices[0].Delta.Content,
		}, nil
	}
	return proto.Chunk{}, stream.ErrNoContent
}

// Err implements stream.Stream.
func (s *Stream) Err() error { return s.stream.Err() } //nolint:wrapcheck

// Messages implements stream.Stream.
func (s *Stream) Messages() []proto.Message { return s.messages }

// Next implements stream.Stream.
func (s *Stream) Next() bool {
	if s.done {
		s.done = false
		s.stream = s.factory()
		s.message = openai.ChatCompletionAccumulator{}
	}

	if s.stream.Next() {
		return true
	}

	s.done = true
	if len(s.message.Choices) > 0 {
		msg := s.message.Choices[0].Message.ToParam()
		s.request.Messages = append(s.request.Messages, msg)
		s.messages = append(s.messages, toProtoMessage(msg))
	}

	return false
}
