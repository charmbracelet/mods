package main

import (
	"crypto/rand"
	"crypto/sha1" //nolint:gosec
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const (
	cacheExt  = ".gob"
	sha1short = 7
)

var sha1reg = regexp.MustCompile(`\b[0-9a-f]{40}\b`)

func newConversationID() string {
	b := make([]byte, 1024)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", sha1.Sum(b)) //nolint: gosec
}

func perfectMatchOrMostRecentCache(cfg Config, input string) (string, error) {
	name := input
	if !strings.HasSuffix(name, cacheExt) {
		name += cacheExt
	}

	if _, err := os.Stat(filepath.Join(cfg.CachePath, name)); err == nil {
		return input, nil
	}

	entries, err := listCache(cfg)
	if err != nil {
		return "", err
	}

	var recent time.Time
	var result string
	for _, entry := range entries {
		st, err := os.Stat(filepath.Join(cfg.CachePath, entry+cacheExt))
		if err != nil {
			return "", err
		}
		if st.ModTime().After(recent) {
			recent = st.ModTime()
			result = entry
		}
	}
	return result, nil
}

func findCache(cfg Config, input string) (string, error) {
	if len(input) < 4 {
		return perfectMatchOrMostRecentCache(cfg, input)
	}
	if sha1reg.MatchString(input) {
		return input, nil
	}
	entries, err := listCache(cfg)
	if err != nil {
		return "", err
	}
	var results []string
	for _, entry := range entries {
		if !sha1reg.MatchString(entry) {
			continue
		}
		if strings.HasPrefix(entry, input) {
			results = append(results, entry)
		}
	}
	if len(results) == 0 {
		return perfectMatchOrMostRecentCache(cfg, input)
	}
	if len(results) == 1 {
		return results[0], nil
	}
	return "", fmt.Errorf("multiple conversations matched %q: %s", input, strings.Join(results, ", "))
}

func readCache(messages *[]openai.ChatCompletionMessage, cfg Config) error {
	name := cfg.cacheReadFrom
	if name == "" {
		return nil
	}
	if !strings.HasSuffix(name, cacheExt) {
		name += cacheExt
	}

	file, err := os.Open(filepath.Join(cfg.CachePath, name))
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck

	decoder := gob.NewDecoder(file)
	err = decoder.Decode(messages)
	if err != nil {
		return err
	}

	return nil
}

func writeCache(messages *[]openai.ChatCompletionMessage, cfg Config) error {
	name := cfg.cacheWriteTo
	if name == "" {
		return fmt.Errorf("missing cache name")
	}
	if !strings.HasSuffix(name, cacheExt) {
		name += cacheExt
	}

	err := os.MkdirAll(cfg.CachePath, 0o700)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(cfg.CachePath, name))
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck

	encoder := gob.NewEncoder(file)
	err = encoder.Encode(messages)
	if err != nil {
		return err
	}

	return nil
}

func listCache(cfg Config) ([]string, error) {
	entries, err := os.ReadDir(cfg.CachePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	files := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		files = append(files, strings.TrimSuffix(entry.Name(), cacheExt))
	}

	return files, nil
}

func deleteCache(cfg Config) error {
	return os.Remove(filepath.Join(cfg.CachePath, cfg.cacheWriteTo+cacheExt))
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
