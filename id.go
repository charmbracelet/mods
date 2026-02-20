package main

import (
	"crypto/rand"
	"fmt"
	"regexp"
)

const (
	convIdShort  = 7
	convIdMinLen = 4
)

var convIdReg = regexp.MustCompile(`\b[0-9a-f]{40}\b`)

func newConversationID() string {
	var b [20]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%x", b)
}
