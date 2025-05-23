// Package proto shared protocol.
package proto

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// Roles.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Chunk is a streaming chunk of text.
type Chunk struct {
	Content string
}

// ToolCallStatus is the status of a tool call.
type ToolCallStatus struct {
	Name string
	Err  error
}

func (c ToolCallStatus) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n> Ran tool: `%s`\n", c.Name))
	if c.Err != nil {
		sb.WriteString(">\n> *Failed*:\n> ```\n")
		for line := range strings.SplitSeq(c.Err.Error(), "\n") {
			sb.WriteString("> " + line)
		}
		sb.WriteString("\n> ```\n")
	}
	sb.WriteByte('\n')
	return sb.String()
}

// Message is a message in the conversation.
type Message struct {
	Role      string
	Content   string
	ToolCalls []ToolCall
}

// ToolCall is a tool call in a message.
type ToolCall struct {
	ID       string
	Function Function
	IsError  bool
}

// Function is the function signature of a tool call.
type Function struct {
	Name      string
	Arguments []byte
}

// Request is a chat request.
type Request struct {
	Messages       []Message
	API            string
	Model          string
	User           string
	Tools          map[string][]mcp.Tool
	Temperature    *float64
	TopP           *float64
	TopK           *int64
	Stop           []string
	MaxTokens      *int64
	ResponseFormat *string
	ToolCaller     func(name string, data []byte) (string, error)
}

// Conversation is a conversation.
type Conversation []Message

func (cc Conversation) String() string {
	var sb strings.Builder
	for _, msg := range cc {
		if msg.Content == "" {
			continue
		}
		switch msg.Role {
		case RoleSystem:
			sb.WriteString("**System**: ")
		case RoleUser:
			sb.WriteString("**User**: ")
		case RoleTool:
			for _, tool := range msg.ToolCalls {
				s := ToolCallStatus{
					Name: tool.Function.Name,
				}
				if tool.IsError {
					s.Err = errors.New(msg.Content)
				}
				sb.WriteString(s.String())
			}
			continue
		case RoleAssistant:
			sb.WriteString("**Assistant**: ")
		}
		sb.WriteString(msg.Content)
		sb.WriteString("\n\n")
	}
	return sb.String()
}
