package main

import (
	"os"

	"github.com/mattn/go-isatty"
)

var isInputTTY = OnceValue(func() bool {
	return isatty.IsTerminal(os.Stdin.Fd())
})

var isOutputTerminal = OnceValue(func() bool {
	stat, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == os.ModeCharDevice
})
