package main

import (
	"encoding/gob"
	"fmt"
	"io"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared/constant"
)

type modsMessage struct {
	Role       string                `json:"role"`
	Content    string                `json:"content,omitempty"`
	ToolCallID string                `json:"tool_call_id,omitempty"`
	ToolCalls  []modsMessageToolCall `json:"tool_calls,omitempty"`
}

type modsMessageToolCall struct {
	ID       string       `json:"id,omitempty"`
	Function modsFunction `json:"function,omitzero,required"`
}

type modsFunction struct {
	Arguments string `json:"arguments,required"`
	Name      string `json:"name,required"`
}

func fromModsMessages(in []modsMessage) []openai.ChatCompletionMessage {
	var out []openai.ChatCompletionMessage
	for _, msg := range in {
		mmsg := openai.ChatCompletionMessage{
			Role:    constant.Assistant(msg.Role),
			Content: msg.Content,
		}
		for _, call := range msg.ToolCalls {
			mmsg.ToolCalls = append(mmsg.ToolCalls, openai.ChatCompletionMessageToolCall{
				ID: call.ID,
				Function: openai.ChatCompletionMessageToolCallFunction{
					Name:      call.Function.Name,
					Arguments: call.Function.Arguments,
				},
			})
		}
		out = append(out, mmsg)
	}
	return out
}

func toModsMessages(in []openai.ChatCompletionMessage) []modsMessage {
	var out []modsMessage
	for _, msg := range in {
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
				Function: modsFunction{
					Name:      call.Function.Name,
					Arguments: call.Function.Arguments,
				},
			})
			if mmsg.ToolCallID == "" {
				mmsg.ToolCallID = call.ID
			}
		}
		out = append(out, mmsg)
	}
	return out
}

func encode(w io.Writer, messages *[]modsMessage) error {
	if err := gob.NewEncoder(w).Encode(messages); err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	return nil
}

func decode(r io.Reader, messages *[]modsMessage) error {
	if err := gob.NewDecoder(r).Decode(messages); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}
