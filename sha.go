package main

import (
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"regexp"
)

const (
	sha1short  = 7
	sha1minLen = 4
)

var sha1reg = regexp.MustCompile(`\b[0-9a-f]{40}\b`)

func newConversationID() string {
	b := make([]byte, 4096)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", sha1.Sum(b)) //nolint: gosec
}
