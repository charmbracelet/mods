package main

import (
	"io"
	"os"
	"sync/atomic"

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

var _ io.ReadWriter = &modsOutput{}

type modsOutput struct {
	loadingDone *atomic.Bool
}

// Read implements io.Reader.
func (o *modsOutput) Read(p []byte) (n int, err error) {
	if o.loadingDone.Load() && !isOutputTerminal() {
		return os.Stdout.Read(p)
	}

	return os.Stderr.Read(p)
}

// Write implements io.Writer.
func (o *modsOutput) Write(p []byte) (n int, err error) {
	if o.loadingDone.Load() && !isOutputTerminal() {
		return os.Stdout.Write(p)
	}

	return os.Stderr.Write(p)
}

func (o *modsOutput) Fd() uintptr {
	if o.loadingDone.Load() && !isOutputTerminal() {
		return os.Stdout.Fd()
	}

	return os.Stderr.Fd()
}
