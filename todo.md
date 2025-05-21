# Refactor streaming interfaces, serialization model

## Problems

- We need to keep converting everything from and to OpenAI types
- Our streaming API is based on an old, unmaintained package
- Current streaming doesn't properly support, or allow to support, tool calling

## Proposal

Create a stream package with the implementations for each provider:

```go
type Stream interface {
  // returns false when no more messages, caller should run [Stream.CallTools()]
  // once that happens, and then check for this again
  Next() bool

  // the current chunk
  // implementation should accumulate chunks into a message, and keep its
  // internal conversation state
  Current() Chunk

  // closes the underlying stream
  Close() error

  // streaming error
  Err() error

  // the whole conversation
  Messages() []Message

  // handles any pending tool calls
  CallTools() []ToolCallStatus
}

// this is used to stream assistant messages only, only so mods can print the
// stream piece by piece
// mods should NOT build the message from these chunks
// it should instead use the [Stream.Messages()] method to get the whole
// conversation.
// role is always assistant
type Chunk struct {
  Content string
}

type Message struct {
  Content string
  Role string
  ToolCallID string
  ToolCalls []ToolCall
}

type ToolCall struct {
  Arguments string
  Name string
}

type ToolCallStatus struct {
  Name string
  Err error
}
```

The tea model should then pretty print these things, e.g., when a tool is
called, it should print something like:

```markdown
> Ran tool: `tool_name`
```

The `Message` type is the one that will be serialized for caching purposes as
well. This is already partially done, just not in the right places.

`Chunk` should only be used for streaming parts of messages, which will always
be of role `assistant`.

Usage in mods model would look like:

```go
if stream.Next() {
  chunk := stream.Current()
  return completionMessage{
    content: chunk.Content,
  }
}

// stream is done, check for errors
if err := stream.Err(); err != nil {
  // handle
}

results := stream.CallTools()
var msg completionMessage
for _, call := range results {
  msg.Content += fmt.Sprintf("> Ran tool: `%s`", call.Name)
  if call.Err != nil {
    msg.Content += fmt.Sprintf(" - failed: `%s`", call.Err.Error())
  }
  msg.Content += "\n"
}

if len(results) == 0 {
  return msg, io.EOF
}
return msg, nil
```

This is a lot better than the current proposal of having each provider do this
by themselves.
