package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	const content = "just text"
	t.Run("normal msg", func(t *testing.T) {
		msg, err := loadMsg(content)
		require.NoError(t, err)
		require.Equal(t, content, msg)
	})

	t.Run("file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "foo.txt")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		msg, err := loadMsg("file://" + path)
		require.NoError(t, err)
		require.Equal(t, content, msg)
	})

	t.Run("http url", func(t *testing.T) {
		msg, err := loadMsg("http://raw.githubusercontent.com/charmbracelet/mods/main/LICENSE")
		require.NoError(t, err)
		require.Contains(t, msg, "MIT License")
	})

	t.Run("https url", func(t *testing.T) {
		msg, err := loadMsg("https://raw.githubusercontent.com/charmbracelet/mods/main/LICENSE")
		require.NoError(t, err)
		require.Contains(t, msg, "MIT License")
	})
}
