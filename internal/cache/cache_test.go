package cache

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/charmbracelet/mods/internal/proto"
	"github.com/stretchr/testify/require"
)

func TestCache(t *testing.T) {
	t.Run("read non-existent", func(t *testing.T) {
		cache, err := NewConversations(t.TempDir())
		require.NoError(t, err)
		err = cache.Read("super-fake", &[]proto.Message{})
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("write", func(t *testing.T) {
		cache, err := NewConversations(t.TempDir())
		require.NoError(t, err)
		messages := []proto.Message{
			{
				Role:    proto.RoleUser,
				Content: "first 4 natural numbers",
			},
			{
				Role:    proto.RoleAssistant,
				Content: "1, 2, 3, 4",
			},
		}
		require.NoError(t, cache.Write("fake", &messages))

		result := []proto.Message{}
		require.NoError(t, cache.Read("fake", &result))

		require.ElementsMatch(t, messages, result)
	})

	t.Run("delete", func(t *testing.T) {
		cache, err := NewConversations(t.TempDir())
		require.NoError(t, err)
		cache.Write("fake", &[]proto.Message{})
		require.NoError(t, cache.Delete("fake"))
		require.ErrorIs(t, cache.Read("fake", nil), os.ErrNotExist)
	})

	t.Run("invalid id", func(t *testing.T) {
		t.Run("write", func(t *testing.T) {
			cache, err := NewConversations(t.TempDir())
			require.NoError(t, err)
			require.ErrorIs(t, cache.Write("", nil), errInvalidID)
		})
		t.Run("delete", func(t *testing.T) {
			cache, err := NewConversations(t.TempDir())
			require.NoError(t, err)
			require.ErrorIs(t, cache.Delete(""), errInvalidID)
		})
		t.Run("read", func(t *testing.T) {
			cache, err := NewConversations(t.TempDir())
			require.NoError(t, err)
			require.ErrorIs(t, cache.Read("", nil), errInvalidID)
		})
	})
}

func TestExpiringCache(t *testing.T) {
	t.Run("write and read", func(t *testing.T) {
		cache, err := NewExpiring[string](t.TempDir())
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
		cache, err := NewExpiring[string](t.TempDir())
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
		cache, err := NewExpiring[string](t.TempDir())
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
