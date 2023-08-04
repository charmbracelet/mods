package main

import (
	"encoding/gob" //nolint:gosec
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	openai "github.com/sashabaranov/go-openai"
)

const cacheExt = ".gob"

func readCache(messages *[]openai.ChatCompletionMessage, cfg Config, id string) error {
	if id == "" {
		return fmt.Errorf("cannot read empty cache id")
	}
	file, err := os.Open(filepath.Join(cfg.CachePath, id+cacheExt))
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck

	decoder := gob.NewDecoder(file)
	return decoder.Decode(messages)
}

func writeCache(messages *[]openai.ChatCompletionMessage, cfg Config, id string) error {
	if id == "" {
		return fmt.Errorf("cannot write empty cache id")
	}
	if err := os.MkdirAll(cfg.CachePath, 0o700); err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(cfg.CachePath, id+cacheExt))
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck

	encoder := gob.NewEncoder(file)
	return encoder.Encode(messages)
}

func deleteCache(cfg Config, id string) error {
	if id == "" {
		return fmt.Errorf("cannot delete empty cache id")
	}
	return os.Remove(filepath.Join(cfg.CachePath, id+cacheExt))
}

var _ chatCompletionReceiver = &cachedCompletionStream{}

type cachedCompletionStream struct {
	messages []openai.ChatCompletionMessage
	read     int
	m        sync.Mutex
}

func (c *cachedCompletionStream) Close() { /* noop */ }
func (c *cachedCompletionStream) Recv() (openai.ChatCompletionStreamResponse, error) {
	c.m.Lock()
	defer c.m.Unlock()

	if c.read == len(c.messages) {
		return openai.ChatCompletionStreamResponse{}, io.EOF
	}

	msg := c.messages[c.read]
	prefix := ""

	switch msg.Role {
	case openai.ChatMessageRoleSystem:
		prefix += "\n **Response**: "
	case openai.ChatMessageRoleUser:
		if c.read > 0 {
			prefix = "\n---\n"
		}
		prefix += "\n**Prompt**: "
	case openai.ChatMessageRoleAssistant:
		prefix += "\n**Assistant**: "
	case openai.ChatMessageRoleFunction:
		prefix += "\n**Function**: "
	}

	c.read++
	return openai.ChatCompletionStreamResponse{
		Choices: []openai.ChatCompletionStreamChoice{
			{
				Delta: openai.ChatCompletionStreamChoiceDelta{
					Content: prefix + msg.Content + "\n",
					Role:    msg.Role,
				},
			},
		},
	}, nil
}
