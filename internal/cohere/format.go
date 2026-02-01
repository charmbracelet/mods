package cohere

import (
	"github.com/charmbracelet/mods/internal/proto"
	cohere "github.com/cohere-ai/cohere-go/v2"
)

func fromProtoMessages(input []proto.Message) cohere.ChatMessages {
	messages := make(cohere.ChatMessages, 0, len(input))
	for _, msg := range input {
		switch msg.Role {
		case proto.RoleSystem:
			messages = append(messages, &cohere.ChatMessageV2{
				Role: "system",
				System: &cohere.SystemMessageV2{
					Content: &cohere.SystemMessageV2Content{
						String: msg.Content,
					},
				},
			})
		case proto.RoleAssistant:
			messages = append(messages, &cohere.ChatMessageV2{
				Role: "assistant",
				Assistant: &cohere.AssistantMessage{
					Content: &cohere.AssistantMessageV2Content{
						String: msg.Content,
					},
				},
			})
		default:
			messages = append(messages, &cohere.ChatMessageV2{
				Role: "user",
				User: &cohere.UserMessageV2{
					Content: &cohere.UserMessageV2Content{
						String: msg.Content,
					},
				},
			})
		}
	}
	return messages
}

func toProtoMessages(input cohere.ChatMessages) []proto.Message {
	var messages []proto.Message
	for _, in := range input {
		switch in.Role {
		case "user":
			messages = append(messages, proto.Message{
				Role:    proto.RoleUser,
				Content: in.User.Content.String,
			})
		case "system":
			messages = append(messages, proto.Message{
				Role:    proto.RoleSystem,
				Content: in.System.Content.String,
			})
		case "assistant":
			messages = append(messages, proto.Message{
				Role:    proto.RoleAssistant,
				Content: in.Assistant.Content.String,
			})
		case "tool":
			// not supported yet
		}
	}
	return messages
}
