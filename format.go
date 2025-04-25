package main

import (
	"encoding/gob"
	"fmt"
	"io"

	"github.com/openai/openai-go"
)

type modsMessage struct {
	Role       string                `json:"role"`
	Content    string                `json:"content,omitempty"`
	ToolCallID string                `json:"tool_call_id,omitempty"`
	ToolCalls  []modsMessageToolCall `json:"tool_calls,omitempty"`
}

type modsMessageToolCall struct {
	ID string `json:"id,omitempty"`
}

func convert(in *[]openai.ChatCompletionMessage) *[]modsMessage {
	if in == nil {
		return nil
	}
	var out []modsMessage
	for _, msg := range *in {
		mmsg := modsMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
		if tool := msg.ToParam().OfTool; tool != nil {
			mmsg.ToolCallID = tool.ToolCallID
		}
		for _, call := range msg.ToolCalls {
			mmsg.ToolCalls = append(mmsg.ToolCalls, modsMessageToolCall{
				ID: call.ID,
			})
		}
		out = append(out, mmsg)
	}
	return &out
}

func encode(w io.Writer, messages *[]openai.ChatCompletionMessage) error {
	if err := gob.NewEncoder(w).Encode(convert(messages)); err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	return nil
}

func decode(r io.Reader, messages *[]openai.ChatCompletionMessage) error {
	if err := gob.NewDecoder(r).Decode(convert(messages)); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}
