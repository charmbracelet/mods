package main

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
)

var isInputTTY = OnceValue(func() bool {
	return isatty.IsTerminal(os.Stdin.Fd())
})

var isOutputTTY = OnceValue(func() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
})

var stdoutRenderer = OnceValue(func() *lipgloss.Renderer {
	return lipgloss.DefaultRenderer()
})

var stdoutStyles = OnceValue(func() styles {
	return makeStyles(stdoutRenderer())
})

var stderrRenderer = OnceValue(func() *lipgloss.Renderer {
	return lipgloss.NewRenderer(os.Stderr, termenv.WithColorCache(true))
})

var stderrStyles = OnceValue(func() styles {
	return makeStyles(stderrRenderer())
})
