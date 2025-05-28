package ollama

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/charmbracelet/mods/internal/proto"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/ollama/ollama/api"
)

func fromMCPTools(mcps map[string][]mcp.Tool) []api.Tool {
	var tools []api.Tool
	for name, serverTools := range mcps {
		for _, tool := range serverTools {
			t := api.Tool{
				Type:  "function",
				Items: nil,
				Function: api.ToolFunction{
					Name:        fmt.Sprintf("%s_%s", name, tool.Name),
					Description: tool.Description,
				},
			}
			_ = json.Unmarshal(tool.RawInputSchema, &t.Function.Parameters)
			tools = append(tools, t)
		}
	}
	return tools
}

func fromProtoMessages(input []proto.Message) []api.Message {
	messages := make([]api.Message, 0, len(input))
	for _, msg := range input {
		messages = append(messages, fromProtoMessage(msg))
	}
	return messages
}

func fromProtoMessage(input proto.Message) api.Message {
	m := api.Message{
		Content: input.Content,
		Role:    input.Role,
	}
	for _, call := range input.ToolCalls {
		var args api.ToolCallFunctionArguments
		_ = json.Unmarshal(call.Function.Arguments, &args)
		idx, _ := strconv.Atoi(call.ID)
		m.ToolCalls = append(m.ToolCalls, api.ToolCall{
			Function: api.ToolCallFunction{
				Index:     idx,
				Name:      call.Function.Name,
				Arguments: args,
			},
		})
	}
	return m
}

func toProtoMessage(in api.Message) proto.Message {
	msg := proto.Message{
		Role:    in.Role,
		Content: in.Content,
	}
	for _, call := range in.ToolCalls {
		msg.ToolCalls = append(msg.ToolCalls, proto.ToolCall{
			ID: strconv.Itoa(call.Function.Index),
			Function: proto.Function{
				Arguments: []byte(call.Function.Arguments.String()),
				Name:      call.Function.Name,
			},
		})
	}
	return msg
}
