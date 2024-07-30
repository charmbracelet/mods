package main

import (
	"strings"
	"testing"
)

func TestIsCompletionCmd(t *testing.T) {
	for args, is := range map[string]bool{
		"":                                     false,
		"something":                            false,
		"something something":                  false,
		"completion for my bash script how to": false,
		"completion bash how to":               false,
		"completion":                           false,
		"completion -h":                        true,
		"completion --help":                    true,
		"completion help":                      true,
		"completion bash":                      true,
		"completion fish":                      true,
		"completion zsh":                       true,
		"completion powershell":                true,
		"completion bash -h":                   true,
		"completion fish -h":                   true,
		"completion zsh -h":                    true,
		"completion powershell -h":             true,
		"completion bash --help":               true,
		"completion fish --help":               true,
		"completion zsh --help":                true,
		"completion powershell --help":         true,
		"__complete":                           true,
		"__complete blah blah blah":            true,
	} {
		t.Run(args, func(t *testing.T) {
			vargs := append([]string{"mods"}, strings.Fields(args)...)
			if b := isCompletionCmd(vargs); b != is {
				t.Errorf("%v: expected %v, got %v", vargs, is, b)
			}
		})
	}
}

func TestIsManCmd(t *testing.T) {
	for args, is := range map[string]bool{
		"":                    false,
		"something":           false,
		"something something": false,
		"man is no more":      false,
		"mans":                false,
		"man foo":             false,
		"man":                 true,
		"man -h":              true,
		"man --help":          true,
	} {
		t.Run(args, func(t *testing.T) {
			vargs := append([]string{"mods"}, strings.Fields(args)...)
			if b := isManCmd(vargs); b != is {
				t.Errorf("%v: expected %v, got %v", vargs, is, b)
			}
		})
	}
}
