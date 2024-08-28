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

type (
	// AnthropicAPIVersion represents the version of the Anthropic API.
	AnthropicAPIVersion string
	// AnthropicAPIBeta represents the beta version of the Anthropic API.
	AnthropicAPIBeta string
)

const (
	// AnthropicV20230601 represents the version of the Anthropic API.
	AnthropicV20230601 AnthropicAPIVersion = "2023-06-01"
	// AnthropicBeta represents the beta version of the Anthropic API.
	AnthropicBeta AnthropicAPIBeta = "messages-2023-12-15"
)

var (
	headerData  = []byte("data: ")
	errorPrefix = []byte(`event: error`)
)

// AnthropicClientConfig represents the configuration for the Anthropic API client.
type AnthropicClientConfig struct {
	AuthToken          string
	BaseURL            string
	HTTPClient         *http.Client
	Version            AnthropicAPIVersion
	Beta               AnthropicAPIBeta
	EmptyMessagesLimit uint
}

// DefaultAnthropicConfig returns the default configuration for the Anthropic API client.
func DefaultAnthropicConfig(authToken string) AnthropicClientConfig {
	return AnthropicClientConfig{
		AuthToken:          authToken,
		BaseURL:            "https://api.anthropic.com/v1",
		Version:            AnthropicV20230601,
		Beta:               AnthropicBeta,
		HTTPClient:         &http.Client{},
		EmptyMessagesLimit: defaultEmptyMessagesLimit,
	}
}

// AnthropicMessageCompletionRequest represents the request body for the chat completion API.
type AnthropicMessageCompletionRequest struct {
	Model         string                         `json:"model"`
	System        string                         `json:"system"`
	Messages      []openai.ChatCompletionMessage `json:"messages"`
	MaxTokens     int                            `json:"max_tokens"`
	Temperature   float32                        `json:"temperature,omitempty"`
	TopP          float32                        `json:"top_p,omitempty"`
	TopK          int                            `json:"top_k,omitempty"`
	Stream        bool                           `json:"stream,omitempty"`
	StopSequences []string                       `json:"stop_sequences,omitempty"`
}

// AnthropicRequestBuilder is an interface for building HTTP requests for the Anthropic API.
type AnthropicRequestBuilder interface {
	Build(ctx context.Context, method, url string, body any, header http.Header) (*http.Request, error)
}

// NewAnthropicRequestBuilder creates a new HTTPRequestBuilder.
func NewAnthropicRequestBuilder() *HTTPRequestBuilder {
	return &HTTPRequestBuilder{
		marshaller: &JSONMarshaller{},
	}
}

// AnthropicClient is a client for the Anthropic API.
type AnthropicClient struct {
	config AnthropicClientConfig

	requestBuilder AnthropicRequestBuilder
}

// NewAnthropicClient creates a new AnthropicClient with the given configuration.
func NewAnthropicClientWithConfig(config AnthropicClientConfig) *AnthropicClient {
	return &AnthropicClient{
		config:         config,
		requestBuilder: NewAnthropicRequestBuilder(),
	}
}

const anthropicChatCompletionsSuffix = "/messages"

func (c *AnthropicClient) setCommonHeaders(req *http.Request) {
	req.Header.Set("anthropic-version", string(c.config.Version))
	req.Header.Set("x-api-key", c.config.AuthToken)
}

func (c *AnthropicClient) newRequest(ctx context.Context, method, url string, setters ...requestOption) (*http.Request, error) {
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
		return new(http.Request), err
	}
	c.setCommonHeaders(req)
	return req, nil
}

