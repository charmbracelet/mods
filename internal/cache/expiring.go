package cache

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ExpiringCache is a cache implementation that supports expiration of cached items.
type ExpiringCache[T any] struct {
	cache *Cache[T]
}

// NewExpiring creates a new cache instance that supports item expiration.
func NewExpiring[T any](path string) (*ExpiringCache[T], error) {
	cache, err := New[T](path, TemporaryCache)
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
