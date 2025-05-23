package cohere

import (
	"github.com/charmbracelet/mods/internal/proto"
	cohere "github.com/cohere-ai/cohere-go/v2"
)

func fromProtoMessages(input []proto.Message) (history []*cohere.Message, message string) {
	var messages []*cohere.Message //nolint:prealloc
	for _, msg := range input {
		messages = append(messages, &cohere.Message{
			Role: fromProtoRole(msg.Role),
			Chatbot: &cohere.ChatMessage{
				Message: msg.Content,
			},
		})
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

func fromProtoRole(role string) string {
	switch role {
	case proto.RoleSystem:
		return "SYSTEM"
	case proto.RoleAssistant:
		return "CHATBOT"
	default:
		return "USER"
	}
}
