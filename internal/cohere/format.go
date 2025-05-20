package cohere

import (
	"github.com/charmbracelet/mods/proto"
	cohere "github.com/cohere-ai/cohere-go/v2"
)

func fromProtoMessages(input []proto.Message) (history []*cohere.Message, message string) {
	var messages []*cohere.Message
	for _, message := range input {
		switch message.Role {
		case "system":
			messages = append(messages, &cohere.Message{
				Role: "SYSTEM",
				Chatbot: &cohere.ChatMessage{
					Message: message.Content,
				},
			})
		case "assistant":
			messages = append(messages, &cohere.Message{
				Role: "CHATBOT",
				Chatbot: &cohere.ChatMessage{
					Message: message.Content,
				},
			})
		case "user":
			messages = append(messages, &cohere.Message{
				Role: "USER",
				User: &cohere.ChatMessage{
					Message: message.Content,
				},
			})
		}
	}
	if len(messages) > 1 {
		history = messages[:len(messages)-1]
	}
	message = messages[len(messages)-1].User.Message
	return
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
			// TODO: ...
		}
	}
	return messages
}
