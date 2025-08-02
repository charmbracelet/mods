// Package google implements [stream.Stream] for Google.
package google

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/charmbracelet/mods/internal/proto"
	"github.com/charmbracelet/mods/internal/stream"
	"github.com/openai/openai-go"
)

var _ stream.Client = &Client{}

const emptyMessagesLimit uint = 300

var (
	googleHeaderData = []byte("data: ")
	errorPrefix      = []byte(`event: error`)
)

// Config represents the configuration for the Google API client.
type Config struct {
	BaseURL        string
	HTTPClient     *http.Client
	ThinkingBudget int
}

// DefaultConfig returns the default configuration for the Google API client.
func DefaultConfig(model, authToken string) Config {
	return Config{
		BaseURL:    fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", model, authToken),
		HTTPClient: &http.Client{},
	}
}

// Part is a datatype containing media that is part of a multi-part Content message.
type Part struct {
	Text string `json:"text,omitempty"`
}

// Content is the base structured datatype containing multi-part content of a message.
type Content struct {
	Parts []Part `json:"parts,omitempty"`
	Role  string `json:"role,omitempty"`
}

// ThinkingConfig - for more details see https://ai.google.dev/gemini-api/docs/thinking#rest .
type ThinkingConfig struct {
	ThinkingBudget int `json:"thinkingBudget,omitempty"`
}

// GenerationConfig are the options for model generation and outputs. Not all parameters are configurable for every model.
type GenerationConfig struct {
	StopSequences    []string        `json:"stopSequences,omitempty"`
	ResponseMimeType string          `json:"responseMimeType,omitempty"`
	CandidateCount   uint            `json:"candidateCount,omitempty"`
	MaxOutputTokens  uint            `json:"maxOutputTokens,omitempty"`
	Temperature      float64         `json:"temperature,omitempty"`
	TopP             float64         `json:"topP,omitempty"`
	TopK             int64           `json:"topK,omitempty"`
	ThinkingConfig   *ThinkingConfig `json:"thinkingConfig,omitempty"`
}

// MessageCompletionRequest represents the valid parameters and value options for the request.
type MessageCompletionRequest struct {
	Contents         []Content        `json:"contents,omitempty"`
	GenerationConfig GenerationConfig `json:"generationConfig,omitempty"`
}

// RequestBuilder is an interface for building HTTP requests for the Google API.
type RequestBuilder interface {
	Build(ctx context.Context, method, url string, body any, header http.Header) (*http.Request, error)
}

// NewRequestBuilder creates a new HTTPRequestBuilder.
func NewRequestBuilder() *HTTPRequestBuilder {
	return &HTTPRequestBuilder{
		marshaller: &JSONMarshaller{},
	}
}

// Client is a client for the Google API.
type Client struct {
	config Config

	requestBuilder RequestBuilder
}

// Request implements stream.Client.
func (c *Client) Request(ctx context.Context, request proto.Request) stream.Stream {
	stream := new(Stream)
	body := MessageCompletionRequest{
		Contents: fromProtoMessages(request.Messages),
		GenerationConfig: GenerationConfig{
			ResponseMimeType: "",
			CandidateCount:   1,
			StopSequences:    request.Stop,
			MaxOutputTokens:  4096,
		},
	}

	if request.Temperature != nil {
		body.GenerationConfig.Temperature = *request.Temperature
	}
	if request.TopP != nil {
		body.GenerationConfig.TopP = *request.TopP
	}
	if request.TopK != nil {
		body.GenerationConfig.TopK = *request.TopK
	}

	if request.MaxTokens != nil {
		body.GenerationConfig.MaxOutputTokens = uint(*request.MaxTokens) //nolint:gosec
	}

	if c.config.ThinkingBudget != 0 {
		body.GenerationConfig.ThinkingConfig = &ThinkingConfig{
			ThinkingBudget: c.config.ThinkingBudget,
		}
	}

	req, err := c.newRequest(ctx, http.MethodPost, c.config.BaseURL, withBody(body))
	if err != nil {
		stream.err = err
		return stream
	}

	stream, err = googleSendRequestStream(c, req)
	if err != nil {
		stream.err = err
	}
	return stream
}

