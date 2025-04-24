package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/shared/constant"
	oldopenai "github.com/sashabaranov/go-openai"
)

type OpenAIClient struct {
	*openai.Client
}

// OpenAIClientConfig represents the configuration for the OpenAI API client.
type OpenAIClientConfig struct {
	AuthToken  string
	BaseURL    string
	HTTPClient *http.Client
	APIType    string
}

// DefaultOpenAIConfig returns the default configuration for the OpenAI API client.
func DefaultOpenAIConfig(authToken string) OpenAIClientConfig {
	return OpenAIClientConfig{
		AuthToken:  authToken,
		HTTPClient: &http.Client{},
	}
}

// NewOpenAIClientWithConfig creates a new [client.Client] with the given configuration.
func NewOpenAIClientWithConfig(config OpenAIClientConfig) *OpenAIClient {
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
	return &OpenAIClient{
		Client: &client,
	}
}

func (c *OpenAIClient) CreateChatCompletionStream(
	ctx context.Context,
	request openai.ChatCompletionNewParams,
) *OpenAIChatCompletionStream {
	s := &OpenAIChatCompletionStream{
		stream:  c.Chat.Completions.NewStreaming(ctx, request),
		request: request,
	}
	s.factory = func() *ssestream.Stream[openai.ChatCompletionChunk] {
		return c.Chat.Completions.NewStreaming(ctx, s.request)
	}
	return s
}

type OpenAIChatCompletionStream struct {
	request openai.ChatCompletionNewParams
	stream  *ssestream.Stream[openai.ChatCompletionChunk]
	factory func() *ssestream.Stream[openai.ChatCompletionChunk]
	message openai.ChatCompletionAccumulator
}

// Close implements chatCompletionReceiver.
func (r *OpenAIChatCompletionStream) Close() error {
	if r.stream == nil {
		return nil
	}
	if err := r.stream.Close(); err != nil {
		return fmt.Errorf("openai: %w", err)
	}
	r.stream = nil
	return nil
}

// Recv implements chatCompletionReceiver.
func (r *OpenAIChatCompletionStream) Recv() (oldopenai.ChatCompletionStreamResponse, error) {
	if r.stream == nil {
		r.stream = r.factory()
		r.message = openai.ChatCompletionAccumulator{}
	}

	if r.stream.Next() {
		event := r.stream.Current()
		if !r.message.AddChunk(event) {
			return oldopenai.ChatCompletionStreamResponse{}, errors.New("openai: could not accumulate chunk")
		}
		if len(event.Choices) > 0 {
			return oldopenai.ChatCompletionStreamResponse{
				Choices: []oldopenai.ChatCompletionStreamChoice{
					{
						Index: 0,
						Delta: oldopenai.ChatCompletionStreamChoiceDelta{
							Content: event.Choices[0].Delta.Content,
							Role:    oldopenai.ChatMessageRoleAssistant,
						},
					},
				},
			}, nil
		}
		return oldopenai.ChatCompletionStreamResponse{}, errNoContent
	}
	if err := r.stream.Err(); err != nil {
		return oldopenai.ChatCompletionStreamResponse{}, fmt.Errorf("anthropic: %w", err)
	}
	if err := r.stream.Close(); err != nil {
		return oldopenai.ChatCompletionStreamResponse{}, fmt.Errorf("anthropic: %w", err)
	}
	r.request.Messages = append(r.request.Messages, r.message.Choices[0].Message.ToParam())

	toolCalls := r.message.Choices[0].Message.ToolCalls
	var sb strings.Builder
	_, _ = sb.WriteString("\n\n")
	for _, call := range toolCalls {
		content, err := toolCall(call.Function.Name, []byte(call.Function.JSON.Arguments.Raw()))
		r.request.Messages = append(r.request.Messages, openai.ToolMessage(content, call.ID))
		_, _ = sb.WriteString("> Called tool: `" + call.Function.Name + "`")
		if err != nil {
			_, _ = sb.WriteString(" (failed: `" + err.Error() + "`)")
		}
		_, _ = sb.WriteString("\n")
	}
	_, _ = sb.WriteString("\n")

	if len(toolCalls) == 0 {
		return oldopenai.ChatCompletionStreamResponse{}, io.EOF
	}

	r.stream = nil

	return oldopenai.ChatCompletionStreamResponse{
		Choices: []oldopenai.ChatCompletionStreamChoice{
			{
				Index: 0,
				Delta: oldopenai.ChatCompletionStreamChoiceDelta{
					Content: sb.String(),
					Role:    oldopenai.ChatMessageRoleTool,
				},
			},
		},
	}, nil
}

func makeOpenAIMCPTools(mcps map[string][]mcp.Tool) []openai.ChatCompletionToolParam {
	var tools []openai.ChatCompletionToolParam
	for name, serverTools := range mcps {
		for _, tool := range serverTools {
			var params map[string]any
			json.Unmarshal(tool.RawInputSchema, &params)
			tools = append(tools, openai.ChatCompletionToolParam{
				Type: constant.Function("function"),
				Function: openai.FunctionDefinitionParam{
					Name:        fmt.Sprintf("%s_%s", name, tool.Name),
					Description: openai.String(tool.Description),
					Parameters:  params,
				},
			})
		}
	}
	return tools
}
