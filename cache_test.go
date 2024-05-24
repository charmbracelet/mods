package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "update .golden files")

func TestCache(t *testing.T) {
	t.Run("read non-existent", func(t *testing.T) {
		cache := newCache(t.TempDir())
		err := cache.read("super-fake", &[]openai.ChatCompletionMessage{})
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("write", func(t *testing.T) {
		cache := newCache(t.TempDir())
		messages := []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "first 4 natural numbers",
			},
			{
				Role:    openai.ChatMessageRoleAssistant,
				Content: "1, 2, 3, 4",
			},
		}
		require.NoError(t, cache.write("fake", &messages))

		result := []openai.ChatCompletionMessage{}
		require.NoError(t, cache.read("fake", &result))

		require.ElementsMatch(t, messages, result)
	})

	t.Run("delete", func(t *testing.T) {
		cache := newCache(t.TempDir())
		cache.write("fake", &[]openai.ChatCompletionMessage{})
		require.NoError(t, cache.delete("fake"))
		require.ErrorIs(t, cache.read("fake", nil), os.ErrNotExist)
	})

	t.Run("invalid id", func(t *testing.T) {
		t.Run("write", func(t *testing.T) {
			cache := newCache(t.TempDir())
			require.ErrorIs(t, cache.write("", nil), errInvalidID)
		})
		t.Run("delete", func(t *testing.T) {
			cache := newCache(t.TempDir())
			require.ErrorIs(t, cache.delete(""), errInvalidID)
		})
		t.Run("read", func(t *testing.T) {
			cache := newCache(t.TempDir())
			require.ErrorIs(t, cache.read("", nil), errInvalidID)
		})
	})
}

func TestCachedCompletionStream(t *testing.T) {
	stream := cachedCompletionStream{
		messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "you are a medieval king",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "first 4 natural numbers",
			},
			{
				Role:    openai.ChatMessageRoleAssistant,
				Content: "1, 2, 3, 4",
			},

			{
				Role:    openai.ChatMessageRoleUser,
				Content: "as a json array",
			},
			{
				Role:    openai.ChatMessageRoleAssistant,
				Content: "[ 1, 2, 3, 4 ]",
			},
			{
				Role:    openai.ChatMessageRoleAssistant,
				Content: "something from an assistant",
			},
			{
				Role:    openai.ChatMessageRoleFunction,
				Content: "something from a function",
			},
		},
	}
	t.Cleanup(func() { require.NoError(t, stream.Close()) })

	var output []string

	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		output = append(output, resp.Choices[0].Delta.Content)
	}

	golden := filepath.Join("testdata", t.Name()+".md.golden")
	content := strings.Join(output, "\n")
	if *update {
		require.NoError(t, os.WriteFile(golden, []byte(content), 0o644))
	}

	bts, err := os.ReadFile(golden)
	require.NoError(t, err)

	require.Equal(t, string(bytes.ReplaceAll(bts, []byte("\r\n"), []byte("\n"))), content)
}
