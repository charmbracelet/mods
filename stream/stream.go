package stream

import (
	"context"
	"errors"

	"github.com/charmbracelet/mods/proto"
)

var ErrNoContent = errors.New("no content")

type Client interface {
	Request(context.Context, proto.Request) Stream
}

type Stream interface {
	// returns false when no more messages, caller should run [Stream.CallTolls()]
	// once that happens, and then check for this again
	Next() bool

	// the current chunk
	// implementation should accumulate chunks into a message, and keep its
	// internal conversation state
	Current() (proto.Chunk, error)

	// closes the underlying stream
	Close() error

	// streaming error
	Err() error

	// the whole conversation
	Messages() []proto.Message

	// handles any pending tool calls
	CallTools() []proto.ToolCallStatus
}
