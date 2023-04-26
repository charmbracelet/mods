package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
	openai "github.com/sashabaranov/go-openai"
)

const markdownPrefix = "Format the response as Markdown."

type state int

const (
	startState state = iota
	completionState
	quitState
	errorState
)

// Mods is the Bubble Tea model that manages reading stdin and querying the
// OpenAI API.
type Mods struct {
	Config  config
	Output  string
	Input   string
	state   state
	error   prettyError
	spinner tea.Model
}

func newMods(cfg config) Mods {
	var s tea.Model
	if cfg.AltSpinner {
		s = spinner(0)
	} else {
		s = newCyclingChars()
	}
	return Mods{
		Config:  cfg,
		state:   startState,
		spinner: s,
	}
}

// stdinContent is a tea.Msg that wraps the content read from stdin.
type stdinContent struct{ content string }

// completionOutput a tea.Msg that wraps the content returned from openai.
type completionOutput struct{ output string }

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

func readStdinCmd() tea.Msg {
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		reader := bufio.NewReader(os.Stdin)
		stdinBytes, err := io.ReadAll(reader)
		if err != nil {
			return prettyError{err, "Unable to read stdin."}
		}
		return stdinContent{string(stdinBytes)}
	}
	return stdinContent{""}
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

func startCompletionCmd(cfg config, content string) tea.Cmd {
	return func() tea.Msg {
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

		var maxPromptChars int
		if cfg.Model == "gpt-4" {
			maxPromptChars = 25500
		} else {
			maxPromptChars = 12250
		}
		if len(content)-1 < maxPromptChars {
			maxPromptChars = len(content) - 1
		}
		content = content[:maxPromptChars]

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
		if err != nil {
			return prettyError{err: err, reason: "There was a problem with the OpenAI API."}
		}
		return completionOutput{resp.Choices[0].Message.Content}
	}
}

// Init implements tea.Model.
func (m Mods) Init() tea.Cmd {
	return tea.Batch(readStdinCmd, m.spinner.Init())
}

// Update implements tea.Model.
func (m Mods) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case stdinContent:
		if msg.content == "" && m.Config.Prefix == "" {
			m.state = quitState
			return m, tea.Quit
		}
		if msg.content != "" {
			m.Input = msg.content
		}
		m.state = completionState
		return m, startCompletionCmd(m.Config, msg.content)
	case completionOutput:
		m.Output = msg.output
		m.state = quitState
		return m, tea.Quit
	case prettyError:
		m.error = msg
		m.state = errorState
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.state = quitState
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m Mods) View() string {
	switch m.state {
	case errorState:
		return m.error.Error()
	case completionState:
		if !m.Config.Quiet {
			return m.spinner.View()
		}
	}
	return ""
}

// FormattedOutput returns the response from OpenAI with the user configured
// prefix and standard in settings.
func (m Mods) FormattedOutput() string {
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
