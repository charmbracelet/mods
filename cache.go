package main

import (
	"crypto/rand"
	"crypto/sha1" //nolint:gosec
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	name := cfg.loadFrom
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
	name := cfg.saveTo
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
	if err != nil {
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
	return os.Remove(filepath.Join(cfg.CachePath, cfg.Delete+cacheExt))
}
