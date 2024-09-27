package main

import (
	"encoding/json"
	"os"
)

func getCopilotAuthToken() (string, error) {
	// TODO: Windows?
	bts, err := os.ReadFile(os.Getenv("HOME") + "/.config/github-copilot/hosts.json")
	if err != nil {
		return "", err
	}
	hosts := map[string]map[string]string{}
	if err := json.Unmarshal(bts, &hosts); err != nil {
		return "", err
	}
	return hosts["github.com"]["oauth_token"], nil
}
