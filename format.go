package main

import (
	"encoding/gob"
	"fmt"
	"io"

	openai "github.com/sashabaranov/go-openai"
)

func encode(w io.Writer, messages *[]openai.ChatCompletionMessage) error {
	if err := gob.NewEncoder(w).Encode(messages); err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	return nil
}

func decode(r io.Reader, messages *[]openai.ChatCompletionMessage) error {
	if err := gob.NewDecoder(r).Decode(messages); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}
