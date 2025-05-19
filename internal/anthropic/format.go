package anthropic

import (
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/charmbracelet/mods/proto"
	"github.com/mark3labs/mcp-go/mcp"
)

func fromMCPTools(mcps map[string][]mcp.Tool) []anthropic.ToolUnionParam {
	var tools []anthropic.ToolUnionParam
	for name, serverTools := range mcps {
		for _, tool := range serverTools {
			tools = append(tools, anthropic.ToolUnionParam{
				OfTool: &anthropic.ToolParam{
					InputSchema: anthropic.ToolInputSchemaParam{
						Properties: tool.InputSchema.Properties,
					},
					Name:        fmt.Sprintf("%s_%s", name, tool.Name),
					Description: anthropic.String(tool.Description),
				},
			})
		}
	}
	return tools
}

func fromProtoMessages(input []proto.Message) (system []anthropic.TextBlockParam, messages []anthropic.MessageParam) {
	var toolBlocks []anthropic.ContentBlockParamUnion
	for _, msg := range input {
		switch msg.Role {
		case proto.RoleSystem:
			system = append(system, *anthropic.NewTextBlock(msg.Content).OfRequestTextBlock)
		case proto.RoleTool:
			toolBlocks = append(toolBlocks, anthropic.NewToolResultBlock(msg.ToolCallID, msg.Content, false))
		case proto.RoleUser:
			if len(toolBlocks) > 0 {
				messages = append(
					messages,
					anthropic.NewUserMessage(toolBlocks...),
				)
				toolBlocks = nil
			}
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		case proto.RoleAssistant:
			messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content)))
		}
	}
	return system, messages
}

func toProtoMessages(input []anthropic.MessageParam) []proto.Message {
	var messages []proto.Message
	for _, in := range input {
		msg := proto.Message{
			Role: string(in.Role),
		}

		for _, c := range in.Content {
			if block := c.OfRequestTextBlock; block != nil {
				msg.Content += block.Text
			}
		}

		messages = append(messages, msg)
	}
	return messages
}
