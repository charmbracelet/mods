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
	"regexp"
	"strconv"
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
	system        string
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
	gr, _ := glamour.NewTermRenderer(glamour.WithEnvironmentConfig(), glamour.WithWordWrap(cfg.WordWrap))
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
	Close() error
}

// Init implements tea.Model.
func (m *Mods) Init() tea.Cmd {
	return m.findCacheOpsDetails()
}

// Update implements tea.Model.
func (m *Mods) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case cacheDetailsMsg:
		m.Config.cacheWriteToID = msg.WriteID
		m.Config.cacheWriteToTitle = msg.Title
		m.Config.cacheReadFromID = msg.ReadID
		m.Config.Model = msg.Model

		if !m.Config.Quiet {
			m.anim = newAnim(m.Config.Fanciness, m.Config.StatusText, m.renderer, m.Styles)
			cmds = append(cmds, m.anim.Init())
		}
		m.state = configLoadedState
		cmds = append(cmds, m.readStdinCmd)

	case completionInput:
		if msg.content != "" {
			m.Input = removeWhitespace(msg.content)
		}
		if m.Input == "" && m.Config.Prefix == "" && m.Config.Show == "" && !m.Config.ShowLast {
			return m, m.quit
		}
		if m.Config.Dirs ||
			m.Config.Delete != "" ||
			m.Config.DeleteOlderThan != 0 ||
			m.Config.ShowHelp ||
			m.Config.List ||
			m.Config.ListRoles ||
			m.Config.Settings ||
			m.Config.ResetSettings {
			return m, m.quit
		}

		if m.Config.IncludePromptArgs {
			m.appendToOutput(m.Config.Prefix + "\n\n")
		}

		if m.Config.IncludePrompt > 0 {
			parts := strings.Split(m.Input, "\n")
			if len(parts) > m.Config.IncludePrompt {
				parts = parts[0:m.Config.IncludePrompt]
			}
			m.appendToOutput(strings.Join(parts, "\n") + "\n")
		}
		m.state = requestState
		cmds = append(cmds, m.startCompletionCmd(msg.content))
	case completionOutput:
		if msg.stream == nil {
			m.state = doneState
			return m, m.quit
		}
		if msg.content != "" {
			m.appendToOutput(msg.content)
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
	if !m.Config.Quiet && (m.state == configLoadedState || m.state == requestState) {
		var cmd tea.Cmd
		m.anim, cmd = m.anim.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.viewportNeeded() {
		// Only respond to keypresses when the viewport (i.e. the content) is
		// taller than the window.
		var cmd tea.Cmd
		m.glamViewport, cmd = m.glamViewport.Update(msg)
		cmds = append(cmds, cmd)
	}
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
	wait := time.Millisecond * 100 * time.Duration(math.Pow(2, float64(m.retries))) //nolint:mnd
	time.Sleep(wait)
	return completionInput{content}
}

func (m *Mods) startCompletionCmd(content string) tea.Cmd {
	if m.Config.Show != "" || m.Config.ShowLast {
		return m.readFromCache()
	}

	return func() tea.Msg {
		var ok bool
		var mod Model
		var api API
		var ccfg openai.ClientConfig
		var accfg AnthropicClientConfig
		var cccfg CohereClientConfig
		var occfg OllamaClientConfig

		cfg := m.Config
		mod, ok = cfg.Models[cfg.Model]
		if !ok {
			if cfg.API == "" {
				return modsError{
					reason: fmt.Sprintf(
						"Model %s is not in the settings file.",
						m.Styles.InlineCode.Render(cfg.Model),
					),
					err: newUserErrorf(
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
				err: newUserErrorf(
					"Your configured API endpoints are: %s",
					eps,
				),
				reason: fmt.Sprintf(
					"The API endpoint %s is not configured.",
					m.Styles.InlineCode.Render(cfg.API),
				),
			}
		}

		switch mod.API {
		case "ollama":
			occfg = DefaultOllamaConfig()
			if api.BaseURL != "" {
				occfg.BaseURL = api.BaseURL
			}
		case "anthropic":
			key, err := m.ensureKey(api, "ANTHROPIC_API_KEY", "https://console.anthropic.com/settings/keys")
			if err != nil {
				return err
			}
			accfg = DefaultAnthropicConfig(key)
			if api.BaseURL != "" {
				accfg.BaseURL = api.BaseURL
			}
			if api.Version != "" {
				accfg.Version = AnthropicAPIVersion(api.Version)
			}
		case "cohere":
			key, err := m.ensureKey(api, "COHERE_API_KEY", "https://dashboard.cohere.com/api-keys")
			if err != nil {
				return err
			}
			cccfg = DefaultCohereConfig(key)
			if api.BaseURL != "" {
				ccfg.BaseURL = api.BaseURL
			}
		case "azure", "azure-ad":
			key, err := m.ensureKey(api, "AZURE_OPENAI_KEY", "https://aka.ms/oai/access")
			if err != nil {
				return err
			}
			ccfg = openai.DefaultAzureConfig(key, api.BaseURL)
			if mod.API == "azure-ad" {
				ccfg.APIType = openai.APITypeAzureAD
			}
		default:
			key, err := m.ensureKey(api, "OPENAI_API_KEY", "https://platform.openai.com/account/api-keys")
			if err != nil {
				return err
			}
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
			accfg.HTTPClient = httpClient
			cccfg.HTTPClient = httpClient
			occfg.HTTPClient = httpClient
		}

		if mod.MaxChars == 0 {
			mod.MaxChars = cfg.MaxInputChars
		}

		switch mod.API {
		case "anthropic":
			return m.createAnthropicStream(content, accfg, mod)
		case "cohere":
			return m.createCohereStream(content, cccfg, mod)
		case "ollama":
			return m.createOllamaStream(content, occfg, mod)
		default:
			return m.createOpenAIStream(content, ccfg, mod)
		}
	}
}

func (m Mods) ensureKey(api API, defaultEnv, docsURL string) (string, error) {
	key := api.APIKey
	if key == "" && api.APIKeyEnv != "" {
		key = os.Getenv(api.APIKeyEnv)
	}
	if key == "" {
		key = os.Getenv(defaultEnv)
	}
	if key != "" {
		return key, nil
	}
	return "", modsError{
		reason: fmt.Sprintf(
			"%[1]s required; set the environment variable %[1]s or update %[2]s through %[3]s.",
			m.Styles.InlineCode.Render(defaultEnv),
			m.Styles.InlineCode.Render("mods.yaml"),
			m.Styles.InlineCode.Render("mods --settings"),
		),
		err: newUserErrorf(
			"You can grab one at %s.",
			m.Styles.Link.Render(docsURL),
		),
	}
}

func (m *Mods) handleRequestError(err error, mod Model, content string) tea.Msg {
	ae := &openai.APIError{}
	if errors.As(err, &ae) {
		return m.handleAPIError(ae, mod, content)
	}
	return modsError{ae, fmt.Sprintf(
		"There was a problem with the %s API request.",
		mod.API,
	)}
}

func (m *Mods) handleAPIError(err *openai.APIError, mod Model, content string) tea.Msg {
	cfg := m.Config
	switch err.HTTPStatusCode {
	case http.StatusNotFound:
		if mod.Fallback != "" {
			m.Config.Model = mod.Fallback
			return m.retry(content, modsError{
				err:    err,
				reason: fmt.Sprintf("%s API server error.", mod.API),
			})
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

			return m.retry(cutPrompt(err.Message, content), pe)
		}
		// bad request (do not retry)
		return modsError{err: err, reason: fmt.Sprintf("%s API request error.", mod.API)}
	case http.StatusUnauthorized:
		// invalid auth or key (do not retry)
		return modsError{err: err, reason: fmt.Sprintf("Invalid %s API key.", mod.API)}
	case http.StatusTooManyRequests:
		// rate limiting or engine overload (wait and retry)
		return m.retry(content, modsError{
			err: err, reason: fmt.Sprintf("You’ve hit your %s API rate limit.", mod.API),
		})
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
			_ = msg.stream.Close()
			m.messages = append(m.messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: m.Output,
			})
			return completionOutput{}
		}
		if err != nil {
			_ = msg.stream.Close()
			return modsError{err, "There was an error when streaming the API response."}
		}
		if len(resp.Choices) > 0 {
			msg.content = resp.Choices[0].Delta.Content
		}
		return msg
	}
}

type cacheDetailsMsg struct {
	WriteID, Title, ReadID, Model string
}

func (m *Mods) findCacheOpsDetails() tea.Cmd {
	return func() tea.Msg {
		continueLast := m.Config.ContinueLast || (m.Config.Continue != "" && m.Config.Title == "")
		readID := ordered.First(m.Config.Continue, m.Config.Show)
		writeID := ordered.First(m.Config.Title, m.Config.Continue)
		title := writeID
		model := config.Model

		if readID != "" || continueLast || m.Config.ShowLast {
			found, err := m.findReadID(readID)
			if err != nil {
				return modsError{
					err:    err,
					reason: "Could not find the conversation.",
				}
			}
			if found != nil {
				readID = found.ID
				if found.Model != nil {
					model = *found.Model
				}
			}
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
			Model:   model,
		}
	}
}

func (m *Mods) findReadID(in string) (*Conversation, error) {
	convo, err := m.db.Find(in)
	if err == nil {
		return convo, nil
	}
	if errors.Is(err, errNoMatches) && m.Config.Show == "" {
		convo, err := m.db.FindHEAD()
		if err != nil {
			return nil, err
		}
		return convo, nil
	}
	return nil, err
}

func (m *Mods) readStdinCmd() tea.Msg {
	if !isInputTTY() {
		reader := bufio.NewReader(os.Stdin)
		stdinBytes, err := io.ReadAll(reader)
		if err != nil {
			return modsError{err, "Unable to read stdin."}
		}

		return completionInput{increaseIndent(string(stdinBytes))}
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

const tabWidth = 4

func (m *Mods) appendToOutput(s string) {
	m.Output += s
	if !isOutputTTY() || m.Config.Raw {
		m.contentMutex.Lock()
		m.content = append(m.content, s)
		m.contentMutex.Unlock()
		return
	}

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

// if the input is whitespace only, make it empty.
func removeWhitespace(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return s
}

func responseFormat(cfg *Config) *openai.ChatCompletionResponseFormat {
	if cfg.API != "openai" {
		// only openai's api supports ChatCompletionResponseFormat
		return nil
	}
	return &openai.ChatCompletionResponseFormat{
		Type: responseType(cfg),
	}
}

func responseType(cfg *Config) openai.ChatCompletionResponseFormatType {
	if !cfg.Format {
		return openai.ChatCompletionResponseFormatTypeText
	}
	// only these two models support json
	if cfg.Model != "gpt-4-1106-preview" && cfg.Model != "gpt-3.5-turbo-1106" {
		return openai.ChatCompletionResponseFormatTypeText
	}
	switch cfg.FormatAs {
	case "json":
		return openai.ChatCompletionResponseFormatTypeJSONObject
	default:
		return openai.ChatCompletionResponseFormatTypeText
	}
}

var tokenErrRe = regexp.MustCompile(`This model's maximum context length is (\d+) tokens. However, your messages resulted in (\d+) tokens`)

func cutPrompt(msg, prompt string) string {
	found := tokenErrRe.FindStringSubmatch(msg)
	if len(found) != 3 { //nolint:mnd
		return prompt
	}

	max, _ := strconv.Atoi(found[1])
	current, _ := strconv.Atoi(found[2])

	if max > current {
		return prompt
	}

	// 1 token =~ 4 chars
	// cut 10 extra chars 'just in case'
	reduceBy := 10 + (current-max)*4 //nolint:mnd
	if len(prompt) > reduceBy {
		return prompt[:len(prompt)-reduceBy]
	}

	return prompt
}

func increaseIndent(s string) string {
	lines := strings.Split(s, "\n")
	for i := 0; i < len(lines); i++ {
		lines[i] = "\t" + lines[i]
	}
	return strings.Join(lines, "\n")
}
