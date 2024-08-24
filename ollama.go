package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	openai "github.com/sashabaranov/go-openai"
)

// OllamaClientConfig represents the configuration for the Ollama API client.
type OllamaClientConfig struct {
	BaseURL            string
	HTTPClient         *http.Client
	EmptyMessagesLimit uint
}

// DefaultOllamaConfig returns the default configuration for the Ollama API client.
func DefaultOllamaConfig() OllamaClientConfig {
	return OllamaClientConfig{
		BaseURL:            "http://localhost:11434/api",
		HTTPClient:         &http.Client{},
		EmptyMessagesLimit: defaultEmptyMessagesLimit,
	}
}

// OllamaMessageCompletionRequestOptions represents the valid parameters and values options for the request.
type OllamaMessageCompletionRequestOptions struct {
	Mirostat      int     `json:"mirostat,omitempty"`
	MirostatEta   int     `json:"mirostat_eta,omitempty"`
	MirostatTau   int     `json:"mirostat_tau,omitempty"`
	NumCtx        int     `json:"num_ctx,omitempty"`
	RepeatLastN   int     `json:"repeat_last_n,omitempty"`
	RepeatPenalty float32 `json:"repeat_penalty,omitempty"`
	Temperature   float32 `json:"temperature,omitempty"`
	Seed          int     `json:"seed,omitempty"`
	Stop          string  `json:"stop,omitempty"`
	TfsZ          float32 `json:"tfs_z,omitempty"`
	NumPredict    int     `json:"num_predict,omitempty"`
	TopP          float32 `json:"top_p,omitempty"`
	TopK          int     `json:"top_k,omitempty"`
}

// OllamaMessageCompletionRequest represents the request body for the generate completion API.
type OllamaMessageCompletionRequest struct {
	Model     string                                `json:"model"`
	Messages  []openai.ChatCompletionMessage        `json:"messages"`
	Options   OllamaMessageCompletionRequestOptions `json:"options,omitempty"`
	Stream    bool                                  `json:"stream,omitempty"`
	KeepAlive string                                `json:"keep_alive,omitempty"`
}

// OllamaRequestBuilder is an interface for building HTTP requests for the Ollama API.
type OllamaRequestBuilder interface {
	Build(ctx context.Context, method, url string, body any, header http.Header) (*http.Request, error)
}

// NewOllamaRequestBuilder creates a new HTTPRequestBuilder.
func NewOllamaRequestBuilder() *HTTPRequestBuilder {
	return &HTTPRequestBuilder{
		marshaller: &JSONMarshaller{},
	}
}

// OllamaClient is a client for the Ollama API.
type OllamaClient struct {
	config OllamaClientConfig

	requestBuilder OllamaRequestBuilder
}

// NewOllamaClient creates a new OllamaClient with the given configuration.
func NewOllamaClientWithConfig(config OllamaClientConfig) *OllamaClient {
	return &OllamaClient{
		config:         config,
		requestBuilder: NewOllamaRequestBuilder(),
	}
}

const ollamaChatCompletionsSuffix = "/chat"

func (c *OllamaClient) newRequest(ctx context.Context, method, url string, setters ...requestOption) (*http.Request, error) {
	// Default Options
	args := &requestOptions{
		body:   nil,
		header: make(http.Header),
	}
	for _, setter := range setters {
		setter(args)
	}
	req, err := c.requestBuilder.Build(ctx, method, url, args.body, args.header)
	if err != nil {
		return nil, fmt.Errorf("OllamaClient.newRequest: %w", err)
	}
	return req, nil
}

func (c *OllamaClient) handleErrorResp(resp *http.Response) error {
	// Print the response text
	var errRes openai.ErrorResponse
	err := json.NewDecoder(resp.Body).Decode(&errRes)
	if err != nil || errRes.Error == nil {
		reqErr := &openai.RequestError{
			HTTPStatusCode: resp.StatusCode,
			Err:            err,
		}
		if errRes.Error != nil {
			reqErr.Err = errRes.Error
		}
		return reqErr
	}

	errRes.Error.HTTPStatusCode = resp.StatusCode
	return errRes.Error
}

// OllamaMessageUsage represents the usage of an Ollama message.
type OllamaMessageUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

// OllamaMessage represents an Ollama message.
type OllamaMessage struct {
	Usage        *OllamaMessageUsage `json:"usage,omitempty"`
	StopReason   *string             `json:"stop_reason,omitempty"`
	StopSequence *string             `json:"stop_sequence,omitempty"`
	ID           string              `json:"id,omitempty"`
	Type         string              `json:"type"`
	Role         string              `json:"role,omitempty"`
	Model        string              `json:"model,omitempty"`
	Content      []string            `json:"content,omitempty"`
}

