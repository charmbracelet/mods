package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/mods/proto"
)

func (m *Mods) createOllamaStream(content string, occfg OllamaClientConfig, mod Model) tea.Msg {
	panic("todo")
	// cfg := m.Config
	//
	// client := NewOllamaClientWithConfig(occfg)
	// ctx, cancel := context.WithCancel(context.Background())
	// m.cancelRequest = cancel
	//
	// if err := m.setupStreamContext(content, mod); err != nil {
	// 	return err
	// }
	//
	// req := OllamaMessageCompletionRequest{
	// 	Model:    mod.Name,
	// 	Messages: m.messages,
	// 	Stream:   true,
	// 	Options: OllamaMessageCompletionRequestOptions{
	// 		Temperature: noOmitFloat(cfg.Temperature),
	// 		TopP:        noOmitFloat(cfg.TopP),
	// 	},
	// }
	//
	// if len(cfg.Stop) > 0 {
	// 	req.Options.Stop = cfg.Stop[0]
	// }
	//
	// if cfg.MaxTokens > 0 {
	// 	req.Options.NumCtx = cfg.MaxTokens
	// }
	//
	// stream, err := client.CreateChatCompletionStream(ctx, req)
	// if err != nil {
	// 	return m.handleRequestError(err, mod, content)
	// }
	//
	// return m.receiveCompletionStreamCmd(completionOutput{stream: stream})()
}

func (m *Mods) createGoogleStream(content string, gccfg GoogleClientConfig, mod Model) tea.Msg {
	panic("todo")
	// cfg := m.Config
	//
	// client := NewGoogleClientWithConfig(gccfg)
	// ctx, cancel := context.WithCancel(context.Background())
	// m.cancelRequest = cancel
	//
	// if err := m.setupStreamContext(content, mod); err != nil {
	// 	return err
	// }
	//
	// // Google doesn't support the System role so we need to remove those message
	// // and, instead, store their content on the `System` request value.
	// //
	// // Also, the shape of Google messages is slightly different, so we make the
	// // conversion here.
	// messages := []GoogleContent{}
	//
	// for _, message := range m.messages {
	// 	if message.Role == "system" {
	// 		parts := []GoogleParts{
	// 			{Text: fmt.Sprintf("%s\n", message.Content)},
	// 		}
	// 		messages = append(messages, GoogleContent{
	// 			Role:  roleUser,
	// 			Parts: parts,
	// 		})
	// 	} else {
	// 		role := roleUser
	// 		if message.Role == roleAssistant {
	// 			role = "model"
	// 		}
	// 		parts := []GoogleParts{
	// 			{Text: message.Content},
	// 		}
	// 		messages = append(messages, GoogleContent{
	// 			Role:  role,
	// 			Parts: parts,
	// 		})
	// 	}
	// }
	//
	// generationConfig := GoogleGenerationConfig{
	// 	StopSequences:  cfg.Stop,
	// 	Temperature:    cfg.Temperature,
	// 	TopP:           cfg.TopP,
	// 	TopK:           cfg.TopK,
	// 	CandidateCount: 1,
	// }
	//
	// if cfg.MaxTokens > 0 {
	// 	generationConfig.MaxOutputTokens = uint(cfg.MaxTokens)
	// } else {
	// 	generationConfig.MaxOutputTokens = 4096
	// }
	//
	// req := GoogleMessageCompletionRequest{
	// 	Contents:         messages,
	// 	GenerationConfig: generationConfig,
	// }
	//
	// stream, err := client.CreateChatCompletionStream(ctx, req)
	// if err != nil {
	// 	return m.handleRequestError(err, mod, content)
	// }
	//
	// return m.receiveCompletionStreamCmd(completionOutput{stream: stream})()
}

func (m *Mods) createCohereStream(content string, cccfg CohereClientConfig, mod Model) tea.Msg {
	panic("todo")
	// cfg := m.Config
	//
	// client := NewCohereClientWithConfig(cccfg)
	// ctx, cancel := context.WithCancel(context.Background())
	// m.cancelRequest = cancel
	//
	// if err := m.setupStreamContext(content, mod); err != nil {
	// 	return err
	// }
	//
	// var messages []*cohere.Message
	// for _, message := range m.messages {
	// 	switch message.Role {
	// 	case "system":
	// 		// For system, it is recommended to use the `preamble` field
	// 		// rather than a "SYSTEM" role message
	// 		m.system += message.Content + "\n"
	// 	case "assistant":
	// 		messages = append(messages, &cohere.Message{
	// 			Role: "CHATBOT",
	// 			Chatbot: &cohere.ChatMessage{
	// 				Message: message.Content,
	// 			},
	// 		})
	// 	case "user":
	// 		messages = append(messages, &cohere.Message{
	// 			Role: "USER",
	// 			User: &cohere.ChatMessage{
	// 				Message: message.Content,
	// 			},
	// 		})
	// 	}
	// }
	//
	// var history []*cohere.Message
	// if len(messages) > 1 {
	// 	history = messages[:len(messages)-1]
	// }
	//
	// req := &cohere.ChatStreamRequest{
	// 	Model:         cohere.String(mod.Name),
	// 	ChatHistory:   history,
	// 	Message:       messages[len(messages)-1].User.Message,
	// 	Preamble:      cohere.String(m.system),
	// 	Temperature:   cohere.Float64(cfg.Temperature),
	// 	P:             cohere.Float64(cfg.TopP),
	// 	StopSequences: cfg.Stop,
	// }
	//
	// if cfg.MaxTokens > 0 {
	// 	req.MaxTokens = cohere.Int(int(cfg.MaxTokens))
	// }
	//
	// stream, err := client.CreateChatCompletionStream(ctx, req)
	// if err != nil {
	// 	return m.handleRequestError(CohereToOpenAIAPIError(err), mod, content)
	// }
	//
	// return m.receiveCompletionStreamCmd(completionOutput{stream: stream})()
}

func (m *Mods) setupStreamContext(content string, mod Model) error {
	cfg := m.Config
	m.messages = []proto.Message{}
	if txt := cfg.FormatText[cfg.FormatAs]; cfg.Format && txt != "" {
		m.messages = append(m.messages, proto.Message{
			Role:    proto.RoleSystem,
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
			m.messages = append(m.messages, proto.Message{
				Role:    proto.RoleSystem,
				Content: content,
			})
		}
	}

	if prefix := cfg.Prefix; prefix != "" {
		content = strings.TrimSpace(prefix + "\n\n" + content)
	}

	if !cfg.NoLimit && int64(len(content)) > mod.MaxChars {
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

	m.messages = append(m.messages, proto.Message{
		Role:    proto.RoleUser,
		Content: content,
	})

	return nil
}
