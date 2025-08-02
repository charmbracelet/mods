package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/caarlos0/go-shellwords"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/mods/internal/anthropic"
	"github.com/charmbracelet/mods/internal/cache"
	"github.com/charmbracelet/mods/internal/cohere"
	"github.com/charmbracelet/mods/internal/copilot"
	"github.com/charmbracelet/mods/internal/google"
	"github.com/charmbracelet/mods/internal/ollama"
	"github.com/charmbracelet/mods/internal/openai"
	"github.com/charmbracelet/mods/internal/proto"
	"github.com/charmbracelet/mods/internal/stream"
	"github.com/charmbracelet/x/exp/ordered"
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
	messages      []proto.Message
	cancelRequest []context.CancelFunc
	anim          tea.Model
	width         int
	height        int

	db     *convoDB
	cache  *cache.Conversations
	Config *Config

	content      []string
	contentMutex *sync.Mutex

	ctx context.Context
}

func newMods(
	ctx context.Context,
	r *lipgloss.Renderer,
	cfg *Config,
	db *convoDB,
	cache *cache.Conversations,
) *Mods {
	gr, _ := glamour.NewTermRenderer(
		glamour.WithEnvironmentConfig(),
		glamour.WithWordWrap(cfg.WordWrap),
	)
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
		ctx:          ctx,
	}
}

// completionInput is a tea.Msg that wraps the content read from stdin.
type completionInput struct {
	content string
}

