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
		Tools:         fromMCPTools(request.Tools),
	}

	if request.MaxTokens != nil {
		body.MaxTokens = cohere.Int(int(*request.MaxTokens))
	}

	s.request = body
	s.done = false
	s.toolCall = request.ToolCaller
	s.message = &cohere.ChatMessageV2{
		Role:      "assistant",
		Assistant: &cohere.AssistantMessage{},
	}
	s.messages = request.Messages
	s.stream, s.err = c.V2.ChatStream(ctx, s.request)
	s.factory = func() (*core.Stream[cohere.V2ChatStreamResponse], error) {
		return c.V2.ChatStream(ctx, s.request)
	}
	return s
}

// Stream is a cohere stream.
type Stream struct {
	stream   *core.Stream[cohere.V2ChatStreamResponse]
	factory  func() (*core.Stream[cohere.V2ChatStreamResponse], error)
	request  *cohere.V2ChatStreamRequest
	err      error
	done     bool
	message  *cohere.ChatMessageV2
	toolCall func(name string, data []byte) (string, error)
	messages []proto.Message
}

// CallTools implements stream.Stream.
func (s *Stream) CallTools() []proto.ToolCallStatus {
	calls := s.messages[len(s.messages)-1].ToolCalls

	statuses := make([]proto.ToolCallStatus, 0, len(calls))
	for _, call := range calls {
		msg, status := stream.CallTool(
			call.ID,
			call.Function.Name,
			call.Function.Arguments,
			s.toolCall,
		)
		resp := &cohere.ChatMessageV2{
			Role: "tool",
			Tool: &cohere.ToolMessageV2{
				Content: &cohere.ToolMessageV2Content{
					ToolContentList: []*cohere.ToolContent{
						{
							Type: "document",
							Document: &cohere.DocumentContent{
								Document: &cohere.Document{
									Data: map[string]any{"data": msg.Content},
									Id:   cohere.String("0"),
								},
							},
						},
					},
				},
				ToolCallId: call.ID,
			},
		}
		s.request.Messages = append(s.request.Messages, resp)
		s.messages = append(s.messages, msg)
		statuses = append(statuses, status)
	}

	s.done = false
	s.stream, s.err = s.factory()

	return statuses
}

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
	switch resp.GetType() {
	case "content-delta":
		if text := resp.GetContentDelta().GetDelta().GetMessage().GetContent().GetText(); text != nil {
			if s.message.Assistant.Content == nil {
				s.message.Assistant.Content = new(cohere.AssistantMessageV2Content)
			}
			s.message.Assistant.Content.String += *text
			return proto.Chunk{
				Content: *text,
			}, nil
		}
	case "tool-call-start":
		if toolCalls := resp.GetToolCallStart().GetDelta().GetMessage().GetToolCalls(); toolCalls != nil {
			s.message.Assistant.ToolCalls = append(s.message.Assistant.ToolCalls, toolCalls)
		}
	case "tool-plan-delta":
		if toolPlan := resp.GetToolPlanDelta().GetDelta().GetMessage().GetToolPlan(); toolPlan != nil {
			if s.message.Assistant.ToolPlan == nil {
				s.message.Assistant.ToolPlan = cohere.String("")
			}
			*s.message.Assistant.ToolPlan += *toolPlan
		}
	case "tool-call-delta":
		if toolCalls := resp.GetToolCallDelta().GetDelta().GetMessage().GetToolCalls(); toolCalls != nil {
			toolCall := s.message.Assistant.ToolCalls[len(s.message.Assistant.ToolCalls)-1]
			if toolCall.Function.Arguments == nil {
				toolCall.Function.Arguments = cohere.String("")
			}
			*toolCall.Function.Arguments += *toolCalls.Function.Arguments
		}
	case "tool-call-end":
		toolCall := s.message.Assistant.ToolCalls[len(s.message.Assistant.ToolCalls)-1]
		if *toolCall.Function.Arguments == "" {
			*toolCall.Function.Arguments = "{}"
		}
	}
	return proto.Chunk{}, stream.ErrNoContent
}

// Err implements stream.Stream.
func (s *Stream) Err() error { return s.err }

// Messages implements stream.Stream.
func (s *Stream) Messages() []proto.Message {
	return s.messages
}

// Next implements stream.Stream.
func (s *Stream) Next() bool {
	if s.done {
		s.request.Messages = append(s.request.Messages, s.message)
		s.messages = append(s.messages, toProtoMessage(s.message))
		s.message = &cohere.ChatMessageV2{
			Role:      "assistant",
			Assistant: new(cohere.AssistantMessage),
		}
	}

	if s.err != nil {
		return false
	}

	return !s.done
}
