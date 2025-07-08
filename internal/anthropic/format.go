package anthropic

import (
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/charmbracelet/mods/internal/proto"
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
	for _, msg := range input {
		switch msg.Role {
		case proto.RoleSystem:
			// system is not a role in anthropic, must set it as the system part of the request.
			system = append(system, *anthropic.NewTextBlock(msg.Content).OfText)
		case proto.RoleTool:
			for _, call := range msg.ToolCalls {
				block := newToolResultBlock(call.ID, msg.Content, call.IsError)
				//	tool is not a role in anthropic, must be a user message.
				messages = append(messages, anthropic.NewUserMessage(block))
				break
			}
		case proto.RoleUser:
			block := anthropic.NewTextBlock(msg.Content)
			messages = append(messages, anthropic.NewUserMessage(block))
		case proto.RoleAssistant:
			blocks := []anthropic.ContentBlockParamUnion{
				anthropic.NewTextBlock(msg.Content),
			}
			for _, tool := range msg.ToolCalls {
				block := anthropic.ContentBlockParamUnion{
					OfToolUse: &anthropic.ToolUseBlockParam{
						ID:    tool.ID,
						Name:  tool.Function.Name,
						Input: json.RawMessage(tool.Function.Arguments),
					},
				}
				blocks = append(blocks, block)
			}
			messages = append(messages, anthropic.NewAssistantMessage(blocks...))
		}
	}
	return system, messages
}

func toProtoMessage(in anthropic.MessageParam) proto.Message {
	msg := proto.Message{
		Role: string(in.Role),
	}

	for _, block := range in.Content {
		if txt := block.OfText; txt != nil {
			msg.Content += txt.Text
		}

		if call := block.OfToolResult; call != nil {
			msg.ToolCalls = append(msg.ToolCalls, proto.ToolCall{
				ID:      call.ToolUseID,
				IsError: call.IsError.Value,
			})
		}

		if call := block.OfToolUse; call != nil {
			msg.ToolCalls = append(msg.ToolCalls, proto.ToolCall{
				ID: call.ID,
				Function: proto.Function{
					Name:      call.Name,
					Arguments: call.Input.(json.RawMessage),
				},
			})
		}
	}

	return msg
}

// anthropic v1.5 removed this method, copied it back here so we don't need to
// refactor too much.
func newToolResultBlock(toolUseID string, content string, isError bool) anthropic.ContentBlockParamUnion {
	toolBlock := anthropic.ToolResultBlockParam{
		ToolUseID: toolUseID,
		Content: []anthropic.ToolResultBlockParamContentUnion{
			{OfText: &anthropic.TextBlockParam{Text: content}},
		},
		IsError: anthropic.Bool(isError),
	}
	return anthropic.ContentBlockParamUnion{OfToolResult: &toolBlock}
}
