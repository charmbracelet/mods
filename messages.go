package main

import (
	"strings"

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

func firstLine(s string) string {
	first, _, _ := strings.Cut(s, "\n")
	return first
}
