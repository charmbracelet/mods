package google

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type httpHeader http.Header

// ErrTooManyEmptyStreamMessages represents an error when a stream has sent too many empty messages.
var ErrTooManyEmptyStreamMessages = errors.New("stream has sent too many empty messages")

// Marshaller is an interface for marshalling values to bytes.
type Marshaller interface {
	Marshal(value any) ([]byte, error)
}

// JSONMarshaller is a marshaller that marshals values to JSON.
type JSONMarshaller struct{}

// Marshal marshals a value to JSON.
func (jm *JSONMarshaller) Marshal(value any) ([]byte, error) {
	result, err := json.Marshal(value)
	if err != nil {
		return result, fmt.Errorf("JSONMarshaller.Marshal: %w", err)
	}
	return result, nil
}

// HTTPRequestBuilder is an implementation of OllamaRequestBuilder that builds HTTP requests.
type HTTPRequestBuilder struct {
	marshaller Marshaller
}

// Build builds an HTTP request.
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

type requestOptions struct {
	body   MessageCompletionRequest
	header http.Header
}

type requestOption func(*requestOptions)

func withBody(body MessageCompletionRequest) requestOption {
	return func(args *requestOptions) {
		args.body = body
	}
}

// ErrorAccumulator is an interface for accumulating errors.
type ErrorAccumulator interface {
	Write(p []byte) error
	Bytes() []byte
}

// Unmarshaler is an interface for unmarshalling bytes.
type Unmarshaler interface {
	Unmarshal(data []byte, v any) error
}

func isFailureStatusCode(resp *http.Response) bool {
	return resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest
}

// JSONUnmarshaler is an unmarshaler that unmarshals JSON data.
type JSONUnmarshaler struct{}

// Unmarshal unmarshals JSON data.
func (jm *JSONUnmarshaler) Unmarshal(data []byte, v any) error {
	err := json.Unmarshal(data, v)
	if err != nil {
		return fmt.Errorf("JSONUnmarshaler.Unmarshal: %w", err)
	}
	return nil
}
