package cohere

import (
	"github.com/charmbracelet/mods/internal/proto"
	cohere "github.com/cohere-ai/cohere-go/v2"
)

func fromProtoMessages(input []proto.Message) (history []*cohere.Message, message string) {
	messages := make([]*cohere.Message, 0, len(input))
	for _, msg := range input {
		switch msg.Role {
		case proto.RoleSystem:
			messages = append(messages, &cohere.Message{
				Role: "SYSTEM",
				System: &cohere.ChatMessage{
					Message: msg.Content,
				},
			})
		case proto.RoleAssistant:
			messages = append(messages, &cohere.Message{
				Role: "CHATBOT",
				Chatbot: &cohere.ChatMessage{
					Message: msg.Content,
				},
			})
		default:
			messages = append(messages, &cohere.Message{
				Role: "USER",
				User: &cohere.ChatMessage{
					Message: msg.Content,
				},
			})
		}
	}
	if len(messages) > 1 {
		history = messages[:len(messages)-1]
	}
	message = messages[len(messages)-1].User.Message
	return history, message
}

func toProtoMessages(input []*cohere.Message) []proto.Message {
	var messages []proto.Message
	for _, in := range input {
		switch in.Role {
		case "USER":
			messages = append(messages, proto.Message{
				Role:    proto.RoleUser,
				Content: in.User.Message,
			})
		case "SYSTEM":
			messages = append(messages, proto.Message{
				Role:    proto.RoleSystem,
				Content: in.System.Message,
			})
		case "CHATBOT":
			messages = append(messages, proto.Message{
				Role:    proto.RoleAssistant,
				Content: in.Chatbot.Message,
			})
		case "TOOL":
			// not supported yet
		}
	}
	return messages
}
