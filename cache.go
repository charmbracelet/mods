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

type convoCache struct {
	dir string
}

func newCache(cfg Config) (*convoCache, error) {
	if err := os.MkdirAll(cfg.CachePath, 0o700); err != nil {
		return nil, err
	}
	return &convoCache{
		dir: cfg.CachePath,
	}, nil
}

func (c *convoCache) read(id string, messages *[]openai.ChatCompletionMessage) error {
	if id == "" {
		return fmt.Errorf("cannot read empty cache id")
	}
	file, err := os.Open(filepath.Join(c.dir, id+cacheExt))
	if err != nil {
		return fmt.Errorf("could not read cache: %w", err)
	}
	defer file.Close() //nolint:errcheck

	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(messages); err != nil {
		return fmt.Errorf("could not decode cache: %w", err)
	}
	return nil
}

func (c *convoCache) write(id string, messages *[]openai.ChatCompletionMessage) error {
	if id == "" {
		return fmt.Errorf("cannot write empty cache id")
	}

	file, err := os.Create(filepath.Join(c.dir, id+cacheExt))
	if err != nil {
		return fmt.Errorf("could not create cache file: %w", err)
	}
	defer file.Close() //nolint:errcheck

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(messages); err != nil {
		return fmt.Errorf("failed to encode cache: %w", err)
	}

	return nil
}

func (c *convoCache) delete(id string) error {
	if id == "" {
		return fmt.Errorf("cannot delete empty cache id")
	}
	if err := os.Remove(filepath.Join(c.dir, id+cacheExt)); err != nil {
		return fmt.Errorf("failed to delete cache: %w", err)
	}
	return nil
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
