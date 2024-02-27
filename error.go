package main

import "fmt"

// newUserErrorf is a user-facing error.
// this function is mostly to avoid linters complain about errors starting with a capitalized letter.
func newUserErrorf(format string, a ...any) error {
	return fmt.Errorf(format, a...)
}

// modsError is a wrapper around an error that adds additional context.
type modsError struct {
	err    error
	reason string
}

func (m modsError) Error() string {
	return m.err.Error()
}

func (m modsError) Reason() string {
	return m.reason
}
