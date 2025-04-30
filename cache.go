package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/mods/proto"
	"github.com/charmbracelet/mods/stream"
)

// CacheType represents the type of cache being used.
type CacheType string

// Cache types for different purposes.
const (
	ConversationCache CacheType = "conversations"
	TemporaryCache    CacheType = "temp"
)

const cacheExt = ".gob"

var errInvalidID = errors.New("invalid id")

// Cache is a generic cache implementation that stores data in files.
type Cache[T any] struct {
	baseDir string
	cType   CacheType
}

// NewCache creates a new cache instance with the specified base directory and cache type.
func NewCache[T any](baseDir string, cacheType CacheType) (*Cache[T], error) {
	dir := filepath.Join(baseDir, string(cacheType))
	if err := os.MkdirAll(dir, os.ModePerm); err != nil { //nolint:gosec
		return nil, fmt.Errorf("create cache directory: %w", err)
	}
	return &Cache[T]{
		baseDir: baseDir,
		cType:   cacheType,
	}, nil
}

func (c *Cache[T]) dir() string {
	return filepath.Join(c.baseDir, string(c.cType))
}

func (c *Cache[T]) Read(id string, readFn func(io.Reader) error) error {
	if id == "" {
		return fmt.Errorf("read: %w", errInvalidID)
	}
	file, err := os.Open(filepath.Join(c.dir(), id+cacheExt))
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

	file, err := os.Create(filepath.Join(c.dir(), id+cacheExt))
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}
	defer file.Close() //nolint:errcheck

	if err := writeFn(file); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

// Delete removes a cached item by its ID.
func (c *Cache[T]) Delete(id string) error {
	if id == "" {
		return fmt.Errorf("delete: %w", errInvalidID)
	}
	if err := os.Remove(filepath.Join(c.dir(), id+cacheExt)); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}

type convoCache struct {
	cache *Cache[[]proto.Message]
}

func newCache(dir string) *convoCache {
	cache, err := NewCache[[]proto.Message](dir, ConversationCache)
	if err != nil {
		return nil
	}
	return &convoCache{
		cache: cache,
	}
}

func (c *convoCache) read(id string, messages *[]proto.Message) error {
	return c.cache.Read(id, func(r io.Reader) error {
		return decode(r, messages)
	})
}

func (c *convoCache) write(id string, messages *[]proto.Message) error {
	return c.cache.Write(id, func(w io.Writer) error {
		return encode(w, messages)
	})
}

func (c *convoCache) delete(id string) error {
	return c.cache.Delete(id)
}

var _ stream.Stream = &cachedCompletionStream{}

type cachedCompletionStream struct {
	messages []proto.Message
	read     int
	m        sync.Mutex
}

func (c *cachedCompletionStream) CallTools() []proto.ToolCallStatus { return nil }
func (c *cachedCompletionStream) Current() (proto.Chunk, error)     { return proto.Chunk{}, nil }
func (c *cachedCompletionStream) Err() error                        { return nil }
func (c *cachedCompletionStream) Messages() []proto.Message         { return c.messages }
func (c *cachedCompletionStream) Next() bool                        { return false }
func (c *cachedCompletionStream) Close() error                      { return nil }

// ExpiringCache is a cache implementation that supports expiration of cached items.
type ExpiringCache[T any] struct {
	cache *Cache[T]
}

// NewExpiringCache creates a new cache instance that supports item expiration.
func NewExpiringCache[T any]() (*ExpiringCache[T], error) {
	cache, err := NewCache[T](config.CachePath, TemporaryCache)
	if err != nil {
		return nil, fmt.Errorf("create expiring cache: %w", err)
	}
	return &ExpiringCache[T]{cache: cache}, nil
}

func (c *ExpiringCache[T]) getCacheFilename(id string, expiresAt int64) string {
	return fmt.Sprintf("%s.%d", id, expiresAt)
}

func (c *ExpiringCache[T]) Read(id string, readFn func(io.Reader) error) error {
	pattern := fmt.Sprintf("%s.*", id)
	matches, err := filepath.Glob(filepath.Join(c.cache.dir(), pattern))
	if err != nil {
		return fmt.Errorf("failed to read read expiring cache: %w", err)
	}

	if len(matches) == 0 {
		return fmt.Errorf("item not found")
	}

	filename := filepath.Base(matches[0])
	parts := strings.Split(filename, ".")
	expectedFilenameParts := 2 // name and expiration timestamp

	if len(parts) != expectedFilenameParts {
		return fmt.Errorf("invalid cache filename")
	}

	expiresAt, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid expiration timestamp")
	}

	if expiresAt < time.Now().Unix() {
		if err := os.Remove(matches[0]); err != nil {
			return fmt.Errorf("failed to remove expired cache file: %w", err)
		}
		return os.ErrNotExist
	}

	file, err := os.Open(matches[0])
	if err != nil {
		return fmt.Errorf("failed to open expiring cache file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			err = cerr
		}
	}()

	return readFn(file)
}

func (c *ExpiringCache[T]) Write(id string, expiresAt int64, writeFn func(io.Writer) error) error {
	pattern := fmt.Sprintf("%s.*", id)
	oldFiles, _ := filepath.Glob(filepath.Join(c.cache.dir(), pattern))
	for _, file := range oldFiles {
		if err := os.Remove(file); err != nil {
			return fmt.Errorf("failed to remove old cache file: %w", err)
		}
	}

	filename := c.getCacheFilename(id, expiresAt)
	file, err := os.Create(filepath.Join(c.cache.dir(), filename))
	if err != nil {
		return fmt.Errorf("failed to create expiring cache file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			err = cerr
		}
	}()

	return writeFn(file)
}

// Delete removes an expired cached item by its ID.
func (c *ExpiringCache[T]) Delete(id string) error {
	pattern := fmt.Sprintf("%s.*", id)
	matches, err := filepath.Glob(filepath.Join(c.cache.dir(), pattern))
	if err != nil {
		return fmt.Errorf("failed to delete expiring cache: %w", err)
	}

	for _, match := range matches {
		if err := os.Remove(match); err != nil {
			return fmt.Errorf("failed to delete expiring cache file: %w", err)
		}
	}

	return nil
}
