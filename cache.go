package main

import (
	"encoding/gob"
	"io"
	"os"
	"path/filepath"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

var (
	cacheExt         = ".gob"
	defaultCacheName = "_current" + cacheExt
)

func readCache(name string, messages *[]openai.ChatCompletionMessage, cfg Config) error {
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

func writeCache(name string, messages *[]openai.ChatCompletionMessage, cfg Config) error {
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

func saveCache(cfg Config) error {
	inputFile, err := os.Open(filepath.Join(cfg.CachePath, defaultCacheName))
	if err != nil {
		return err
	}
	defer inputFile.Close() //nolint:errcheck
	outputFile, err := os.Create(filepath.Join(cfg.CachePath, cfg.Save+cacheExt))
	if err != nil {
		return err
	}
	defer outputFile.Close() //nolint:errcheck
	_, err = io.Copy(outputFile, inputFile)
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
		if entry.IsDir() || entry.Name() == defaultCacheName {
			continue
		}

		files = append(files, strings.TrimSuffix(entry.Name(), cacheExt))
	}

	return files, nil
}

func deleteCache(cfg Config) error {
	return os.Remove(filepath.Join(cfg.CachePath, cfg.Delete+cacheExt))
}
