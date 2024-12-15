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
	"time"

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

func TestExpiringCache(t *testing.T) {
	t.Run("write and read", func(t *testing.T) {
		config.CachePath = t.TempDir()
		cache, err := NewExpiringCache[string]()
		require.NoError(t, err)

		// Write a value with expiry
		data := "test data"
		expiresAt := time.Now().Add(time.Hour).Unix()
		err = cache.Write("test", expiresAt, func(w io.Writer) error {
			_, err := w.Write([]byte(data))
			return err
		})
		require.NoError(t, err)

		// Read it back
		var result string
		err = cache.Read("test", func(r io.Reader) error {
			b, err := io.ReadAll(r)
			if err != nil {
				return err
			}
			result = string(b)
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, data, result)
	})

	t.Run("expired token", func(t *testing.T) {
		config.CachePath = t.TempDir()
		cache, err := NewExpiringCache[string]()
		require.NoError(t, err)

		// Write a value that's already expired
		data := "test data"
		expiresAt := time.Now().Add(-time.Hour).Unix() // expired 1 hour ago
		err = cache.Write("test", expiresAt, func(w io.Writer) error {
			_, err := w.Write([]byte(data))
			return err
		})
		require.NoError(t, err)

		// Try to read it
		err = cache.Read("test", func(r io.Reader) error {
			return nil
		})
		require.Error(t, err)
		require.True(t, os.IsNotExist(err))
	})

	t.Run("overwrite token", func(t *testing.T) {
		config.CachePath = t.TempDir()
		cache, err := NewExpiringCache[string]()
		require.NoError(t, err)

		// Write initial value
		data1 := "test data 1"
		expiresAt1 := time.Now().Add(time.Hour).Unix()
		err = cache.Write("test", expiresAt1, func(w io.Writer) error {
			_, err := w.Write([]byte(data1))
			return err
		})
		require.NoError(t, err)

		// Write new value
		data2 := "test data 2"
		expiresAt2 := time.Now().Add(2 * time.Hour).Unix()
		err = cache.Write("test", expiresAt2, func(w io.Writer) error {
			_, err := w.Write([]byte(data2))
			return err
		})
		require.NoError(t, err)

		// Read it back - should get the new value
		var result string
		err = cache.Read("test", func(r io.Reader) error {
			b, err := io.ReadAll(r)
			if err != nil {
				return err
			}
			result = string(b)
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, data2, result)
	})
}