func (c *AnthropicClient) handleErrorResp(resp *http.Response) error {
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

// AnthropicMessageUsage represents the usage of an Anthropic message.
type AnthropicMessageUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

// AnthropicMessage represents an Anthropic message.
type AnthropicMessage struct {
	Usage        *AnthropicMessageUsage `json:"usage,omitempty"`
	StopReason   *string                `json:"stop_reason,omitempty"`
	StopSequence *string                `json:"stop_sequence,omitempty"`
	ID           string                 `json:"id,omitempty"`
	Type         string                 `json:"type"`
	Role         string                 `json:"role,omitempty"`
	Model        string                 `json:"model,omitempty"`
	Content      []string               `json:"content,omitempty"`
}

// AnthropicMessageContentBlock represents a content block in an Anthropic message.
type AnthropicMessageContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// AnthropicMessageTextDelta represents a text delta in an Anthropic message.
type AnthropicMessageTextDelta struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// AnthropicCompletionMessageResponse represents a response to an Anthropic completion message.
type AnthropicCompletionMessageResponse struct {
	Type         string                        `json:"type"`
	Message      *AnthropicMessage             `json:"message,omitempty"`
	Index        int                           `json:"index,omitempty"`
	ContentBlock *AnthropicMessageContentBlock `json:"content_block,omitempty"`
	Delta        *AnthropicMessageTextDelta    `json:"delta,omitempty"`
}

// AnthropicChatCompletionStream represents a stream for chat completion.
type AnthropicChatCompletionStream struct {
	*anthropicStreamReader
}

type anthropicStreamReader struct {
	emptyMessagesLimit uint
	isFinished         bool

	reader         *bufio.Reader
	response       *http.Response
	errAccumulator ErrorAccumulator
	unmarshaler    Unmarshaler

	httpHeader
}

// Recv reads the next response from the stream.
func (stream *anthropicStreamReader) Recv() (response openai.ChatCompletionStreamResponse, err error) {
	if stream.isFinished {
		err = io.EOF
		return
	}

	response, err = stream.processLines()
	return
}

// Close closes the stream.
func (stream *anthropicStreamReader) Close() error {
	return stream.response.Body.Close() //nolint:wrapcheck
}

//nolint:gocognit
func (stream *anthropicStreamReader) processLines() (openai.ChatCompletionStreamResponse, error) {
	var (
		emptyMessagesCount uint
		hasError           bool
	)

	for {
		rawLine, readErr := stream.reader.ReadBytes('\n')

		if readErr != nil {
			return *new(openai.ChatCompletionStreamResponse), fmt.Errorf("anthropicStreamReader.processLines: %w", readErr)
		}

		noSpaceLine := bytes.TrimSpace(rawLine)

		if bytes.HasPrefix(noSpaceLine, errorPrefix) {
			hasError = true
			// NOTE: Continue to the next event to get the error data.
			continue
		}

		if !bytes.HasPrefix(noSpaceLine, headerData) || hasError {
			if hasError {
				noSpaceLine = bytes.TrimPrefix(noSpaceLine, headerData)
			}
			writeErr := stream.errAccumulator.Write(noSpaceLine)
			if writeErr != nil {
				return *new(openai.ChatCompletionStreamResponse), fmt.Errorf("ollamaStreamReader.processLines: %w", writeErr)
			}
			emptyMessagesCount++
			if emptyMessagesCount > stream.emptyMessagesLimit {
				return *new(openai.ChatCompletionStreamResponse), ErrTooManyEmptyStreamMessages
			}
			continue
		}

		noPrefixLine := bytes.TrimPrefix(noSpaceLine, headerData)
		if string(noPrefixLine) == "event: message_stop" {
			stream.isFinished = true
			return *new(openai.ChatCompletionStreamResponse), io.EOF
		}

		var chunk AnthropicCompletionMessageResponse
		unmarshalErr := stream.unmarshaler.Unmarshal(noPrefixLine, &chunk)
		if unmarshalErr != nil {
			return *new(openai.ChatCompletionStreamResponse), fmt.Errorf("anthropicStreamReader.processLines: %w", unmarshalErr)
		}

		if chunk.Type != "content_block_delta" {
			continue
		}

		// NOTE: Leverage the existing logic based on OpenAI ChatCompletionStreamResponse by
		//       converting the Anthropic events into them.
		response := openai.ChatCompletionStreamResponse{
			Choices: []openai.ChatCompletionStreamChoice{
				{
					Index: 0,
					Delta: openai.ChatCompletionStreamChoiceDelta{
						Content: chunk.Delta.Text,
						Role:    "assistant",
					},
				},
			},
		}

		return response, nil
	}
}

func anthropicSendRequestStream(client *AnthropicClient, req *http.Request) (*anthropicStreamReader, error) {
	req.Header.Set("content-type", "application/json")
	req.Header.Set("anthropic-beta", string(client.config.Beta))

	resp, err := client.config.HTTPClient.Do(req) //nolint:bodyclose // body is closed in stream.Close()
	if err != nil {
		return new(anthropicStreamReader), err
	}
	if isFailureStatusCode(resp) {
		return new(anthropicStreamReader), client.handleErrorResp(resp)
	}
	return &anthropicStreamReader{
		emptyMessagesLimit: client.config.EmptyMessagesLimit,
		reader:             bufio.NewReader(resp.Body),
		response:           resp,
		errAccumulator:     NewErrorAccumulator(),
		unmarshaler:        &JSONUnmarshaler{},
		httpHeader:         httpHeader(resp.Header),
	}, nil
}

// CreateChatCompletionStream — API call to create a chat completion w/ streaming
// support. It sets whether to stream back partial progress. If set, tokens will be
// sent as data-only server-sent events as they become available, with the
// stream terminated by a data: [DONE] message.
func (c *AnthropicClient) CreateChatCompletionStream(
	ctx context.Context,
	request AnthropicMessageCompletionRequest,
) (stream *AnthropicChatCompletionStream, err error) {
	urlSuffix := anthropicChatCompletionsSuffix

	request.Stream = true
	req, err := c.newRequest(ctx, http.MethodPost, c.config.BaseURL+urlSuffix, withBody(request))
	if err != nil {
		return nil, err
	}

	resp, err := anthropicSendRequestStream(c, req)
	if err != nil {
		return
	}
	stream = &AnthropicChatCompletionStream{
		anthropicStreamReader: resp,
	}
	return
}
