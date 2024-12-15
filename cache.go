package main

import (
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

type CacheType string

const (
	ConversationCache CacheType = "conversations"
)

// Cache represents a generic cache that can store any type
type Cache[T any] struct {
	baseDir string
	cType   CacheType
}

// NewCache creates a new cache instance
func NewCache[T any](baseDir string, cacheType CacheType) (*Cache[T], error) {
	cacheDir := filepath.Join(baseDir, string(cacheType))
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}
	return &Cache[T]{
		baseDir: baseDir,
		cType:   cacheType,
	}, nil
}

func (c *Cache[T]) cacheDir() string {
	return filepath.Join(c.baseDir, string(c.cType))
}

func (c *Cache[T]) Read(id string, readFn func(io.Reader) error) error {
	if id == "" {
		return fmt.Errorf("read: %w", errInvalidID)
	}
	file, err := os.Open(filepath.Join(c.cacheDir(), id+cacheExt))
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	defer file.Close() //nolint:errcheck

	if err := readFn(file); err != nil {
		return fmt.Errorf("read: %w", err)
	}
	return nil
}

func (c *Cache[T]) Write(id string, writeFn func(io.Writer) error) error {
	if id == "" {
		return fmt.Errorf("write: %w", errInvalidID)
	}

	file, err := os.Create(filepath.Join(c.cacheDir(), id+cacheExt))
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}
	defer file.Close() //nolint:errcheck

	if err := writeFn(file); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

func (c *Cache[T]) Delete(id string) error {
	if id == "" {
		return fmt.Errorf("delete: %w", errInvalidID)
	}
	if err := os.Remove(filepath.Join(c.cacheDir(), id+cacheExt)); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}

// convoCache wraps Cache for backward compatibility
type convoCache struct {
	cache *Cache[[]openai.ChatCompletionMessage]
}

func newCache(dir string) *convoCache {
	cache, err := NewCache[[]openai.ChatCompletionMessage](dir, ConversationCache)
	if err != nil {
		// maintain backward compatibility by handling error silently
		return nil
	}
	return &convoCache{
		cache: cache,
	}
}

func (c *convoCache) read(id string, messages *[]openai.ChatCompletionMessage) error {
	return c.cache.Read(id, func(r io.Reader) error {
		return decode(r, messages)
	})
}

func (c *convoCache) write(id string, messages *[]openai.ChatCompletionMessage) error {
	return c.cache.Write(id, func(w io.Writer) error {
		return encode(w, messages)
	})
}

func (c *convoCache) delete(id string) error {
	return c.cache.Delete(id)
}

var _ chatCompletionReceiver = &cachedCompletionStream{}

type cachedCompletionStream struct {
	messages []openai.ChatCompletionMessage
	read     int
	m        sync.Mutex
}

func (c *cachedCompletionStream) Close() error { return nil }

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
		prefix += "\n**System**: "
	case openai.ChatMessageRoleUser:
		prefix += "\n**Prompt**: "
	case openai.ChatMessageRoleAssistant:
		prefix += "\n**Assistant**: "
	case openai.ChatMessageRoleFunction:
		prefix += "\n**Function**: "
	case openai.ChatMessageRoleTool:
		prefix += "\n**Tool**: "
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
