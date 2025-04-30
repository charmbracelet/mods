package main

import (
	"testing"

	"github.com/openai/openai-go"
	"github.com/stretchr/testify/require"
)

func TestLastPrompt(t *testing.T) {
	t.Run("no prompt", func(t *testing.T) {
		require.Equal(t, "", lastPrompt(nil))
	})

	t.Run("single prompt", func(t *testing.T) {
		require.Equal(t, "single", lastPrompt([]openai.ChatCompletionMessage{
			{
				Role:    roleUser,
				Content: "single",
			},
		}))
	})

	t.Run("multiple prompts", func(t *testing.T) {
		require.Equal(t, "last", lastPrompt([]openai.ChatCompletionMessage{
			{
				Role:    roleUser,
				Content: "first",
			},
			{
				Role:    roleAssistant,
				Content: "hallo",
			},
			{
				Role:    roleUser,
				Content: "middle 1",
			},
			{
				Role:    roleUser,
				Content: "middle 2",
			},
			{
				Role:    roleUser,
				Content: "last",
			},
		}))
	})
}

func TestFirstLine(t *testing.T) {
	t.Run("single line", func(t *testing.T) {
		require.Equal(t, "line", firstLine("line"))
	})
	t.Run("single line ending with \n", func(t *testing.T) {
		require.Equal(t, "line", firstLine("line\n"))
	})
	t.Run("multiple lines", func(t *testing.T) {
		require.Equal(t, "line", firstLine("line\nsomething else\nline3\nfoo\nends with a double \n\n"))
	})
}
