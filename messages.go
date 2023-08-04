package main

import (
	openai "github.com/sashabaranov/go-openai"
)

func lastPrompt(messages []openai.ChatCompletionMessage) string {
	var result string
	for _, msg := range messages {
		if msg.Role != openai.ChatMessageRoleUser {
			continue
		}
		result = msg.Content
	}
	return result
}
