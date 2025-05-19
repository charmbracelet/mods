package anthropic

import (
	"context"
	"net/http"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	"github.com/charmbracelet/mods/proto"
	"github.com/charmbracelet/mods/stream"
)

var _ stream.Client = &Client{}

// Client is a client for the Anthropic API.
type Client struct {
	*anthropic.Client
}

// Request implements stream.Client.
func (c *Client) Request(ctx context.Context, request proto.Request) stream.Stream {
	system, messages := fromProtoMessages(request.Messages)
	body := anthropic.MessageNewParams{
		Model:         request.Model,
		Messages:      messages,
		System:        system,
		Tools:         fromMCPTools(request.Tools),
		StopSequences: request.Stop,
	}

	if request.MaxTokens != nil {
		body.MaxTokens = int64(*request.MaxTokens)
	} else {
		body.MaxTokens = 4096 // TODO: double check this
	}

	if request.Temperature != nil {
		body.Temperature = anthropic.Float(*request.Temperature)
	}

	if request.TopP != nil {
		body.TopP = anthropic.Float(*request.TopP)
	}

	s := &Stream{
		stream:   c.Messages.NewStreaming(ctx, body),
		request:  body,
		toolCall: request.ToolCaller,
	}

	s.factory = func() *ssestream.Stream[anthropic.MessageStreamEventUnion] {
		return c.Messages.NewStreaming(ctx, s.request)
	}
	return s
}

// Config represents the configuration for the Anthropic API client.
type Config struct {
	AuthToken          string
	BaseURL            string
	HTTPClient         *http.Client
	EmptyMessagesLimit uint
}

// DefaultConfig returns the default configuration for the Anthropic API client.
func DefaultConfig(authToken string) Config {
	return Config{
		AuthToken:  authToken,
		HTTPClient: &http.Client{},
	}
}

func New(config Config) *Client {
	opts := []option.RequestOption{
		option.WithAPIKey(config.AuthToken),
		option.WithHTTPClient(config.HTTPClient),
	}
	if config.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(strings.TrimSuffix(config.BaseURL, "/v1")))
	}
	client := anthropic.NewClient(opts...)
	return &Client{
		Client: &client,
	}
}

// Stream represents a stream for chat completion.
type Stream struct {
	done     bool
	stream   *ssestream.Stream[anthropic.MessageStreamEventUnion]
	request  anthropic.MessageNewParams
	factory  func() *ssestream.Stream[anthropic.MessageStreamEventUnion]
	message  anthropic.Message
	toolCall func(name string, data []byte) (string, error)
}

// CallTools implements stream.Stream.
func (s *Stream) CallTools() []proto.ToolCallStatus {
	var result []proto.ToolCallStatus
	for _, block := range s.message.Content {
		switch call := block.AsAny().(type) {
		case anthropic.ToolUseBlock:
			content, err := s.toolCall(call.Name, []byte(call.JSON.Input.Raw()))
			s.request.Messages = append(
				s.request.Messages,
				anthropic.NewUserMessage(
					anthropic.NewToolResultBlock(
						call.ID,
						content,
						err != nil,
					),
				),
			)
			result = append(result, proto.ToolCallStatus{
				Name: call.Name,
				Err:  err,
			})
		}
	}
	return result
}

// Close implements stream.Stream.
func (s *Stream) Close() error { return s.stream.Close() }

// Current implements stream.Stream.
func (s *Stream) Current() (proto.Chunk, error) {
	event := s.stream.Current()
	if err := s.message.Accumulate(event); err != nil {
		return proto.Chunk{}, err
	}
	switch eventVariant := event.AsAny().(type) {
	case anthropic.ContentBlockDeltaEvent:
		switch deltaVariant := eventVariant.Delta.AsAny().(type) {
		case anthropic.TextDelta:
			return proto.Chunk{
				Content: deltaVariant.Text,
			}, nil
		}
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
		s.message = anthropic.Message{}
	}

	if s.stream.Next() {
		return true
	}

	s.done = true
	s.request.Messages = append(s.request.Messages, s.message.ToParam())

	return false
}
