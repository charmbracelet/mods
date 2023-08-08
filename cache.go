package main

import (
	//nolint:gosec
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	openai "github.com/sashabaranov/go-openai"
)

const cacheExt = ".gob"

var errInvalidID = errors.New("invalid id")

type convoCache struct {
	dir string
}

func newCache(dir string) (*convoCache, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil { //nolint: gomnd
		return nil, fmt.Errorf("newCache: %w", err)
	}
	return &convoCache{
		dir: dir,
	}, nil
}

func (c *convoCache) read(id string, messages *[]openai.ChatCompletionMessage) error {
	if id == "" {
		return fmt.Errorf("read: %w", errInvalidID)
	}
	file, err := os.Open(filepath.Join(c.dir, id+cacheExt))
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	defer file.Close() //nolint:errcheck

	if err := decode(file, messages); err != nil {
		return fmt.Errorf("read: %w", err)
	}
	return nil
}

func (c *convoCache) write(id string, messages *[]openai.ChatCompletionMessage) error {
	if id == "" {
		return fmt.Errorf("write: %w", errInvalidID)
	}

	file, err := os.Create(filepath.Join(c.dir, id+cacheExt))
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}
	defer file.Close() //nolint:errcheck

	if err := encode(file, messages); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

func (c *convoCache) delete(id string) error {
	if id == "" {
		return fmt.Errorf("delete: %w", errInvalidID)
	}
	if err := os.Remove(filepath.Join(c.dir, id+cacheExt)); err != nil {
		return fmt.Errorf("delete: %w", err)
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
		prefix += "\n**Response**: "
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
