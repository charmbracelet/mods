package main

import (
	"strings"

	"github.com/charmbracelet/mods/proto"
)

func lastPrompt(messages []proto.Message) string {
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
