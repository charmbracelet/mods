package openai

import (
	"fmt"

	"github.com/charmbracelet/mods/internal/proto"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared/constant"
)

func fromMCPTools(mcps map[string][]mcp.Tool) []openai.ChatCompletionToolParam {
	var tools []openai.ChatCompletionToolParam
	for name, serverTools := range mcps {
		for _, tool := range serverTools {
			params := map[string]any{
				"type":       "object",
				"properties": tool.InputSchema.Properties,
			}
			if len(tool.InputSchema.Required) > 0 {
				params["required"] = tool.InputSchema.Required
			}

			tools = append(tools, openai.ChatCompletionToolParam{
				Type: constant.Function("function"),
				Function: openai.FunctionDefinitionParam{
					Name:        fmt.Sprintf("%s_%s", name, tool.Name),
					Description: openai.String(tool.Description),
					Parameters:  params,
				},
			})
		}
	}
	return tools
}

func fromProtoMessages(input []proto.Message) []openai.ChatCompletionMessageParamUnion {
	var messages []openai.ChatCompletionMessageParamUnion
	for _, msg := range input {
		switch msg.Role {
		case proto.RoleSystem:
			messages = append(messages, openai.SystemMessage(msg.Content))
		case proto.RoleTool:
			for _, call := range msg.ToolCalls {
				messages = append(messages, openai.ToolMessage(msg.Content, call.ID))
				break
			}
		case proto.RoleUser:
			messages = append(messages, openai.UserMessage(msg.Content))
		case proto.RoleAssistant:
			m := openai.AssistantMessage(msg.Content)
			for _, tool := range msg.ToolCalls {
				m.OfAssistant.ToolCalls = append(m.OfAssistant.ToolCalls, openai.ChatCompletionMessageToolCallParam{
					ID: tool.ID,
					Function: openai.ChatCompletionMessageToolCallFunctionParam{
						Arguments: string(tool.Function.Arguments),
						Name:      tool.Function.Name,
					},
				})
			}
			messages = append(messages, m)
		}
	}
	return messages
}

func toProtoMessage(in openai.ChatCompletionMessageParamUnion) proto.Message {
	msg := proto.Message{
		Role: msgRole(in),
	}
	switch content := in.GetContent().AsAny().(type) {
	case *string:
		if content == nil || *content == "" {
			break
		}
		msg.Content = *content
	case *[]openai.ChatCompletionContentPartTextParam:
		if content == nil || len(*content) == 0 {
			break
		}
		for _, c := range *content {
			msg.Content += c.Text
		}
	}
	if msg.Role == proto.RoleAssistant {
		for _, call := range in.OfAssistant.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, proto.ToolCall{
				ID: call.ID,
				Function: proto.Function{
					Name:      call.Function.Name,
					Arguments: []byte(call.Function.Arguments),
				},
			})
		}
	}
	return msg
}

func msgRole(in openai.ChatCompletionMessageParamUnion) string {
	if in.OfSystem != nil {
		return proto.RoleSystem
	}
	if in.OfAssistant != nil {
		return proto.RoleAssistant
	}
	if in.OfUser != nil {
		return proto.RoleUser
	}
	if in.OfTool != nil {
		return proto.RoleTool
	}
	return ""
}
