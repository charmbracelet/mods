package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/ordered"
	openai "github.com/sashabaranov/go-openai"
)

type state int

const (
	startState state = iota
	configLoadedState
	requestState
	responseState
	doneState
	errorState
)

// Mods is the Bubble Tea model that manages reading stdin and querying the
// OpenAI API.
type Mods struct {
	Output        string
	Input         string
	Styles        styles
	Error         *modsError
	state         state
	retries       int
	renderer      *lipgloss.Renderer
	glam          *glamour.TermRenderer
	glamViewport  viewport.Model
	glamOutput    string
	glamHeight    int
	messages      []openai.ChatCompletionMessage
	cancelRequest context.CancelFunc
	anim          tea.Model
	width         int
	height        int

	db     *convoDB
	cache  *convoCache
	Config *Config

	content      []string
	contentMutex *sync.Mutex
}

func newMods(r *lipgloss.Renderer, cfg *Config, db *convoDB, cache *convoCache) *Mods {
	gr, _ := glamour.NewTermRenderer(glamour.WithEnvironmentConfig())
	vp := viewport.New(0, 0)
	vp.GotoBottom()
	return &Mods{
		Styles:       makeStyles(r),
		glam:         gr,
		state:        startState,
		renderer:     r,
		glamViewport: vp,
		contentMutex: &sync.Mutex{},
		db:           db,
		cache:        cache,
		Config:       cfg,
	}
}

// completionInput is a tea.Msg that wraps the content read from stdin.
type completionInput struct {
	content string
}

// completionOutput a tea.Msg that wraps the content returned from openai.
type completionOutput struct {
	content string
	stream  chatCompletionReceiver
}

type chatCompletionReceiver interface {
	Recv() (openai.ChatCompletionStreamResponse, error)
	Close()
}

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
	return m.findCacheOpsDetails()
}

// Update implements tea.Model.
func (m *Mods) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case cacheDetailsMsg:
		m.Config.cacheWriteToID = msg.WriteID
		m.Config.cacheWriteToTitle = msg.Title
		m.Config.cacheReadFromID = msg.ReadID

		m.anim = newAnim(m.Config.Fanciness, m.Config.StatusText, m.renderer, m.Styles)
		m.state = configLoadedState
		cmds = append(cmds, readStdinCmd, m.anim.Init())

	case completionInput:
		if msg.content != "" {
			m.Input = msg.content
		}
		if msg.content == "" && m.Config.Prefix == "" && m.Config.Show == "" {
			return m, m.quit
		}
		m.state = requestState
		cmds = append(cmds, m.startCompletionCmd(msg.content))
	case completionOutput:
		if msg.stream == nil {
			m.state = doneState
			return m, m.quit
		}
		if msg.content != "" {
			m.Output += msg.content
			if !isOutputTTY() || m.Config.Raw {
				m.contentMutex.Lock()
				m.content = append(m.content, msg.content)
				m.contentMutex.Unlock()
			} else {
				const tabWidth = 4
				wasAtBottom := m.glamViewport.ScrollPercent() == 1.0
				oldHeight := m.glamHeight
				m.glamOutput, _ = m.glam.Render(m.Output)
				m.glamOutput = strings.TrimRightFunc(m.glamOutput, unicode.IsSpace)
				m.glamOutput = strings.ReplaceAll(m.glamOutput, "\t", strings.Repeat(" ", tabWidth))
				m.glamHeight = lipgloss.Height(m.glamOutput)
				m.glamOutput += "\n"
				truncatedGlamOutput := m.renderer.NewStyle().MaxWidth(m.width).Render(m.glamOutput)
				m.glamViewport.SetContent(truncatedGlamOutput)
				if oldHeight < m.glamHeight && wasAtBottom {
					// If the viewport's at the bottom and we've received a new
					// line of content, follow the output by auto scrolling to
					// the bottom.
					m.glamViewport.GotoBottom()
				}
			}
			m.state = responseState
		}
		cmds = append(cmds, m.receiveCompletionStreamCmd(msg))
	case modsError:
		m.Error = &msg
		m.state = errorState
		return m, m.quit
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.glamViewport.Width = m.width
		m.glamViewport.Height = m.height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.state = doneState
			return m, m.quit
		}
	}
	if m.state == configLoadedState || m.state == requestState {
		m.anim, cmd = m.anim.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.viewportNeeded() {
		// Only respond to keypresses when the viewport (i.e. the content) is
		// taller than the window.
		m.glamViewport, cmd = m.glamViewport.Update(msg)
	}
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m Mods) viewportNeeded() bool {
	return m.glamHeight > m.height
}

