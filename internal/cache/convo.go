package cache

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"

	"github.com/charmbracelet/mods/internal/proto"
)

// Conversations is the conversation cache.
type Conversations struct {
	cache *Cache[[]proto.Message]
}

// NewConversations creates a new conversation cache.
func NewConversations(dir string) (*Conversations, error) {
	cache, err := New[[]proto.Message](dir, ConversationCache)
	if err != nil {
		return nil, err
	}
	return &Conversations{
		cache: cache,
	}, nil
}

func (c *Conversations) Read(id string, messages *[]proto.Message) error {
	return c.cache.Read(id, func(r io.Reader) error {
		return decode(r, messages)
	})
}

func (c *Conversations) Write(id string, messages *[]proto.Message) error {
	return c.cache.Write(id, func(w io.Writer) error {
		return encode(w, messages)
	})
}

// Delete a conversation.
func (c *Conversations) Delete(id string) error {
	return c.cache.Delete(id)
}

func init() {
	gob.Register(errors.New(""))
}

func encode(w io.Writer, messages *[]proto.Message) error {
	if err := gob.NewEncoder(w).Encode(messages); err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	return nil
}

func decode(r io.Reader, messages *[]proto.Message) error {
	if err := gob.NewDecoder(r).Decode(messages); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}
