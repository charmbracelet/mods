package main

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"
)

func (m *Mods) createOpenAIStream(content string, ccfg openai.ClientConfig, mod Model) tea.Msg {
	cfg := m.Config

	client := openai.NewClientWithConfig(ccfg)
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelRequest = cancel

	if err := m.setupStreamContext(content, mod); err != nil {
		return err
	}

	req := openai.ChatCompletionRequest{
		Model:    mod.Name,
		Messages: m.messages,
		Stream:   true,
	}

	if mod.API != "perplexity" || !strings.Contains(mod.Name, "online") {
		req.Temperature = noOmitFloat(cfg.Temperature)
		req.TopP = noOmitFloat(cfg.TopP)
		req.Stop = cfg.Stop
		req.MaxTokens = cfg.MaxTokens
		req.ResponseFormat = responseFormat(cfg)
	}

	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return m.handleRequestError(err, mod, content)
	}

	return m.receiveCompletionStreamCmd(completionOutput{stream: stream})()
}

func (m *Mods) createOllamaStream(content string, occfg OllamaClientConfig, mod Model) tea.Msg {
	cfg := m.Config

	client := NewOllamaClientWithConfig(occfg)
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelRequest = cancel

	if err := m.setupStreamContext(content, mod); err != nil {
		return err
	}

	req := OllamaMessageCompletionRequest{
		Model:    mod.Name,
		Messages: m.messages,
		Stream:   true,
		Options: OllamaMessageCompletionRequestOptions{
			Temperature: noOmitFloat(cfg.Temperature),
			TopP:        noOmitFloat(cfg.TopP),
		},
	}

	if len(cfg.Stop) > 0 {
		req.Options.Stop = cfg.Stop[0]
	}

	if cfg.MaxTokens > 0 {
		req.Options.NumCtx = cfg.MaxTokens
	}

	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return m.handleRequestError(err, mod, content)
	}

	return m.receiveCompletionStreamCmd(completionOutput{stream: stream})()
}

func (m *Mods) createAnthropicStream(content string, accfg AnthropicClientConfig, mod Model) tea.Msg {
	cfg := m.Config

	client := NewAnthropicClientWithConfig(accfg)
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelRequest = cancel

	if err := m.setupStreamContext(content, mod); err != nil {
		return err
	}

	// Anthropic doesn't support the System role so we need to remove those message
	// and, instead, store their content on the `System` request value.
	messages := []openai.ChatCompletionMessage{}

	for _, message := range m.messages {
		if message.Role == openai.ChatMessageRoleSystem {
			m.system += message.Content + "\n"
		} else {
			messages = append(messages, message)
		}
	}

	req := AnthropicMessageCompletionRequest{
		Model:         mod.Name,
		Messages:      messages,
		System:        m.system,
		Stream:        true,
		Temperature:   noOmitFloat(cfg.Temperature),
		TopP:          noOmitFloat(cfg.TopP),
		StopSequences: cfg.Stop,
	}

	if cfg.MaxTokens > 0 {
		req.MaxTokens = cfg.MaxTokens
	} else {
		req.MaxTokens = 4096
	}

	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return m.handleRequestError(err, mod, content)
	}

	return m.receiveCompletionStreamCmd(completionOutput{stream: stream})()
}

func (m *Mods) setupStreamContext(content string, mod Model) error {
	cfg := m.Config
	m.messages = []openai.ChatCompletionMessage{}
	if cfg.Format {
		m.messages = append(m.messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: cfg.FormatText[cfg.FormatAs],
		})
	}

	if cfg.Role != "" {
		roleSetup, ok := cfg.Roles[cfg.Role]
		if !ok {
			return modsError{
				err:    fmt.Errorf("role %q does not exist", cfg.Role),
				reason: "Could not use role",
			}
		}
		for _, msg := range roleSetup {
			content, err := loadMsg(msg)
			if err != nil {
				return modsError{
					err:    err,
					reason: "Could not use role",
				}
			}
			m.messages = append(m.messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleSystem,
				Content: content,
			})
		}
	}

	if prefix := cfg.Prefix; prefix != "" {
		content = strings.TrimSpace(prefix + "\n\n" + content)
	}

	if !cfg.NoLimit && len(content) > mod.MaxChars {
		content = content[:mod.MaxChars]
	}

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

	return nil
}
