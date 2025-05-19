package openai

import (
	"context"
	"net/http"
	"strings"

	"github.com/charmbracelet/mods/proto"
	"github.com/charmbracelet/mods/stream"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/shared"
)

var _ stream.Client = &Client{}

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// Client is the openai client.
type Client struct {
	*openai.Client
}

// Config represents the configuration for the OpenAI API client.
type Config struct {
	AuthToken  string
	BaseURL    string
	HTTPClient HTTPClient
	APIType    string
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
			OfChatCompletionNewsStopArray: request.Stop,
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
	}
	s.factory = func() *ssestream.Stream[openai.ChatCompletionChunk] {
		return c.Chat.Completions.NewStreaming(ctx, s.request)
	}
	return s
}

type Stream struct {
	done     bool
	request  openai.ChatCompletionNewParams
	stream   *ssestream.Stream[openai.ChatCompletionChunk]
	factory  func() *ssestream.Stream[openai.ChatCompletionChunk]
	message  openai.ChatCompletionAccumulator
	toolCall func(name string, data []byte) (string, error)
}

// CallTools implements stream.Stream.
func (s *Stream) CallTools() []proto.ToolCallStatus {
	var result []proto.ToolCallStatus
	for _, call := range s.message.Choices[0].Message.ToolCalls {
		content, err := s.toolCall(call.Function.Name, []byte(call.Function.Arguments))
		s.request.Messages = append(s.request.Messages, openai.ToolMessage(content, call.ID))
		result = append(result, proto.ToolCallStatus{
			Name: call.Function.Name,
			Err:  err,
		})
	}
	return result
}

// Close implements stream.Stream.
func (s *Stream) Close() error { return s.stream.Close() }

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
func (s *Stream) Err() error { return s.stream.Err() }

// Messages implements stream.Stream.
func (s *Stream) Messages() []proto.Message {
	return toProtoMessages(s.request.Messages)
}

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
		s.request.Messages = append(s.request.Messages, s.message.Choices[0].Message.ToParam())
	}

	return false
}
