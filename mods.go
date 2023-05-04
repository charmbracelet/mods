package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
	openai "github.com/sashabaranov/go-openai"
)

const markdownPrefix = "Format the response as Markdown."

const (
	maxCharsGPT4 = 24500
	maxCharsGPT  = 12250
)

type state int

const (
	startState state = iota
	completionState
	errorState
)

// Mods is the Bubble Tea model that manages reading stdin and querying the
// OpenAI API.
type Mods struct {
	Config  config
	Output  string
	Input   string
	Error   *prettyError
	state   state
	retries int
	spinner tea.Model
}

func newMods(cfg config) *Mods {
	var s tea.Model
	if cfg.SimpleSpinner {
		s = newEllipsis()
	} else {
		s = newCyclingChars()
	}
	return &Mods{
		Config:  cfg,
		state:   startState,
		spinner: s,
	}
}

// completionInput is a tea.Msg that wraps the content read from stdin.
type completionInput struct{ content string }

// completionOutput a tea.Msg that wraps the content returned from openai.
type completionOutput struct{ content string }

// prettyError is a wrapper around an error that adds a reason and a pretty
// error message using lipgloss.
type prettyError struct {
	err    error
	reason string
}

func (e prettyError) Error() string {
	var sb strings.Builder
	fmt.Fprintln(&sb)
	fmt.Fprintln(&sb, errorStyle.Render("  Error:", e.reason))
	fmt.Fprintln(&sb)
	fmt.Fprintln(&sb, "  "+errorStyle.Render(e.err.Error()))
	fmt.Fprintln(&sb)
	return sb.String()
}

// Init implements tea.Model.
func (m *Mods) Init() tea.Cmd {
	return tea.Batch(readStdinCmd, m.spinner.Init())
}

// Update implements tea.Model.
func (m *Mods) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case completionInput:
		if msg.content == "" && m.Config.Prefix == "" {
			return m, tea.Quit
		}
		if msg.content != "" {
			m.Input = msg.content
		}
		m.state = completionState
		return m, m.startCompletionCmd(msg.content)
	case completionOutput:
		m.Output = msg.content
		return m, tea.Quit
	case prettyError:
		m.Error = &msg
		m.state = errorState
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m *Mods) View() string {
	//nolint:exhaustive
	switch m.state {
	case errorState:
		return m.Error.Error()
	case completionState:
		if !m.Config.Quiet {
			return m.spinner.View()
		}
	}
	return ""
}

// FormattedOutput returns the response from OpenAI with the user configured
// prefix and standard in settings.
func (m *Mods) FormattedOutput() string {
	prefixFormat := "> %s\n\n---\n\n%s"
	stdinFormat := "```\n%s```\n\n---\n\n%s"
	out := m.Output

	if m.Config.IncludePrompt != 0 {
		if m.Config.IncludePrompt < 0 {
			out = fmt.Sprintf(stdinFormat, m.Input, out)
		}
		scanner := bufio.NewScanner(strings.NewReader(m.Input))
		i := 0
		in := ""
		for scanner.Scan() {
			if i == m.Config.IncludePrompt {
				break
			}
			in += (scanner.Text() + "\n")
			i++
		}
		out = fmt.Sprintf(stdinFormat, in, out)
	}

	if m.Config.IncludePromptArgs || m.Config.IncludePrompt != 0 {
		out = fmt.Sprintf(prefixFormat, m.Config.Prefix, out)
	}

	return out
}

func (m *Mods) retry(content string, err prettyError) tea.Msg {
	m.retries++
	if m.retries >= m.Config.MaxRetries {
		return err
	}
	wait := time.Millisecond * 100 * time.Duration(math.Pow(2, float64(m.retries))) //nolint:gomnd
	time.Sleep(wait)
	return completionInput{content}
}

func (m *Mods) startCompletionCmd(content string) tea.Cmd {
	return func() tea.Msg {
		cfg := m.Config
		key := os.Getenv("OPENAI_API_KEY")
		if key == "" {
			return prettyError{
				err:    fmt.Errorf("You can grab one at %s", linkStyle.Render("https://platform.openai.com/account/api-keys.")),
				reason: codeStyle.Render("OPENAI_API_KEY") + errorStyle.Render(" environment variabled is required."),
			}
		}
		client := openai.NewClient(key)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		prefix := cfg.Prefix
		if cfg.Markdown {
			prefix = fmt.Sprintf("%s %s", prefix, markdownPrefix)
		}
		if prefix != "" {
			content = strings.TrimSpace(prefix + "\n\n" + content)
		}

		if !cfg.NoLimit {
			var maxPromptChars int
			if cfg.Model == "gpt-4" {
				maxPromptChars = maxCharsGPT4
			} else {
				maxPromptChars = maxCharsGPT
			}
			if len(content) > maxPromptChars {
				content = content[:maxPromptChars]
			}
		}

		resp, err := client.CreateChatCompletion(
			ctx,
			openai.ChatCompletionRequest{
				Model:       cfg.Model,
				Temperature: noOmitFloat(cfg.Temperature),
				TopP:        noOmitFloat(cfg.TopP),
				MaxTokens:   cfg.MaxTokens,
				Messages: []openai.ChatCompletionMessage{
					{
						Role:    openai.ChatMessageRoleUser,
						Content: content,
					},
				},
			},
		)
		ae := &openai.APIError{}
		if errors.As(err, &ae) {
			switch ae.HTTPStatusCode {
			case http.StatusBadRequest:
				if ae.Code == "context_length_exceeded" {
					pe := prettyError{err: err, reason: "Maximum prompt size exceeded."}
					if m.Config.NoLimit {
						return pe
					}
					return m.retry(content[:len(content)-10], pe)
				}
				// bad request (do not retry)
				return prettyError{err: err, reason: "OpenAI API request error."}
			case http.StatusUnauthorized:
				// invalid auth or key (do not retry)
				return prettyError{err: err, reason: "Invalid OpenAI API key."}
			case http.StatusTooManyRequests:
				// rate limiting or engine overload (wait and retry)
				return m.retry(content, prettyError{err: err, reason: "You've hit your OpenAI API rate limit."})
			case http.StatusInternalServerError:
				// openai server error (retry)
				return m.retry(content, prettyError{err: err, reason: "OpenAI API server error."})
			default:
				return m.retry(content, prettyError{err: err, reason: "Unknown OpenAI API error."})
			}
		}

		if err != nil {
			return prettyError{err: err, reason: "There was a problem with the OpenAI API request."}
		}
		return completionOutput{resp.Choices[0].Message.Content}
	}
}

func readStdinCmd() tea.Msg {
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		reader := bufio.NewReader(os.Stdin)
		stdinBytes, err := io.ReadAll(reader)
		if err != nil {
			return prettyError{err, "Unable to read stdin."}
		}
		return completionInput{string(stdinBytes)}
	}
	return completionInput{""}
}

// noOmitFloat converts a 0.0 value to a float usable by the OpenAI client
// library, which currently uses Float32 fields in the request struct with the
// omitempty tag. This means we need to use math.SmallestNonzeroFloat32 instead
// of 0.0 so it doesn't get stripped from the request and replaced server side
// with the default values.
// Issue: https://github.com/sashabaranov/go-openai/issues/9
func noOmitFloat(f float32) float32 {
	if f == 0.0 {
		return math.SmallestNonzeroFloat32
	}
	return f
}
