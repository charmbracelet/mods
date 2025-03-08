package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	cohere "github.com/cohere-ai/cohere-go/v2"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
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

	// Remap system messages to user messages due to beta limitations
	messages := []openai.ChatCompletionMessage{}
	for _, message := range m.messages {
		if message.Role == openai.ChatMessageRoleSystem {
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: message.Content,
			})
		} else {
			messages = append(messages, message)
		}
	}

	req := openai.ChatCompletionRequest{
		Model:    mod.Name,
		Messages: messages,
		Stream:   true,
		User:     cfg.User,
	}

	if mod.API != "perplexity" || !strings.Contains(mod.Name, "online") {
		req.Temperature = noOmitFloat(cfg.Temperature)
		req.TopP = noOmitFloat(cfg.TopP)
		req.Stop = cfg.Stop
		req.MaxTokens = cfg.MaxTokens
		req.ResponseFormat = responseFormat(cfg)
	}

	// Add MCP tools if enabled and we're using OpenAI API
	enabledServers, err := enabledMCPServers()
	if err != nil {
		return m.handleRequestError(err, mod, content)
	}
	if len(enabledServers) > 0 {
		// Get all available tools from enabled MCP servers
		tools, err := getMCPToolsForOpenAI(enabledServers)
		if err == nil && len(tools) > 0 {
			req.Tools = tools
			req.ToolChoice = "auto"
		}
	}

	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return m.handleRequestError(err, mod, content)
	}

	return m.receiveCompletionStreamCmd(completionOutput{stream: stream})()
}

// getMCPToolsForOpenAI converts MCP tools to OpenAI tools format
func getMCPToolsForOpenAI(enabledServers []string) ([]openai.Tool, error) {
	configuredServers, _, err := configuredMCPServers()
	if err != nil {
		return nil, err
	}

	var openaiTools []openai.Tool

	for _, serverName := range enabledServers {
		serverConfig, exists := configuredServers[serverName]
		if !exists {
			continue
		}

		mcpClient, err := client.NewStdioMCPClient(serverConfig.Command, nil, serverConfig.Args...)
		if err != nil {
			continue
		}

		_, err = mcpClient.Initialize(context.Background(), mcp.InitializeRequest{})
		if err != nil {
			mcpClient.Close()
			continue
		}

		tools, err := mcpClient.ListTools(context.Background(), mcp.ListToolsRequest{})
		if err != nil {
			mcpClient.Close()
			continue
		}

		for _, tool := range tools.Tools {
			// Create a function definition
			functionDef := &openai.FunctionDefinition{
				Name:        fmt.Sprintf("%s__%s", serverName, tool.Name),
				Description: tool.Description,
			}

			// Convert the MCP tool schema to OpenAI parameters format
			if tool.RawInputSchema != nil {
				functionDef.Parameters = json.RawMessage(tool.RawInputSchema)
			} else {
				// Convert structured schema
				params := map[string]interface{}{
					"type": "object",
				}
				if len(tool.InputSchema.Properties) > 0 {
					params["properties"] = tool.InputSchema.Properties
				}
				if len(tool.InputSchema.Required) > 0 {
					params["required"] = tool.InputSchema.Required
				}

				paramsJSON, err := json.Marshal(params)
				if err != nil {
					continue
				}
				functionDef.Parameters = json.RawMessage(paramsJSON)
			}

			// Create the tool with the function definition
			openaiTools = append(openaiTools, openai.Tool{
				Type:     openai.ToolTypeFunction,
				Function: functionDef,
			})
		}

		mcpClient.Close()
	}

	return openaiTools, nil
}

