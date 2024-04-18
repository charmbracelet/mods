package main

import (
	"io"
	"net/http"
	"os"
	"strings"
)

func loadMsg(msg string) (string, error) {
	if strings.HasPrefix(msg, "https://") || strings.HasPrefix(msg, "http://") {
		resp, err := http.Get(msg) //nolint: gosec
		if err != nil {
			return "", err
		}
		defer func() { _ = resp.Body.Close() }()
		bts, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(bts), nil
	}

	if strings.HasPrefix(msg, "file://") {
		bts, err := os.ReadFile(strings.TrimPrefix(msg, "file://"))
		if err != nil {
			return "", err
		}
		return string(bts), nil
	}

	return msg, nil
}
