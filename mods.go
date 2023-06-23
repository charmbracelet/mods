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

type state int

const (
	startState state = iota
	configLoadedState
	completionState
	errorState
)

// Mods is the Bubble Tea model that manages reading stdin and querying the
// OpenAI API.
type Mods struct {
	Config   Config
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

func newMods(r *lipgloss.Renderer) *Mods {
	s := makeStyles(r)
	return &Mods{
		state:    startState,
		renderer: r,
		styles:   s,
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
	return m.loadConfigCmd
}

// Update implements tea.Model.
func (m *Mods) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case Config:
		m.Config = msg
		m.state = configLoadedState
		if m.Config.ShowHelp || m.Config.Version || m.Config.Settings {
			return m, tea.Quit
		}
		m.anim = newAnim(m.Config.Fanciness, m.Config.StatusText, m.renderer, m.styles)
		return m, tea.Batch(readStdinCmd, m.anim.Init())
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
	if m.state == configLoadedState || m.state == completionState {
		var cmd tea.Cmd
		m.anim, cmd = m.anim.Update(msg)
		return m, cmd
	}
	return m, nil
}

// View implements tea.Model.
func (m *Mods) View() string {
	//nolint:exhaustive
	switch m.state {
	case errorState:
		return m.ErrorView()
	case completionState:
		if !m.Config.Quiet {
			return m.anim.View()
		}
	}
	return ""
}

// ErrorView renders the currently set modsError.
func (m Mods) ErrorView() string {
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

	if m.Config.IncludePrompt != 0 && m.Input != "" {
		if m.Config.IncludePrompt < 0 {
			out = fmt.Sprintf(stdinFormat, m.Input, out)
		} else {
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
	}

	if m.Config.IncludePromptArgs || m.Config.IncludePrompt != 0 {
		prefix := m.Config.Prefix
		if m.Config.Format {
			prefix = fmt.Sprintf("%s %s", prefix, m.Config.FormatText)
		}
		out = fmt.Sprintf(prefixFormat, prefix, out)
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

func (m *Mods) loadConfigCmd() tea.Msg {
	cfg, err := newConfig()
	if err != nil {
		return modsError{err, "There was an error in your config file."}
	}
	return cfg
}

func (m *Mods) startCompletionCmd(content string) tea.Cmd {
	return func() tea.Msg {
		var ok bool
		var mod Model
		var api API
		var key string
		var ccfg openai.ClientConfig

		cfg := m.Config
		mod, ok = cfg.Models[cfg.Model]
		if !ok {
			if cfg.API == "" {
				return modsError{
					reason: "Model " + m.styles.inlineCode.Render(cfg.Model) + " is not in the settings file.",
					err:    fmt.Errorf("Please specify an API endpoint with %s or configure the model in the settings: %s", m.styles.inlineCode.Render("--api"), m.styles.inlineCode.Render("mods -s")),
				}
			}
			mod.Name = cfg.Model
			mod.API = cfg.API
			mod.MaxChars = cfg.MaxInputChars
		}
		for _, a := range cfg.APIs {
			if mod.API == a.Name {
				api = a
				break
			}
		}
		if api.Name == "" {
			eps := make([]string, 0)
			for _, a := range cfg.APIs {
				eps = append(eps, m.styles.inlineCode.Render(a.Name))
			}
			return modsError{
				reason: fmt.Sprintf("The API endpoint %s is not configured ", m.styles.inlineCode.Render(cfg.API)),
				err:    fmt.Errorf("Your configured API endpoints are: %s", eps),
			}
		}
		if api.APIKeyEnv != "" {
			key = os.Getenv(api.APIKeyEnv)
		}

		switch mod.API {
		case "openai":
			if key == "" {
				key = os.Getenv("OPENAI_API_KEY")
			}
			if key == "" {
				return modsError{
					reason: m.styles.inlineCode.Render("OPENAI_API_KEY") + " environment variable is required.",
					err:    fmt.Errorf("You can grab one at %s", m.styles.link.Render("https://platform.openai.com/account/api-keys.")),
				}
			}
			ccfg = openai.DefaultConfig(key)
			if api.BaseURL != "" {
				ccfg.BaseURL = api.BaseURL
			}
		case "azure", "azure-ad":
			if key == "" {
				key = os.Getenv("AZURE_OPENAI_KEY")
			}
			if key == "" {
				return modsError{
					reason: m.styles.inlineCode.Render("AZURE_OPENAI_KEY") + " environment variable is required.",
					err:    fmt.Errorf("You can apply for one at %s", m.styles.link.Render("https://aka.ms/oai/access")),
				}
			}
			ccfg = openai.DefaultAzureConfig(key, api.BaseURL)
			if mod.API == "azure-ad" {
				ccfg.APIType = openai.APITypeAzureAD
			}
		default:
			ccfg = openai.DefaultConfig(key)
			if api.BaseURL != "" {
				ccfg.BaseURL = api.BaseURL
			}
		}

		client := openai.NewClientWithConfig(ccfg)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		prefix := cfg.Prefix
		if cfg.Format {
			prefix = fmt.Sprintf("%s %s", prefix, cfg.FormatText)
		}
		if prefix != "" {
			content = strings.TrimSpace(prefix + "\n\n" + content)
		}

		if !cfg.NoLimit {
			if len(content) > mod.MaxChars {
				content = content[:mod.MaxChars]
			}
		}

		resp, err := client.CreateChatCompletion(
			ctx,
			openai.ChatCompletionRequest{
				Model:       mod.Name,
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
				if mod.Fallback != "" {
					m.Config.Model = mod.Fallback
					return m.retry(content, modsError{err: err, reason: "OpenAI API server error."})
				}
				return modsError{err: err, reason: fmt.Sprintf("Missing model '%s' for API '%s'", cfg.Model, cfg.API)}
			case http.StatusBadRequest:
				if ae.Code == "context_length_exceeded" {
					pe := modsError{err: err, reason: "Maximum prompt size exceeded."}
					if cfg.NoLimit {
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
				if mod.API == "openai" {
					return m.retry(content, modsError{err: err, reason: "OpenAI API server error."})
				}
				return modsError{err: err, reason: fmt.Sprintf("Error loading model '%s' for API '%s'", mod.Name, mod.API)}
			default:
				return m.retry(content, modsError{err: err, reason: "Unknown API error."})
			}
		}

		if err != nil {
			return modsError{err: err, reason: fmt.Sprintf("There was a problem with the %s API request.", mod.API)}
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
