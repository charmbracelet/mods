package main

import (
	"os"

	"github.com/mattn/go-isatty"
)

var isInputTTY = OnceValue(func() bool {
	return isatty.IsTerminal(os.Stdin.Fd())
})

var isOutputTTY = OnceValue(func() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
})

var isErrTTY = OnceValue(func() bool {
	return isatty.IsTerminal(os.Stderr.Fd())
})
