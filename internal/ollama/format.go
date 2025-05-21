package ollama

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/mods/proto"
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
		messages = append(messages, api.Message{
			Content: msg.Content,
			Role:    msg.Role,
		})
	}
	return messages
}

func toProtoMessages(input []api.Message) []proto.Message {
	messages := make([]proto.Message, 0, len(input))
	for _, in := range input {
		msg := proto.Message{
			Role:    in.Role,
			Content: in.Content,
		}
		// for _, call := range in.ToolCalls {
		// 	msg.ToolCalls = append(msg.ToolCalls, proto.MessageToolCall{
		// 		// ID:       call.Function.Index, XXX: ?
		// 		Function: proto.Function{
		// 			Arguments: call.Function.Arguments.String(),
		// 			Name:      call.Function.Name,
		// 		},
		// 	})
		// }
		messages = append(messages, msg)
	}
	return messages
}
