// Package cohere implements [stream.Stream] for Cohere.
package cohere

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/charmbracelet/mods/internal/proto"
	"github.com/charmbracelet/mods/internal/stream"
	cohere "github.com/cohere-ai/cohere-go/v2"
	"github.com/cohere-ai/cohere-go/v2/client"
	"github.com/cohere-ai/cohere-go/v2/core"
	"github.com/cohere-ai/cohere-go/v2/option"
)

var _ stream.Client = &Client{}

// Config represents the configuration for the Ollama API client.
type Config struct {
	AuthToken  string
	BaseURL    string
	HTTPClient *http.Client
}

// DefaultConfig returns the default configuration for the Ollama API client.
func DefaultConfig(authToken string) Config {
	return Config{
		AuthToken:  authToken,
		BaseURL:    "",
		HTTPClient: &http.Client{},
	}
}

// Client cohere client.
type Client struct {
	*client.Client
}

// New creates a new [Client] with the given [Config].
func New(config Config) *Client {
	opts := []option.RequestOption{
		client.WithToken(config.AuthToken),
		client.WithHTTPClient(config.HTTPClient),
	}

	if config.BaseURL != "" {
		opts = append(opts, client.WithBaseURL(config.BaseURL))
	}

	return &Client{
		Client: client.NewClient(opts...),
	}
}

// Request implements stream.Client.
func (c *Client) Request(ctx context.Context, request proto.Request) stream.Stream {
	s := &Stream{}
	messages := fromProtoMessages(request.Messages)
	body := &cohere.V2ChatStreamRequest{
		Model:         request.Model,
		Messages:      messages,
		Temperature:   request.Temperature,
		P:             request.TopP,
		StopSequences: request.Stop,
	}

	if request.MaxTokens != nil {
		body.MaxTokens = cohere.Int(int(*request.MaxTokens))
	}

	s.request = body
	s.done = false
	s.message = &cohere.ChatMessageV2{
		Role: "assistant",
		Assistant: &cohere.AssistantMessage{
			Content: &cohere.AssistantMessageV2Content{},
		},
	}
	s.stream, s.err = c.V2.ChatStream(ctx, s.request)
	return s
}

// Stream is a cohere stream.
type Stream struct {
	stream  *core.Stream[cohere.V2ChatStreamResponse]
	request *cohere.V2ChatStreamRequest
	err     error
	done    bool
	message *cohere.ChatMessageV2
}

// CallTools implements stream.Stream.
// Not supported.
func (s *Stream) CallTools() []proto.ToolCallStatus { return nil }

// Close implements stream.Stream.
func (s *Stream) Close() error {
	s.done = true
	return s.stream.Close() //nolint:wrapcheck
}

// Current implements stream.Stream.
func (s *Stream) Current() (proto.Chunk, error) {
	resp, err := s.stream.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			s.done = true
			return proto.Chunk{}, stream.ErrNoContent
		}
		return proto.Chunk{}, fmt.Errorf("cohere: %w", err)
	}
	if text := resp.GetContentDelta().GetDelta().GetMessage().GetContent().GetText(); text != nil {
		s.message.Assistant.Content.String += *text
		return proto.Chunk{
			Content: *text,
		}, nil
	}
	return proto.Chunk{}, stream.ErrNoContent
}

// Err implements stream.Stream.
func (s *Stream) Err() error { return s.err }

// Messages implements stream.Stream.
func (s *Stream) Messages() []proto.Message {
	return toProtoMessages(append(s.request.Messages, s.message))
}

// Next implements stream.Stream.
func (s *Stream) Next() bool {
	if s.err != nil {
		return false
	}
	return !s.done
}
