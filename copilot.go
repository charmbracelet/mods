package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

func getCopilotAuthToken() (string, error) {
	var sessionPath string
	if runtime.GOOS == "windows" {
		// C:\Users\user\AppData\Local\github-copilot\hosts.json
		sessionPath = filepath.Join(os.Getenv("LOCALAPPDATA"), "github-copilot", "hosts.json")
	} else {
		// ~/.config/github-copilot/hosts.json
		sessionPath = filepath.Join(os.Getenv("HOME"), ".config/github-copilot", "hosts.json")
	}

	bts, err := os.ReadFile(sessionPath)
	if err != nil {
		return "", err
	}
	hosts := map[string]map[string]string{}
	if err := json.Unmarshal(bts, &hosts); err != nil {
		return "", err
	}
	return hosts["github.com"]["oauth_token"], nil
}