// View implements tea.Model.
func (m *Mods) View() string {
	//nolint:exhaustive
	switch m.state {
	case errorState:
		return ""
	case requestState:
		if !m.Config.Quiet {
			return m.anim.View()
		}
	case responseState:
		if !m.Config.Raw && isOutputTTY() {
			if m.viewportNeeded() {
				return m.glamViewport.View()
			}
			// We don't need the viewport yet.
			return m.glamOutput
		}

		if isOutputTTY() {
			return m.Output
		}

		m.contentMutex.Lock()
		for _, c := range m.content {
			fmt.Print(c)
		}
		m.content = []string{}
		m.contentMutex.Unlock()
	case doneState:
		if !isOutputTTY() {
			fmt.Printf("\n")
		}
		return ""
	}
	return ""
}

func (m *Mods) quit() tea.Msg {
	if m.cancelRequest != nil {
		m.cancelRequest()
	}
	return tea.Quit()
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
	if m.Config.Show != "" {
		return m.readFromCache()
	}

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
					reason: fmt.Sprintf(
						"Model %s is not in the settings file.",
						m.Styles.InlineCode.Render(cfg.Model),
					),
					err: fmt.Errorf(
						"Please specify an API endpoint with %s or configure the model in the settings: %s",
						m.Styles.InlineCode.Render("--api"),
						m.Styles.InlineCode.Render("mods -s"),
					),
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
				eps = append(eps, m.Styles.InlineCode.Render(a.Name))
			}
			return modsError{
				err: fmt.Errorf(
					"Your configured API endpoints are: %s",
					eps,
				),
				reason: fmt.Sprintf(
					"The API endpoint %s is not configured.",
					m.Styles.InlineCode.Render(cfg.API),
				),
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
					reason: fmt.Sprintf(
						"%s environment variable is required.",
						m.Styles.InlineCode.Render("OPENAI_API_KEY"),
					),
					err: fmt.Errorf(
						"You can grab one at %s",
						m.Styles.Link.Render("https://platform.openai.com/account/api-keys."),
					),
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
					reason: fmt.Sprintf(
						"%s environment variable is required.",
						m.Styles.InlineCode.Render("AZURE_OPENAI_KEY"),
					),
					err: fmt.Errorf(
						"You can apply for one at %s",
						m.Styles.Link.Render("https://aka.ms/oai/access"),
					),
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

		if cfg.HTTPProxy != "" {
			proxyURL, err := url.Parse(cfg.HTTPProxy)
			if err != nil {
				return modsError{err, "There was an error parsing your proxy URL."}
			}
			httpClient := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
			ccfg.HTTPClient = httpClient
		}

		client := openai.NewClientWithConfig(ccfg)
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelRequest = cancel
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

		m.messages = []openai.ChatCompletionMessage{}
		if !cfg.NoCache && cfg.cacheReadFromID != "" {
			if err := m.cache.read(cfg.cacheReadFromID, &m.messages); err != nil {
				return modsError{
					err: err,
					reason: fmt.Sprintf(
						"There was a problem reading the cache. Use %s / %s to disable it.",
						m.Styles.InlineCode.Render("--no-cache"),
						m.Styles.InlineCode.Render("NO_CACHE"),
					),
				}
			}
		}

		m.messages = append(m.messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: content,
		})

		stream, err := client.CreateChatCompletionStream(
			ctx,
			openai.ChatCompletionRequest{
				Model:       mod.Name,
				Temperature: noOmitFloat(cfg.Temperature),
				TopP:        noOmitFloat(cfg.TopP),
				MaxTokens:   cfg.MaxTokens,
				Messages:    m.messages,
				Stream:      true,
			},
		)
		ae := &openai.APIError{}
		if errors.As(err, &ae) {
			return m.handleAPIError(ae, cfg, mod, content)
		}

		if err != nil {
			return modsError{err, fmt.Sprintf(
				"There was a problem with the %s API request.",
				mod.API,
			)}
		}

		return m.receiveCompletionStreamCmd(completionOutput{stream: stream})()
	}
}

