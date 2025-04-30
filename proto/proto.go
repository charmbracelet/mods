package proto

import "github.com/mark3labs/mcp-go/mcp"

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

type Chunk struct {
	Content string
}

type ToolCallStatus struct {
	Name string
	Err  error
}

type Message struct {
	Role       string
	Content    string
	ToolCallID string
	ToolCalls  []MessageToolCall
}

type MessageToolCall struct {
	ID       string
	Function Function
}

type Function struct {
	Arguments string
	Name      string
}

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
