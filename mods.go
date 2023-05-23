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
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	openai "github.com/sashabaranov/go-openai"
)

const markdownPrefix = "Format the response as Markdown."

const (
	maxCharsGPT432k = 98000
	maxCharsGPT4    = 24500
	maxCharsGPT     = 12250
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
	Config   config
	Output   string
	Input    string
	Error    *modsError
	state    state
	retries  int
	styles   styles
	renderer *lipgloss.Renderer
	anim     tea.Model
	width    int
	height   int
}

func newMods(cfg config, r *lipgloss.Renderer) *Mods {
	s := makeStyles(r)
	return &Mods{
		Config:   cfg,
		state:    startState,
		renderer: r,
		styles:   s,
		anim:     newCyclingChars(cfg.Fanciness, cfg.StatusText, r, s),
	}
}

// completionInput is a tea.Msg that wraps the content read from stdin.
type completionInput struct{ content string }

// completionOutput a tea.Msg that wraps the content returned from openai.
type completionOutput struct{ content string }

// modsError is a wrapper around an error that adds additional context.
type modsError struct {
	err    error
	reason string
}

func (m modsError) Error() string {
	return m.err.Error()
}

// Init implements tea.Model.
func (m *Mods) Init() tea.Cmd {
	return tea.Batch(readStdinCmd, m.anim.Init())
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
	case modsError:
		m.Error = &msg
		m.state = errorState
		return m, tea.Quit
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.anim, cmd = m.anim.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m *Mods) View() string {
	//nolint:exhaustive
	switch m.state {
	case errorState:
		return m.errorView()
	case completionState:
		if !m.Config.Quiet {
			return m.anim.View()
		}
	}
	return ""
}

func (m Mods) errorView() string {
	const maxWidth = 120
	const horizontalPadding = 2
	w := m.width - (horizontalPadding * 2)
	if w > maxWidth {
		w = maxWidth
	}
	s := m.renderer.NewStyle().Width(w).Padding(0, horizontalPadding)
	return fmt.Sprintf(
		"\n%s\n\n%s\n\n",
		s.Render(m.styles.errorHeader.String(), m.Error.reason),
		s.Render(m.styles.errorDetails.Render(m.Error.Error())),
	)
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

func (m *Mods) retry(content string, err modsError) tea.Msg {
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
			return modsError{
				err:    fmt.Errorf("You can grab one at %s", m.styles.link.Render("https://platform.openai.com/account/api-keys.")),
				reason: m.styles.inlineCode.Render("OPENAI_API_KEY") + " environment variabled is required.",
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
			switch cfg.Model {
			case "gpt-4":
				maxPromptChars = maxCharsGPT4
			case "gpt-4-32k":
				maxPromptChars = maxCharsGPT432k
			default:
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
			case http.StatusNotFound:
				if m.Config.Model != "gpt-3.5-turbo" {
					m.Config.Model = "gpt-3.5-turbo"
					return m.retry(content, modsError{err: err, reason: "OpenAI API server error."})
				}
			case http.StatusBadRequest:
				if ae.Code == "context_length_exceeded" {
					pe := modsError{err: err, reason: "Maximum prompt size exceeded."}
					if m.Config.NoLimit {
						return pe
					}
					return m.retry(content[:len(content)-10], pe)
				}
				// bad request (do not retry)
				return modsError{err: err, reason: "OpenAI API request error."}
			case http.StatusUnauthorized:
				// invalid auth or key (do not retry)
				return modsError{err: err, reason: "Invalid OpenAI API key."}
			case http.StatusTooManyRequests:
				// rate limiting or engine overload (wait and retry)
				return m.retry(content, modsError{err: err, reason: "You’ve hit your OpenAI API rate limit."})
			case http.StatusInternalServerError:
				// openai server error (retry)
				return m.retry(content, modsError{err: err, reason: "OpenAI API server error."})
			default:
				return m.retry(content, modsError{err: err, reason: "Unknown OpenAI API error."})
			}
		}

		if err != nil {
			return modsError{err: err, reason: "There was a problem with the OpenAI API request."}
		}
		return completionOutput{resp.Choices[0].Message.Content}
	}
}

func readStdinCmd() tea.Msg {
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		reader := bufio.NewReader(os.Stdin)
		stdinBytes, err := io.ReadAll(reader)
		if err != nil {
			return modsError{err, "Unable to read stdin."}
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
