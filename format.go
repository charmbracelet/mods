package main

import (
	"encoding/gob"
	"fmt"
	"io"

	"github.com/charmbracelet/mods/proto"
)

const (
	roleSystem    = "system"
	roleUser      = "user"
	roleAssistant = "assistant"
	roleFunction  = "function"
	roleTool      = "tool"
)

type modsMessage struct {
	Role       string
	Content    string
	ToolCallID string
	ToolCalls  []modsMessageToolCall
}

type modsMessageToolCall struct {
	ID       string
	Function modsFunction
}

type modsFunction struct {
	Arguments string
	Name      string
}

func encode(w io.Writer, messages *[]proto.Message) error {
	if err := gob.NewEncoder(w).Encode(messages); err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	return nil
}

func decode(r io.Reader, messages *[]proto.Message) error {
	if err := gob.NewDecoder(r).Decode(messages); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}
