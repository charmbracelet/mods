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

var googleHeaderData = []byte("data: ")

// GoogleClientConfig represents the configuration for the Google API client.
type GoogleClientConfig struct {
	BaseURL            string
	HTTPClient         *http.Client
	EmptyMessagesLimit uint
}

// DefaultGoogleConfig returns the default configuration for the Google API client.
func DefaultGoogleConfig(model, authToken string) GoogleClientConfig {
	return GoogleClientConfig{
		BaseURL:            fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", model, authToken),
		HTTPClient:         &http.Client{},
		EmptyMessagesLimit: defaultEmptyMessagesLimit,
	}
}

// GoogleParts is a datatype containing media that is part of a multi-part Content message.
type GoogleParts struct {
	Text string `json:"text,omitempty"`
}

// GoogleContent is the base structured datatype containing multi-part content of a message.
type GoogleContent struct {
	Parts []GoogleParts `json:"parts,omitempty"`
	Role  string        `json:"role,omitempty"`
}

// GoogleGenerationConfig are the options for model generation and outputs. Not all parameters are configurable for every model.
type GoogleGenerationConfig struct {
	StopSequences    []string `json:"stopSequences,omitempty"`
	ResponseMimeType string   `json:"responseMimeType,omitempty"`
	CandidateCount   uint     `json:"candidateCount,omitempty"`
	MaxOutputTokens  uint     `json:"maxOutputTokens,omitempty"`
	Temperature      float32  `json:"temperature,omitempty"`
	TopP             float32  `json:"topP,omitempty"`
	TopK             int      `json:"topK,omitempty"`
}

// GoogleMessageCompletionRequestOptions represents the valid parameters and value options for the request.
type GoogleMessageCompletionRequest struct {
	Contents         []GoogleContent        `json:"contents,omitempty"`
	GenerationConfig GoogleGenerationConfig `json:"generationConfig,omitempty"`
}

// GoogleRequestBuilder is an interface for building HTTP requests for the Google API.
type GoogleRequestBuilder interface {
	Build(ctx context.Context, method, url string, body any, header http.Header) (*http.Request, error)
}

// NewGoogleRequestBuilder creates a new HTTPRequestBuilder.
func NewGoogleRequestBuilder() *HTTPRequestBuilder {
	return &HTTPRequestBuilder{
		marshaller: &JSONMarshaller{},
	}
}

// GoogleClient is a client for the Anthropic API.
type GoogleClient struct {
	config GoogleClientConfig

	requestBuilder GoogleRequestBuilder
}

// NewGoogleClient creates a new AnthropicClient with the given configuration.
func NewGoogleClientWithConfig(config GoogleClientConfig) *GoogleClient {
	return &GoogleClient{
		config:         config,
		requestBuilder: NewGoogleRequestBuilder(),
	}
}

func (c *GoogleClient) newRequest(ctx context.Context, method, url string, setters ...requestOption) (*http.Request, error) {
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
	return req, nil
}

func (c *GoogleClient) handleErrorResp(resp *http.Response) error {
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

// GoogleCandidates represents a response candidate generated from the model.
type GoogleCandidate struct {
	Content      GoogleContent `json:"content,omitempty"`
	FinishReason string        `json:"finishReason,omitempty"`
	TokenCount   uint          `json:"tokenCount,omitempty"`
	Index        uint          `json:"index,omitempty"`
}

// GoogleCompletionMessageResponse represents a response to an Google completion message.
type GoogleCompletionMessageResponse struct {
	Candidates []GoogleCandidate `json:"candidates,omitempty"`
}

// GoogleChatCompletionStream represents a stream for chat completion.
type GoogleChatCompletionStream struct {
	*googleStreamReader
}

type googleStreamReader struct {
	emptyMessagesLimit uint
	isFinished         bool

	reader         *bufio.Reader
	response       *http.Response
	errAccumulator ErrorAccumulator
	unmarshaler    Unmarshaler

	httpHeader
}

// Recv reads the next response from the stream.
func (stream *googleStreamReader) Recv() (response openai.ChatCompletionStreamResponse, err error) {
	if stream.isFinished {
		err = io.EOF
		return
	}

	response, err = stream.processLines()
	return
}

// Close closes the stream.
func (stream *googleStreamReader) Close() error {
	return stream.response.Body.Close() //nolint:wrapcheck
}

//nolint:gocognit
func (stream *googleStreamReader) processLines() (openai.ChatCompletionStreamResponse, error) {
	var (
		emptyMessagesCount uint
		hasError           bool
	)

	for {
		rawLine, readErr := stream.reader.ReadBytes('\n')

		if readErr != nil {
			return *new(openai.ChatCompletionStreamResponse), fmt.Errorf("googleStreamReader.processLines: %w", readErr)
		}

		noSpaceLine := bytes.TrimSpace(rawLine)

		if bytes.HasPrefix(noSpaceLine, errorPrefix) {
			hasError = true
			// NOTE: Continue to the next event to get the error data.
			continue
		}

		if !bytes.HasPrefix(noSpaceLine, googleHeaderData) || hasError {
			if hasError {
				noSpaceLine = bytes.TrimPrefix(noSpaceLine, googleHeaderData)
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

		noPrefixLine := bytes.TrimPrefix(noSpaceLine, googleHeaderData)

		var chunk GoogleCompletionMessageResponse
		unmarshalErr := stream.unmarshaler.Unmarshal(noPrefixLine, &chunk)
		if unmarshalErr != nil {
			return *new(openai.ChatCompletionStreamResponse), fmt.Errorf("googleStreamReader.processLines: %w", unmarshalErr)
		}

		// NOTE: Leverage the existing logic based on OpenAI ChatCompletionStreamResponse by
		//       converting the Anthropic events into them.
		if len(chunk.Candidates) == 0 {
			continue
		}
		parts := chunk.Candidates[0].Content.Parts
		if len(parts) == 0 {
			continue
		}
		response := openai.ChatCompletionStreamResponse{
			Choices: []openai.ChatCompletionStreamChoice{
				{
					Index: 0,
					Delta: openai.ChatCompletionStreamChoiceDelta{
						Content: chunk.Candidates[0].Content.Parts[0].Text,
						Role:    "assistant",
					},
				},
			},
		}

		return response, nil
	}
}

func googleSendRequestStream(client *GoogleClient, req *http.Request) (*googleStreamReader, error) {
	req.Header.Set("content-type", "application/json")

	resp, err := client.config.HTTPClient.Do(req) //nolint:bodyclose // body is closed in stream.Close()
	if err != nil {
		return new(googleStreamReader), err
	}
	if isFailureStatusCode(resp) {
		return new(googleStreamReader), client.handleErrorResp(resp)
	}
	return &googleStreamReader{
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
func (c *GoogleClient) CreateChatCompletionStream(
	ctx context.Context,
	request GoogleMessageCompletionRequest,
) (stream *GoogleChatCompletionStream, err error) {
	req, err := c.newRequest(ctx, http.MethodPost, c.config.BaseURL, withBody(request))
	if err != nil {
		return nil, err
	}

	resp, err := googleSendRequestStream(c, req)
	if err != nil {
		return
	}
	stream = &GoogleChatCompletionStream{
		googleStreamReader: resp,
	}
	return
}
