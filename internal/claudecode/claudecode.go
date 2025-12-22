// Package claudecode implements [stream.Stream] for Claude Code CLI.
package claudecode

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/charmbracelet/mods/internal/proto"
	"github.com/charmbracelet/mods/internal/stream"
)

var _ stream.Client = &Client{}

// Client is a client for the Claude Code CLI.
type Client struct {
	Command          string
	SkipPermissions  bool
	NoSessionPersist bool
	ContinueLatest   bool
	ResumeID         string
}

// Config represents the configuration for the Claude Code CLI client.
type Config struct {
	Command          string
	SkipPermissions  bool
	NoSessionPersist bool
	ContinueLatest   bool
	ResumeID         string
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Command: "claude",
	}
}

// New creates a new Claude Code client.
func New(config Config) *Client {
	cmd := config.Command
	if cmd == "" {
		cmd = "claude"
	}
	return &Client{
		Command:          cmd,
		SkipPermissions:  config.SkipPermissions,
		NoSessionPersist: config.NoSessionPersist,
		ContinueLatest:   config.ContinueLatest,
		ResumeID:         config.ResumeID,
	}
}

// Request implements stream.Client.
func (c *Client) Request(ctx context.Context, request proto.Request) stream.Stream {
	var promptBuilder strings.Builder
	for _, msg := range request.Messages {
		switch msg.Role {
		case proto.RoleSystem:
			promptBuilder.WriteString("[System]: ")
			promptBuilder.WriteString(msg.Content)
			promptBuilder.WriteString("\n\n")
		case proto.RoleUser:
			promptBuilder.WriteString(msg.Content)
			promptBuilder.WriteString("\n")
		case proto.RoleAssistant:
			promptBuilder.WriteString("[Previous response]: ")
			promptBuilder.WriteString(msg.Content)
			promptBuilder.WriteString("\n\n")
		}
	}

	prompt := strings.TrimSpace(promptBuilder.String())

	args := []string{"-p"}
	if c.SkipPermissions {
		args = append(args, "--dangerously-skip-permissions")
	}
	if c.NoSessionPersist {
		args = append(args, "--no-session-persistence")
	}
	if c.ContinueLatest {
		args = append(args, "--continue")
	}
	if c.ResumeID != "" {
		args = append(args, "--resume", c.ResumeID)
	}
	if request.Model != "" && request.Model != "claude-code" {
		args = append(args, "--model", request.Model)
	}
	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, c.Command, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s := &Stream{err: err, done: make(chan struct{})}
		s.cond = sync.NewCond(&s.mu)
		close(s.done)
		return s
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s := &Stream{err: err, done: make(chan struct{})}
		s.cond = sync.NewCond(&s.mu)
		close(s.done)
		return s
	}

	if err := cmd.Start(); err != nil {
		s := &Stream{err: err, done: make(chan struct{})}
		s.cond = sync.NewCond(&s.mu)
		close(s.done)
		return s
	}

	s := &Stream{
		cmd:      cmd,
		stdout:   stdout,
		stderr:   stderr,
		reader:   bufio.NewReader(stdout),
		messages: request.Messages,
		done:     make(chan struct{}),
	}
	s.cond = sync.NewCond(&s.mu)

	// Drain stderr in background to prevent blocking
	go func() {
		stderrBytes, _ := io.ReadAll(stderr)
		s.mu.Lock()
		s.stderrBuf.Write(stderrBytes)
		s.mu.Unlock()
	}()

	go s.read()

	return s
}

// Stream represents a stream from Claude Code CLI.
type Stream struct {
	cmd      *exec.Cmd
	stdout   io.ReadCloser
	stderr   io.ReadCloser
	reader   *bufio.Reader
	cond     *sync.Cond
	err      error
	messages []proto.Message

	mu        sync.Mutex
	chunks    []string
	chunkIdx  int
	done      chan struct{}
	closed    bool
	finished  bool
	final     bool
	stderrBuf strings.Builder

	response strings.Builder
}

func (s *Stream) read() {
	defer close(s.done)

	buf := make([]byte, 4096)
	for {
		n, err := s.reader.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			s.mu.Lock()
			s.chunks = append(s.chunks, chunk)
			s.response.WriteString(chunk)
			s.cond.Broadcast()
			s.mu.Unlock()
		}
		if err != nil {
			if err != io.EOF {
				s.mu.Lock()
				s.err = err
				s.mu.Unlock()
			}
			break
		}
	}

	// Wait for process to finish and capture exit status
	if s.cmd != nil {
		if waitErr := s.cmd.Wait(); waitErr != nil {
			s.mu.Lock()
			if s.err == nil {
				s.err = waitErr
			}
			s.mu.Unlock()
		}
	}

	s.mu.Lock()
	s.finished = true
	s.mu.Unlock()
	s.cond.Broadcast()
}

// Next implements stream.Stream.
func (s *Stream) Next() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for {
		if s.err != nil {
			return false
		}

		if s.chunkIdx < len(s.chunks) {
			return true
		}

		if s.finished {
			if !s.final {
				s.messages = append(s.messages, proto.Message{
					Role:    proto.RoleAssistant,
					Content: s.response.String(),
				})
				s.final = true
			}
			return false
		}

		s.cond.Wait()
	}
}

// Current implements stream.Stream.
func (s *Stream) Current() (proto.Chunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.chunkIdx >= len(s.chunks) {
		return proto.Chunk{}, stream.ErrNoContent
	}

	chunk := s.chunks[s.chunkIdx]
	s.chunkIdx++
	return proto.Chunk{Content: chunk}, nil
}

// Close implements stream.Stream.
func (s *Stream) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	// Kill process if still running (this will cause read() to exit)
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}

	// Wait for read goroutine to finish (process will be waited there)
	<-s.done

	return nil
}

// Err implements stream.Stream.
func (s *Stream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		return nil
	}
	// Include stderr content in error message if available
	if stderr := s.stderrBuf.String(); stderr != "" {
		return fmt.Errorf("%w: %s", s.err, strings.TrimSpace(stderr))
	}
	return s.err
}

// Messages implements stream.Stream.
func (s *Stream) Messages() []proto.Message {
	return s.messages
}

// CallTools implements stream.Stream.
// Claude Code handles tools internally, so this is a no-op.
func (s *Stream) CallTools() []proto.ToolCallStatus {
	return nil
}
