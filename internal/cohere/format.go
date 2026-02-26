package cohere

import (
	"fmt"
	"slices"

	"github.com/charmbracelet/mods/internal/proto"
	cohere "github.com/cohere-ai/cohere-go/v2"
	"github.com/mark3labs/mcp-go/mcp"
)

func fromMCPTools(mcps map[string][]mcp.Tool) []*cohere.ToolV2 {
	var tools []*cohere.ToolV2
	for name, serverTools := range mcps {
		tools = slices.Grow(tools, len(serverTools))
		for _, tool := range serverTools {
			params := map[string]any{
				"type":       "object",
				"properties": tool.InputSchema.Properties,
			}
			if len(tool.InputSchema.Required) > 0 {
				params["required"] = tool.InputSchema.Required
			}

			tools = append(tools, &cohere.ToolV2{
				Function: &cohere.ToolV2Function{
					Name:        fmt.Sprintf("%s_%s", name, tool.Name),
					Description: &tool.Description,
					Parameters:  params,
				},
			})
		}
	}
	return tools
}

func fromProtoMessages(input []proto.Message) cohere.ChatMessages {
	messages := make(cohere.ChatMessages, 0, len(input))
	for _, in := range input {
		switch in.Role {
		case proto.RoleSystem:
			messages = append(messages, &cohere.ChatMessageV2{
				Role: "system",
				System: &cohere.SystemMessageV2{
					Content: &cohere.SystemMessageV2Content{
						String: in.Content,
					},
				},
			})
		case proto.RoleAssistant:
			msg := &cohere.ChatMessageV2{
				Role:      "assistant",
				Assistant: &cohere.AssistantMessage{},
			}
			if len(in.ToolCalls) > 0 {
				for _, call := range in.ToolCalls {
					args := string(call.Function.Arguments)
					if args == "" {
						args = "{}"
					}
					msg.Assistant.ToolCalls = append(msg.Assistant.ToolCalls, &cohere.ToolCallV2{
						Id: call.ID,
						Function: &cohere.ToolCallV2Function{
							Name:      &call.Function.Name,
							Arguments: &args,
						},
					})
				}
			} else {
				msg.Assistant.Content = &cohere.AssistantMessageV2Content{
					String: in.Content,
				}
			}
			messages = append(messages, msg)
		case proto.RoleTool:
			if len(in.ToolCalls) > 0 {
				messages = append(messages, &cohere.ChatMessageV2{
					Role: "tool",
					Tool: &cohere.ToolMessageV2{
						Content: &cohere.ToolMessageV2Content{
							ToolContentList: []*cohere.ToolContent{
								{
									Type: "document",
									Document: &cohere.DocumentContent{
										Document: &cohere.Document{
											Data: map[string]any{"data": in.Content},
											Id:   cohere.String("0"),
										},
									},
								},
							},
						},
						ToolCallId: in.ToolCalls[0].ID,
					},
				})
			}
		default:
			messages = append(messages, &cohere.ChatMessageV2{
				Role: "user",
				User: &cohere.UserMessageV2{
					Content: &cohere.UserMessageV2Content{
						String: in.Content,
					},
				},
			})
		}
	}
	return messages
}

func toProtoMessage(in *cohere.ChatMessageV2) proto.Message {
	switch in.Role {
	case "user":
		return proto.Message{
			Role:    proto.RoleUser,
			Content: in.User.Content.String,
		}
	case "system":
		return proto.Message{
			Role:    proto.RoleSystem,
			Content: in.System.Content.String,
		}
	case "assistant":
		msg := proto.Message{
			Role:    proto.RoleAssistant,
			Content: in.Assistant.GetContent().GetString(),
		}
		if len(in.Assistant.ToolCalls) > 0 {
			msg.ToolCalls = make([]proto.ToolCall, 0, len(in.Assistant.ToolCalls))
			for _, call := range in.Assistant.ToolCalls {
				var name string
				if namePtr := call.GetFunction().GetName(); namePtr != nil {
					name = *namePtr
				}
				var args []byte
				if argsPtr := call.GetFunction().GetArguments(); argsPtr != nil {
					args = []byte(*argsPtr)
				}
				msg.ToolCalls = append(msg.ToolCalls, proto.ToolCall{
					ID: call.Id,
					Function: proto.Function{
						Name:      name,
						Arguments: args,
					},
				})
			}
		}
		return msg
	case "tool":
		return proto.Message{
			Role:    proto.RoleTool,
			Content: in.Tool.Content.String,
			ToolCalls: []proto.ToolCall{{
				ID: in.Tool.ToolCallId,
			}},
		}
	}
	return proto.Message{}
}
