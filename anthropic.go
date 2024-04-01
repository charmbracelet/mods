package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	openai "github.com/sashabaranov/go-openai"
)

type (
	AnthropicApiVersion string
	AnthropicApiBeta    string
)

type httpHeader http.Header

const (
	AnthropicV20230601        AnthropicApiVersion = "2023-06-01"
	AnthropicBeta             AnthropicApiBeta    = "messages-2023-12-15"
	defaultEmptyMessagesLimit uint                = 300
)

var (
	eventPrefix                   = []byte("event: ")
	headerData                    = []byte("data: ")
	errorPrefix                   = []byte(`event: error`)
	ErrTooManyEmptyStreamMessages = errors.New("stream has sent too many empty messages")
)

type AnthropicClientConfig struct {
	AuthToken          string
	BaseURL            string
	HTTPClient         *http.Client
	Version            AnthropicApiVersion
	Beta               AnthropicApiBeta
	EmptyMessagesLimit uint
}

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

type AnthropicMessageCompletionRequest struct {
	Model         string                         `json:"model"`
	System        string                         `json:"system"`
	Messages      []openai.ChatCompletionMessage `json:"messages"`
	MaxTokens     int                            `json:"max_tokens"`
	Temperature   float32                        `json:"temperature,omitempty"`
	TopP          float32                        `json:"top_p,omitempty"`
	Stream        bool                           `json:"stream,omitempty"`
	StopSequences []string                       `json:"stop_sequences,omitempty"`
}

type Marshaller interface {
	Marshal(value any) ([]byte, error)
}

type JSONMarshaller struct{}

func (jm *JSONMarshaller) Marshal(value any) ([]byte, error) {
	return json.Marshal(value)
}

type AnthropicRequestBuilder interface {
	Build(ctx context.Context, method, url string, body any, header http.Header) (*http.Request, error)
}

type HTTPRequestBuilder struct {
	marshaller Marshaller
}

func (b *HTTPRequestBuilder) Build(
	ctx context.Context,
	method string,
	url string,
	body any,
	header http.Header,
) (req *http.Request, err error) {
	var bodyReader io.Reader
	if body != nil {
		if v, ok := body.(io.Reader); ok {
			bodyReader = v
		} else {
			var reqBytes []byte
			reqBytes, err = b.marshaller.Marshal(body)
			if err != nil {
				return
			}
			bodyReader = bytes.NewBuffer(reqBytes)
		}
	}
	req, err = http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return
	}
	if header != nil {
		req.Header = header
	}
	return
}

func NewAnthropicRequestBuilder() *HTTPRequestBuilder {
	return &HTTPRequestBuilder{
		marshaller: &JSONMarshaller{},
	}
}

type AnthropicClient struct {
	config AnthropicClientConfig

	requestBuilder AnthropicRequestBuilder
}

func NewAnthropicClientWithConfig(config AnthropicClientConfig) *AnthropicClient {
	return &AnthropicClient{
		config:         config,
		requestBuilder: NewAnthropicRequestBuilder(),
	}
}

type requestOptions struct {
	body   any
	header http.Header
}

type requestOption func(*requestOptions)

const chatCompletionsSuffix = "/messages"

func (c *AnthropicClient) setCommonHeaders(req *http.Request) {
	req.Header.Set("anthropic-version", string(c.config.Version))
	req.Header.Set("x-api-key", c.config.AuthToken)
}

