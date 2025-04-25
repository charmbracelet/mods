package main

import (
	"strings"

	"github.com/openai/openai-go"
)

func lastPrompt(messages []openai.ChatCompletionMessage) string {
	var result string
	for _, msg := range messages {
		if msg.Role != "user" {
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
