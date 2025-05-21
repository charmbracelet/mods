package openai

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/mods/proto"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared/constant"
)

func fromMCPTools(mcps map[string][]mcp.Tool) []openai.ChatCompletionToolParam {
	var tools []openai.ChatCompletionToolParam
	for name, serverTools := range mcps {
		for _, tool := range serverTools {
			var params map[string]any
			_ = json.Unmarshal(tool.RawInputSchema, &params)
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
			messages = append(messages, openai.ToolMessage(msg.Content, msg.ToolCall.ID))
		case proto.RoleUser:
			messages = append(messages, openai.UserMessage(msg.Content))
		case proto.RoleAssistant:
			m := openai.AssistantMessage(msg.Content)
			if msg.ToolCall.ID != "" {
				m.OfAssistant.ToolCalls = []openai.ChatCompletionMessageToolCallParam{
					{
						ID: msg.ToolCall.ID,
						Function: openai.ChatCompletionMessageToolCallFunctionParam{
							Arguments: string(msg.ToolCall.Function.Arguments),
							Name:      msg.ToolCall.Function.Name,
						},
					},
				}
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
		if content == nil {
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
	if id := in.GetToolCallID(); id != nil {
		msg.ToolCall.ID = *id
	}
	if fn := in.GetFunctionCall(); fn != nil {
		msg.ToolCall.Function = proto.Function{
			Name:      fn.Name,
			Arguments: []byte(fn.Arguments),
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