func withBody(body any) requestOption {
	return func(args *requestOptions) {
		args.body = body
	}
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
		return nil, err
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

type streamable interface {
	openai.ChatCompletionStreamResponse | openai.CompletionResponse
}

type ErrorAccumulator interface {
	Write(p []byte) error
	Bytes() []byte
}

type Unmarshaler interface {
	Unmarshal(data []byte, v any) error
}

type streamReader struct {
	emptyMessagesLimit uint
	isFinished         bool

	reader         *bufio.Reader
	response       *http.Response
	errAccumulator ErrorAccumulator
	unmarshaler    Unmarshaler

	httpHeader
}

func (stream *streamReader) Recv() (response openai.ChatCompletionStreamResponse, err error) {
	if stream.isFinished {
		err = io.EOF
		return
	}

	response, err = stream.processLines()
	return
}

type AnthropicMessageUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

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

type AnthropicMessageContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type AnthropicMessageTextDelta struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type AnthropicCompletionMessageResponse struct {
	Type         string                        `json:"type"`
	Message      *AnthropicMessage             `json:"message,omitempty"`
	Index        int                           `json:"index,omitempty"`
	ContentBlock *AnthropicMessageContentBlock `json:"content_block,omitempty"`
	Delta        *AnthropicMessageTextDelta    `json:"delta,omitempty"`
}

//nolint:gocognit
func (stream *streamReader) processLines() (openai.ChatCompletionStreamResponse, error) {
	var (
		emptyMessagesCount uint
		hasError           bool
	)

	for {
		rawLine, readErr := stream.reader.ReadBytes('\n')

		if readErr != nil {
			return *new(openai.ChatCompletionStreamResponse), readErr
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
				return *new(openai.ChatCompletionStreamResponse), writeErr
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
			return *new(openai.ChatCompletionStreamResponse), unmarshalErr
		}

		if chunk.Type != "content_block_delta" {
			continue
		}

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

func (stream *streamReader) unmarshalError() (errResp *openai.ErrorResponse) {
	errBytes := stream.errAccumulator.Bytes()
	if len(errBytes) == 0 {
		return
	}

	err := stream.unmarshaler.Unmarshal(errBytes, &errResp)
	if err != nil {
		errResp = nil
	}

	return
}

func (stream *streamReader) Close() {
	stream.response.Body.Close()
}

type errorBuffer interface {
	io.Writer
	Len() int
	Bytes() []byte
}

type DefaultErrorAccumulator struct {
	Buffer errorBuffer
}

func NewErrorAccumulator() ErrorAccumulator {
	return &DefaultErrorAccumulator{
		Buffer: &bytes.Buffer{},
	}
}

func (e *DefaultErrorAccumulator) Write(p []byte) error {
	_, err := e.Buffer.Write(p)
	if err != nil {
		return fmt.Errorf("error accumulator write error, %w", err)
	}
	return nil
}

func (e *DefaultErrorAccumulator) Bytes() (errBytes []byte) {
	if e.Buffer.Len() == 0 {
		return
	}
	errBytes = e.Buffer.Bytes()
	return
}

func isFailureStatusCode(resp *http.Response) bool {
	return resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest
}

type JSONUnmarshaler struct{}

func (jm *JSONUnmarshaler) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func sendRequestStream(client *AnthropicClient, req *http.Request) (*streamReader, error) {
	req.Header.Set("content-type", "application/json")
	req.Header.Set("anthropic-beta", string(client.config.Beta))

	resp, err := client.config.HTTPClient.Do(req) //nolint:bodyclose // body is closed in stream.Close()
	if err != nil {
		return new(streamReader), err
	}
	if isFailureStatusCode(resp) {
		return new(streamReader), client.handleErrorResp(resp)
	}
	return &streamReader{
		emptyMessagesLimit: client.config.EmptyMessagesLimit,
		reader:             bufio.NewReader(resp.Body),
		response:           resp,
		errAccumulator:     NewErrorAccumulator(),
		unmarshaler:        &JSONUnmarshaler{},
		httpHeader:         httpHeader(resp.Header),
	}, nil
}

// ChatCompletionStream
// Note: Perhaps it is more elegant to abstract Stream using generics.
type ChatCompletionStream struct {
	*streamReader
}

// CreateChatCompletionStream — API call to create a chat completion w/ streaming
// support. It sets whether to stream back partial progress. If set, tokens will be
// sent as data-only server-sent events as they become available, with the
// stream terminated by a data: [DONE] message.
func (c *AnthropicClient) CreateChatCompletionStream(
	ctx context.Context,
	request AnthropicMessageCompletionRequest,
) (stream *ChatCompletionStream, err error) {
	urlSuffix := chatCompletionsSuffix

	request.Stream = true
	req, err := c.newRequest(ctx, http.MethodPost, c.config.BaseURL+urlSuffix, withBody(request))
	if err != nil {
		return nil, err
	}

	resp, err := sendRequestStream(c, req)
	if err != nil {
		return
	}
	stream = &ChatCompletionStream{
		streamReader: resp,
	}
	return
}