// completionOutput a tea.Msg that wraps the content returned from openai.
type completionOutput struct {
	content string
	stream  stream.Stream
	errh    func(error) tea.Msg
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
		m.Config.API = msg.API
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
			len(m.Config.Delete) > 0 ||
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
		cmds = append(cmds, m.receiveCompletionStreamCmd(completionOutput{
			stream: msg.stream,
			errh:   msg.errh,
		}))
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

		if isOutputTTY() && !m.Config.Raw {
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
	for _, cancel := range m.cancelRequest {
		cancel()
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
		var mod Model
		var api API
		var ccfg openai.Config
		var accfg anthropic.Config
		var cccfg cohere.Config
		var occfg ollama.Config
		var gccfg google.Config

		cfg := m.Config
		api, mod, err := m.resolveModel(cfg)
		cfg.API = mod.API
		if err != nil {
			return err
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
			occfg = ollama.DefaultConfig()
			if api.BaseURL != "" {
				occfg.BaseURL = api.BaseURL
			}
		case "anthropic":
			key, err := m.ensureKey(api, "ANTHROPIC_API_KEY", "https://console.anthropic.com/settings/keys")
			if err != nil {
				return modsError{err, "Anthropic authentication failed"}
			}
			accfg = anthropic.DefaultConfig(key)
			if api.BaseURL != "" {
				accfg.BaseURL = api.BaseURL
			}
		case "google":
			key, err := m.ensureKey(api, "GOOGLE_API_KEY", "https://aistudio.google.com/app/apikey")
			if err != nil {
				return modsError{err, "Google authentication failed"}
			}
			gccfg = google.DefaultConfig(mod.Name, key)
			gccfg.ThinkingBudget = mod.ThinkingBudget
		case "cohere":
			key, err := m.ensureKey(api, "COHERE_API_KEY", "https://dashboard.cohere.com/api-keys")
			if err != nil {
				return modsError{err, "Cohere authentication failed"}
			}
			cccfg = cohere.DefaultConfig(key)
			if api.BaseURL != "" {
				ccfg.BaseURL = api.BaseURL
			}
		case "azure", "azure-ad": //nolint:goconst
			key, err := m.ensureKey(api, "AZURE_OPENAI_KEY", "https://aka.ms/oai/access")
			if err != nil {
				return modsError{err, "Azure authentication failed"}
			}
			ccfg = openai.Config{
				AuthToken: key,
				BaseURL:   api.BaseURL,
			}
			if mod.API == "azure-ad" {
				ccfg.APIType = "azure-ad"
			}
			if api.User != "" {
				cfg.User = api.User
			}
		case "copilot":
			cli := copilot.New(config.CachePath)
			token, err := cli.Auth()
			if err != nil {
				return modsError{err, "Copilot authentication failed"}
			}

			ccfg = openai.Config{
				AuthToken: token.Token,
				BaseURL:   api.BaseURL,
			}
			ccfg.HTTPClient = cli
			ccfg.BaseURL = ordered.First(api.BaseURL, token.Endpoints.API)

		default:
			key, err := m.ensureKey(api, "OPENAI_API_KEY", "https://platform.openai.com/account/api-keys")
			if err != nil {
				return modsError{err, "OpenAI authentication failed"}
			}
			ccfg = openai.Config{
				AuthToken: key,
				BaseURL:   api.BaseURL,
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

		// Check if the model is an o1 model and unset the max_tokens parameter
		// accordingly, as it's unsupported by o1.
		// We do set max_completion_tokens instead, which is supported.
		// Release won't have a prefix with a dash, so just putting o1 for match.
		if strings.HasPrefix(mod.Name, "o1") {
			cfg.MaxTokens = 0
		}

		ctx, cancel := context.WithTimeout(m.ctx, config.MCPTimeout)
		m.cancelRequest = append(m.cancelRequest, cancel)

		tools, err := mcpTools(ctx)
		if err != nil {
			return err
		}

		if err := m.setupStreamContext(content, mod); err != nil {
			return err
		}

		request := proto.Request{
			Messages:    m.messages,
			API:         mod.API,
			Model:       mod.Name,
			User:        cfg.User,
			Temperature: ptrOrNil(cfg.Temperature),
			TopP:        ptrOrNil(cfg.TopP),
			TopK:        ptrOrNil(cfg.TopK),
			Stop:        cfg.Stop,
			Tools:       tools,
			ToolCaller: func(name string, data []byte) (string, error) {
				ctx, cancel := context.WithTimeout(m.ctx, config.MCPTimeout)
				m.cancelRequest = append(m.cancelRequest, cancel)
				return toolCall(ctx, name, data)
			},
		}
		if cfg.MaxTokens > 0 {
			request.MaxTokens = &cfg.MaxTokens
		}

		var client stream.Client
		switch mod.API {
		case "anthropic":
			client = anthropic.New(accfg)
		case "google":
			client = google.New(gccfg)
		case "cohere":
			client = cohere.New(cccfg)
		case "ollama":
			client, err = ollama.New(occfg)
		default:
			client = openai.New(ccfg)
			if cfg.Format && config.FormatAs == "json" {
				request.ResponseFormat = &config.FormatAs
			}
		}
		if err != nil {
			return modsError{err, "Could not setup client"}
		}

		stream := client.Request(m.ctx, request)
		return m.receiveCompletionStreamCmd(completionOutput{
			stream: stream,
			errh: func(err error) tea.Msg {
				return m.handleRequestError(err, mod, m.Input)
			},
		})()
	}
}

func (m Mods) ensureKey(api API, defaultEnv, docsURL string) (string, error) {
	key := api.APIKey
	if key == "" && api.APIKeyEnv != "" && api.APIKeyCmd == "" {
		key = os.Getenv(api.APIKeyEnv)
	}
	if key == "" && api.APIKeyCmd != "" {
		args, err := shellwords.Parse(api.APIKeyCmd)
		if err != nil {
			return "", modsError{err, "Failed to parse api-key-cmd"}
		}
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput() //nolint:gosec
		if err != nil {
			return "", modsError{err, "Cannot exec api-key-cmd"}
		}
		key = strings.TrimSpace(string(out))
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
			"You can grab one at %s",
			m.Styles.Link.Render(docsURL),
		),
	}
}

func (m *Mods) receiveCompletionStreamCmd(msg completionOutput) tea.Cmd {
	return func() tea.Msg {
		if msg.stream.Next() {
			chunk, err := msg.stream.Current()
			if err != nil && !errors.Is(err, stream.ErrNoContent) {
				_ = msg.stream.Close()
				return msg.errh(err)
			}
			return completionOutput{
				content: chunk.Content,
				stream:  msg.stream,
				errh:    msg.errh,
			}
		}

		// stream is done, check for errors
		if err := msg.stream.Err(); err != nil {
			return msg.errh(err)
		}

		results := msg.stream.CallTools()
		toolMsg := completionOutput{
			stream: msg.stream,
			errh:   msg.errh,
		}
		for _, call := range results {
			toolMsg.content += call.String()
		}
		if len(results) == 0 {
			m.messages = msg.stream.Messages()
			return completionOutput{
				errh: msg.errh,
			}
		}
		return toolMsg
	}
}

type cacheDetailsMsg struct {
	WriteID, Title, ReadID, API, Model string
}

func (m *Mods) findCacheOpsDetails() tea.Cmd {
	return func() tea.Msg {
		continueLast := m.Config.ContinueLast || (m.Config.Continue != "" && m.Config.Title == "")
		readID := ordered.First(m.Config.Continue, m.Config.Show)
		writeID := ordered.First(m.Config.Title, m.Config.Continue)
		title := writeID
		model := m.Config.Model
		api := m.Config.API

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
				if found.Model != nil && found.API != nil {
					model = *found.Model
					api = *found.API
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
			API:     api,
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

func (m *Mods) readFromCache() tea.Cmd {
	return func() tea.Msg {
		var messages []proto.Message
		if err := m.cache.Read(m.Config.cacheReadFromID, &messages); err != nil {
			return modsError{err, "There was an error loading the conversation."}
		}

		m.appendToOutput(proto.Conversation(messages).String())
		return completionOutput{
			errh: func(err error) tea.Msg {
				return modsError{err: err}
			},
		}
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
	truncatedGlamOutput := m.renderer.NewStyle().
		MaxWidth(m.width).
		Render(m.glamOutput)
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

var tokenErrRe = regexp.MustCompile(`This model's maximum context length is (\d+) tokens. However, your messages resulted in (\d+) tokens`)

func cutPrompt(msg, prompt string) string {
	found := tokenErrRe.FindStringSubmatch(msg)
	if len(found) != 3 { //nolint:mnd
		return prompt
	}

	maxt, _ := strconv.Atoi(found[1])
	current, _ := strconv.Atoi(found[2])

	if maxt > current {
		return prompt
	}

	// 1 token =~ 4 chars
	// cut 10 extra chars 'just in case'
	reduceBy := 10 + (current-maxt)*4 //nolint:mnd
	if len(prompt) > reduceBy {
		return prompt[:len(prompt)-reduceBy]
	}

	return prompt
}

func increaseIndent(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = "\t" + lines[i]
	}
	return strings.Join(lines, "\n")
}

func (m *Mods) resolveModel(cfg *Config) (API, Model, error) {
	for _, api := range cfg.APIs {
		if api.Name != cfg.API && cfg.API != "" {
			continue
		}
		for name, mod := range api.Models {
			if name == cfg.Model || slices.Contains(mod.Aliases, cfg.Model) {
				cfg.Model = name
				break
			}
		}
		mod, ok := api.Models[cfg.Model]
		if ok {
			mod.Name = cfg.Model
			mod.API = api.Name
			return api, mod, nil
		}
		if cfg.API != "" {
			return API{}, Model{}, modsError{
				err: newUserErrorf(
					"Available models are: %s",
					strings.Join(slices.Collect(maps.Keys(api.Models)), ", "),
				),
				reason: fmt.Sprintf(
					"The API endpoint %s does not contain the model %s",
					m.Styles.InlineCode.Render(cfg.API),
					m.Styles.InlineCode.Render(cfg.Model),
				),
			}
		}
	}

	return API{}, Model{}, modsError{
		reason: fmt.Sprintf(
			"Model %s is not in the settings file.",
			m.Styles.InlineCode.Render(cfg.Model),
		),
		err: newUserErrorf(
			"Please specify an API endpoint with %s or configure the model in the settings: %s",
			m.Styles.InlineCode.Render("--api"),
			m.Styles.InlineCode.Render("mods --settings"),
		),
	}
}

type number interface{ int64 | float64 }

func ptrOrNil[T number](t T) *T {
	if t < 0 {
		return nil
	}
	return &t
}
