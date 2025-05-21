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
	history, message := fromProtoMessages(request.Messages)
	body := &cohere.ChatStreamRequest{
		Model:         cohere.String(request.Model),
		Message:       message,
		ChatHistory:   history,
		Temperature:   request.Temperature,
		P:             request.TopP,
		StopSequences: request.Stop,
	}

	if request.MaxTokens != nil {
		body.MaxTokens = cohere.Int(int(*request.MaxTokens))
	}

	s.request = body
	s.done = false
	s.message = &cohere.Message{
		Role:    "CHATBOT",
		Chatbot: &cohere.ChatMessage{},
	}
	s.stream, s.err = c.ChatStream(ctx, s.request)
	return s
}

// Stream is a cohere stream.
type Stream struct {
	stream  *core.Stream[cohere.StreamedChatResponse]
	request *cohere.ChatStreamRequest
	err     error
	done    bool
	message *cohere.Message
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
	if errors.Is(err, io.EOF) {
		return proto.Chunk{}, stream.ErrNoContent
	}
	if err != nil {
		return proto.Chunk{}, fmt.Errorf("cohere: %w", err)
	}
	switch resp.EventType {
	case "text-generation":
		s.message.Chatbot.Message += resp.TextGeneration.Text
		return proto.Chunk{
			Content: resp.TextGeneration.Text,
		}, nil
	}
	return proto.Chunk{}, stream.ErrNoContent
}

// Err implements stream.Stream.
func (s *Stream) Err() error { return s.err }

// Messages implements stream.Stream.
func (s *Stream) Messages() []proto.Message {
	return toProtoMessages(append(s.request.ChatHistory, &cohere.Message{
		Role: "USER",
		User: &cohere.ChatMessage{
			Message: s.request.Message,
		},
	}, s.message))
}

// Next implements stream.Stream.
func (s *Stream) Next() bool {
	if s.err != nil {
		return false
	}
	return !s.done
}
