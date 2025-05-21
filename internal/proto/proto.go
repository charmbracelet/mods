// Package proto shared protocol.
package proto

import (
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
				sb.WriteString(fmt.Sprintf("> Ran tool: `%s`\n\n", tool.Function.Name))
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