// OllamaMessageContentBlock represents a content block in an Ollama message.
type OllamaMessageContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// OllamaMessageTextDelta represents a text delta in an Ollama message.
type OllamaMessageTextDelta struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// OllamaMessageCompletionRequest represents the response body for the generate completion API.
type OllamaCompletionMessageResponse struct {
	Model              string                       `json:"model"`
	CreatedAt          string                       `json:"created_at"`
	Message            openai.ChatCompletionMessage `json:"message"`
	Done               bool                         `json:"done"`
	TotalDuration      int                          `json:"total_duration"`
	LoadDuration       int                          `json:"load_duration"`
	PromptEvalCount    int                          `json:"prompt_eval_count"`
	PromptEvalDuration int                          `json:"prompt_eval_duration"`
	EvalCount          int                          `json:"eval_count"`
	EvalDuration       int                          `json:"eval_duration"`
}

// ChatCompletionStream represents a stream for chat completion.
type OllamaChatCompletionStream struct {
	*ollamaStreamReader
}

type ollamaStreamReader struct {
	emptyMessagesLimit uint
	isFinished         bool

	reader         *bufio.Reader
	response       *http.Response
	errAccumulator ErrorAccumulator
	unmarshaler    Unmarshaler

	httpHeader
}

// Recv reads the next response from the stream.
func (stream *ollamaStreamReader) Recv() (response openai.ChatCompletionStreamResponse, err error) {
	if stream.isFinished {
		err = io.EOF
		return
	}

	response, err = stream.processLines()
	return
}

// Close closes the stream.
func (stream *ollamaStreamReader) Close() error {
	return stream.response.Body.Close() //nolint:wrapcheck
}

//nolint:gocognit
func (stream *ollamaStreamReader) processLines() (openai.ChatCompletionStreamResponse, error) {
	for {
		rawLine, readErr := stream.reader.ReadBytes('\n')

		if readErr != nil {
			return *new(openai.ChatCompletionStreamResponse), fmt.Errorf("ollamaStreamReader.processLines: %w", readErr)
		}

		noSpaceLine := bytes.TrimSpace(rawLine)

		var chunk OllamaCompletionMessageResponse
		unmarshalErr := stream.unmarshaler.Unmarshal(noSpaceLine, &chunk)
		if unmarshalErr != nil {
			return openai.ChatCompletionStreamResponse{}, fmt.Errorf("ollamaStreamReader.processLines: %w", unmarshalErr)
		}

		if chunk.Done {
			return openai.ChatCompletionStreamResponse{}, nil
		}

		if chunk.Message.Content == "" {
			continue
		}

		// NOTE: Leverage the existing logic based on OpenAI ChatCompletionStreamResponse by
		//       converting the Ollama events into them.
		response := openai.ChatCompletionStreamResponse{
			Choices: []openai.ChatCompletionStreamChoice{
				{
					Index: 0,
					Delta: openai.ChatCompletionStreamChoiceDelta{
						Content: chunk.Message.Content,
						Role:    "assistant",
					},
				},
			},
		}

		return response, nil
	}
}

func ollamaSendRequestStream(client *OllamaClient, req *http.Request) (*ollamaStreamReader, error) {
	req.Header.Set("content-type", "application/json")

	resp, err := client.config.HTTPClient.Do(req) //nolint:bodyclose // body is closed in stream.Close()
	if err != nil {
		return new(ollamaStreamReader), err
	}
	if isFailureStatusCode(resp) {
		return new(ollamaStreamReader), client.handleErrorResp(resp)
	}

	return &ollamaStreamReader{
		emptyMessagesLimit: client.config.EmptyMessagesLimit,
		reader:             bufio.NewReader(resp.Body),
		response:           resp,
		errAccumulator:     NewErrorAccumulator(),
		unmarshaler:        &JSONUnmarshaler{},
		httpHeader:         httpHeader(resp.Header),
	}, nil
}

// CreateChatCompletionStream — API call to create a generate completion w/ streaming
// support. It sets whether to stream back partial progress. If set, tokens will be
// sent as data-only server-sent events as they become available, with the
// stream terminated by a data: [DONE] message.
func (c *OllamaClient) CreateChatCompletionStream(
	ctx context.Context,
	request OllamaMessageCompletionRequest,
) (stream *OllamaChatCompletionStream, err error) {
	urlSuffix := ollamaChatCompletionsSuffix

	request.Stream = true
	req, err := c.newRequest(ctx, http.MethodPost, c.config.BaseURL+urlSuffix, withBody(request))
	if err != nil {
		return nil, err
	}

	resp, err := ollamaSendRequestStream(c, req)
	if err != nil {
		return
	}

	stream = &OllamaChatCompletionStream{
		ollamaStreamReader: resp,
	}
	return
}
