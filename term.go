package main

import (
	"os"

	"github.com/mattn/go-isatty"
)

var isInputTTY = OnceValue(func() bool {
	return isatty.IsTerminal(os.Stdin.Fd())
})

var isOutputTerminal = OnceValue(func() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
})