func (m *Mods) handleAPIError(err *openai.APIError, cfg *Config, mod Model, content string) tea.Msg {
	switch err.HTTPStatusCode {
	case http.StatusNotFound:
		if mod.Fallback != "" {
			m.Config.Model = mod.Fallback
			return m.retry(content, modsError{err: err, reason: "OpenAI API server error."})
		}
		return modsError{err: err, reason: fmt.Sprintf(
			"Missing model '%s' for API '%s'.",
			cfg.Model,
			cfg.API,
		)}
	case http.StatusBadRequest:
		if err.Code == "context_length_exceeded" {
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
		return modsError{err: err, reason: fmt.Sprintf(
			"Error loading model '%s' for API '%s'.",
			mod.Name,
			mod.API,
		)}
	default:
		return m.retry(content, modsError{err: err, reason: "Unknown API error."})
	}
}

func (m *Mods) receiveCompletionStreamCmd(msg completionOutput) tea.Cmd {
	return func() tea.Msg {
		resp, err := msg.stream.Recv()
		if errors.Is(err, io.EOF) {
			msg.stream.Close()
			m.messages = append(m.messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleSystem,
				Content: m.Output,
			})
			return completionOutput{}
		}
		if err != nil {
			msg.stream.Close()
			return modsError{err, "There was an error when streaming the API response."}
		}
		msg.content = resp.Choices[0].Delta.Content
		return msg
	}
}

type cacheDetailsMsg struct {
	WriteID, Title, ReadID string
}

func (m *Mods) findCacheOpsDetails() tea.Cmd {
	return func() tea.Msg {
		continueLast := m.Config.ContinueLast || (m.Config.Continue != "" && m.Config.Title == "")
		readID := ordered.First(m.Config.Continue, m.Config.Show)
		writeID := ordered.First(m.Config.Title, m.Config.Continue)
		title := writeID

		if continueLast && m.Config.Prefix == "" {
			return modsError{
				err:    fmt.Errorf("Missing prompt"),
				reason: "You must specify a prompt.",
			}
		}

		if readID != "" || continueLast {
			found, err := m.findReadID(readID)
			if err != nil {
				return modsError{
					err:    err,
					reason: "Could not find the conversation.",
				}
			}
			readID = found
		}

		// if we are continuing last, update the existing conversation
		if continueLast {
			writeID = readID
		}

		if writeID == "" {
			writeID = newConversationID()
		}

		if !sha1reg.MatchString(writeID) {
			convo, err := m.db.Find(writeID)
			if err != nil {
				// its a new conversation with a title
				writeID = newConversationID()
			} else {
				writeID = convo.ID
			}
		}

		return cacheDetailsMsg{
			WriteID: writeID,
			Title:   title,
			ReadID:  readID,
		}
	}
}

func (m *Mods) findReadID(in string) (string, error) {
	convo, err := m.db.Find(in)
	if err == nil {
		return convo.ID, nil
	}
	if errors.Is(err, errNoMatches) && m.Config.Show == "" {
		convo, err := m.db.FindHEAD()
		if err != nil {
			return "", err
		}
		return convo.ID, nil
	}
	return "", err
}

func readStdinCmd() tea.Msg {
	if !isInputTTY() {
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

func (m *Mods) readFromCache() tea.Cmd {
	return func() tea.Msg {
		var messages []openai.ChatCompletionMessage
		if err := m.cache.read(m.Config.cacheReadFromID, &messages); err != nil {
			return modsError{err, "There was an error loading the conversation."}
		}

		return m.receiveCompletionStreamCmd(completionOutput{
			stream: &cachedCompletionStream{
				messages: messages,
			},
		})()
	}
}