// New creates a new Client with the given configuration.
func New(config Config) *Client {
	return &Client{
		config:         config,
		requestBuilder: NewRequestBuilder(),
	}
}

func (c *Client) newRequest(ctx context.Context, method, url string, setters ...requestOption) (*http.Request, error) {
	// Default Options
	args := &requestOptions{
		body:   MessageCompletionRequest{},
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

func (c *Client) handleErrorResp(resp *http.Response) error {
	// Print the response text
	var errRes openai.Error
	if err := json.NewDecoder(resp.Body).Decode(&errRes); err != nil {
		return &openai.Error{
			StatusCode: resp.StatusCode,
			Message:    err.Error(),
		}
	}
	errRes.StatusCode = resp.StatusCode
	return &errRes
}

// Candidate represents a response candidate generated from the model.
type Candidate struct {
	Content      Content `json:"content,omitempty"`
	FinishReason string  `json:"finishReason,omitempty"`
	TokenCount   uint    `json:"tokenCount,omitempty"`
	Index        uint    `json:"index,omitempty"`
}

// CompletionMessageResponse represents a response to an Google completion message.
type CompletionMessageResponse struct {
	Candidates []Candidate `json:"candidates,omitempty"`
}

// Stream struct represents a stream of messages from the Google API.
type Stream struct {
	isFinished bool

	reader      *bufio.Reader
	response    *http.Response
	err         error
	unmarshaler Unmarshaler

	httpHeader
}

// CallTools implements stream.Stream.
func (s *Stream) CallTools() []proto.ToolCallStatus {
	// No tool calls in Gemini/Google API yet.
	return nil
}

// Err implements stream.Stream.
func (s *Stream) Err() error { return s.err }

// Messages implements stream.Stream.
func (s *Stream) Messages() []proto.Message {
	// Gemini does not support returning streamed messages after the fact.
	return nil
}

// Next implements stream.Stream.
func (s *Stream) Next() bool {
	return !s.isFinished
}

// Close closes the stream.
func (s *Stream) Close() error {
	return s.response.Body.Close() //nolint:wrapcheck
}

// Current implements stream.Stream.
//
//nolint:gocognit
func (s *Stream) Current() (proto.Chunk, error) {
	var (
		emptyMessagesCount uint
		hasError           bool
	)

	for {
		rawLine, readErr := s.reader.ReadBytes('\n')
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				s.isFinished = true
				return proto.Chunk{}, stream.ErrNoContent // signals end of stream, not a real error
			}
			return proto.Chunk{}, fmt.Errorf("googleStreamReader.processLines: %w", readErr)
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
				return proto.Chunk{}, fmt.Errorf("googleStreamReader.processLines: %s", noSpaceLine)
			}
			emptyMessagesCount++
			if emptyMessagesCount > emptyMessagesLimit {
				return proto.Chunk{}, ErrTooManyEmptyStreamMessages
			}
			continue
		}

		noPrefixLine := bytes.TrimPrefix(noSpaceLine, googleHeaderData)

		var chunk CompletionMessageResponse
		unmarshalErr := s.unmarshaler.Unmarshal(noPrefixLine, &chunk)
		if unmarshalErr != nil {
			return proto.Chunk{}, fmt.Errorf("googleStreamReader.processLines: %w", unmarshalErr)
		}
		if len(chunk.Candidates) == 0 {
			return proto.Chunk{}, stream.ErrNoContent
		}
		parts := chunk.Candidates[0].Content.Parts
		if len(parts) == 0 {
			return proto.Chunk{}, stream.ErrNoContent
		}

		return proto.Chunk{
			Content: chunk.Candidates[0].Content.Parts[0].Text,
		}, nil
	}
}

func googleSendRequestStream(client *Client, req *http.Request) (*Stream, error) {
	req.Header.Set("content-type", "application/json")

	resp, err := client.config.HTTPClient.Do(req) //nolint:bodyclose // body is closed in stream.Close()
	if err != nil {
		return new(Stream), err
	}
	if isFailureStatusCode(resp) {
		return new(Stream), client.handleErrorResp(resp)
	}
	return &Stream{
		reader:      bufio.NewReader(resp.Body),
		response:    resp,
		unmarshaler: &JSONUnmarshaler{},
		httpHeader:  httpHeader(resp.Header),
	}, nil
}