func (m *Mods) createOllamaStream(content string, occfg OllamaClientConfig, mod Model) tea.Msg {
	cfg := m.Config

	// Add warning if MCP is configured but we're using Ollama
	if len(cfg.UseMCPServers) > 0 || cfg.UseAllMCPServers || len(cfg.DefaultMCPServers) > 0 {
		if !m.Config.Quiet {
			fmt.Fprintln(os.Stderr, m.Styles.Timeago.Render("Warning: MCP tools are only supported with OpenAI API models"))
		}
	}

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

func (m *Mods) createGoogleStream(content string, gccfg GoogleClientConfig, mod Model) tea.Msg {
	cfg := m.Config

	// Add warning if MCP is configured but we're using Google
	if len(cfg.UseMCPServers) > 0 || cfg.UseAllMCPServers || len(cfg.DefaultMCPServers) > 0 {
		if !m.Config.Quiet {
			fmt.Fprintln(os.Stderr, m.Styles.Timeago.Render("Warning: MCP tools are only supported with OpenAI API models"))
		}
	}

	client := NewGoogleClientWithConfig(gccfg)
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelRequest = cancel

	if err := m.setupStreamContext(content, mod); err != nil {
		return err
	}

	// Google doesn't support the System role so we need to remove those message
	// and, instead, store their content on the `System` request value.
	//
	// Also, the shape of Google messages is slightly different, so we make the
	// conversion here.
	messages := []GoogleContent{}

	for _, message := range m.messages {
		if message.Role == openai.ChatMessageRoleSystem {
			parts := []GoogleParts{
				{Text: fmt.Sprintf("%s\n", message.Content)},
			}
			messages = append(messages, GoogleContent{
				Role:  "user",
				Parts: parts,
			})
		} else {
			role := "user"
			if message.Role == openai.ChatMessageRoleAssistant {
				role = "model"
			}
			parts := []GoogleParts{
				{Text: message.Content},
			}
			messages = append(messages, GoogleContent{
				Role:  role,
				Parts: parts,
			})
		}
	}

	generationConfig := GoogleGenerationConfig{
		StopSequences:  cfg.Stop,
		Temperature:    cfg.Temperature,
		TopP:           cfg.TopP,
		TopK:           cfg.TopK,
		CandidateCount: 1,
	}

	if cfg.MaxTokens > 0 {
		generationConfig.MaxOutputTokens = uint(cfg.MaxTokens) //nolint: gosec
	} else {
		generationConfig.MaxOutputTokens = 4096
	}

	req := GoogleMessageCompletionRequest{
		Contents:         messages,
		GenerationConfig: generationConfig,
	}

	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return m.handleRequestError(err, mod, content)
	}

	return m.receiveCompletionStreamCmd(completionOutput{stream: stream})()
}

func (m *Mods) createAnthropicStream(content string, accfg AnthropicClientConfig, mod Model) tea.Msg {
	cfg := m.Config

	// Add warning if MCP is configured but we're using Anthropic
	if len(cfg.UseMCPServers) > 0 || cfg.UseAllMCPServers || len(cfg.DefaultMCPServers) > 0 {
		if !m.Config.Quiet {
			fmt.Fprintln(os.Stderr, m.Styles.Timeago.Render("Warning: MCP tools are only supported with OpenAI API models"))
		}
	}

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
		TopK:          cfg.TopK,
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

func (m *Mods) createCohereStream(content string, cccfg CohereClientConfig, mod Model) tea.Msg {
	cfg := m.Config

	// Add warning if MCP is configured but we're using Cohere
	if len(cfg.UseMCPServers) > 0 || cfg.UseAllMCPServers || len(cfg.DefaultMCPServers) > 0 {
		if !m.Config.Quiet {
			fmt.Fprintln(os.Stderr, m.Styles.Timeago.Render("Warning: MCP tools are only supported with OpenAI API models"))
		}
	}

	client := NewCohereClientWithConfig(cccfg)
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelRequest = cancel

	if err := m.setupStreamContext(content, mod); err != nil {
		return err
	}

	var messages []*cohere.Message
	for _, message := range m.messages {
		switch message.Role {
		case openai.ChatMessageRoleSystem:
			// For system, it is recommended to use the `preamble` field
			// rather than a "SYSTEM" role message
			m.system += message.Content + "\n"
		case openai.ChatMessageRoleAssistant:
			messages = append(messages, &cohere.Message{
				Role: "CHATBOT",
				Chatbot: &cohere.ChatMessage{
					Message: message.Content,
				},
			})
		case openai.ChatMessageRoleUser:
			messages = append(messages, &cohere.Message{
				Role: "USER",
				User: &cohere.ChatMessage{
					Message: message.Content,
				},
			})
		}
	}

	var history []*cohere.Message
	if len(messages) > 1 {
		history = messages[:len(messages)-1]
	}

	req := &cohere.ChatStreamRequest{
		Model:         cohere.String(mod.Name),
		ChatHistory:   history,
		Message:       messages[len(messages)-1].User.Message,
		Preamble:      cohere.String(m.system),
		Temperature:   cohere.Float64(float64(cfg.Temperature)),
		P:             cohere.Float64(float64(cfg.TopP)),
		StopSequences: cfg.Stop,
	}

	if cfg.MaxTokens > 0 {
		req.MaxTokens = cohere.Int(cfg.MaxTokens)
	}

	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return m.handleRequestError(CohereToOpenAIAPIError(err), mod, content)
	}

	return m.receiveCompletionStreamCmd(completionOutput{stream: stream})()
}

func (m *Mods) setupStreamContext(content string, mod Model) error {
	cfg := m.Config
	m.messages = []openai.ChatCompletionMessage{}
	if txt := cfg.FormatText[cfg.FormatAs]; cfg.Format && txt != "" {
		m.messages = append(m.messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: txt,
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
