package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/mattn/go-isatty"
	openai "github.com/sashabaranov/go-openai"
)

const (
	typeFlagShorthand       = "t"
	markdownFlagShorthand   = "m"
	quietFlagShorthand      = "q"
	typeFlagDescription     = "OpenAI model type (gpt-3.5-turbo, gpt-4)."
	markdownFlagDescription = "Format response as markdown."
	quietFlagDescription    = "Quiet mode (hide loading spinner)."
)

func printUsage() {
	appNameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Bold(true)
	flagStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#41ffef")).
		Bold(true)
	descriptionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244"))

	fmt.Printf("Usage: %s [OPTIONS] [PREFIX TERM]\n", appNameStyle.Render(os.Args[0]))
	fmt.Println()
	fmt.Println("Options:")
	fmt.Printf("  %s\t%s\n", flagStyle.Render("-"+typeFlagShorthand), descriptionStyle.Render(typeFlagDescription))
	fmt.Printf("  %s\t%s\n", flagStyle.Render("-"+markdownFlagShorthand), descriptionStyle.Render(markdownFlagDescription))
	fmt.Printf("  %s\t%s\n", flagStyle.Render("-"+quietFlagShorthand), descriptionStyle.Render(quietFlagDescription))
}

func readStdinContent() string {
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		reader := bufio.NewReader(os.Stdin)
		stdinBytes, err := io.ReadAll(reader)
		if err != nil {
			log.Fatal("Error reading standard input: ", err)
		}
		return string(stdinBytes)
	}
	return ""
}

func createClient(apiKey string) *openai.Client {
	if apiKey == "" {
		log.Fatal("Error: OPENAI_API_KEY environment variable is required. You can grab one at https://platform.openai.com/account/api-keys.")
	}
	return openai.NewClient(apiKey)
}

func startChatCompletion(client openai.Client, modelVersion string, content string) (string, error) {
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: modelVersion,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: content,
				},
			},
		},
	)
	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}

func main() {
	typeFlag := flag.String(typeFlagShorthand, "gpt-4", typeFlagDescription)
	markdownFlag := flag.Bool(markdownFlagShorthand, false, markdownFlagDescription)
	quietFlag := flag.Bool(quietFlagShorthand, false, quietFlagDescription)
	flag.Usage = printUsage
	flag.Parse()

	client := createClient(os.Getenv("OPENAI_API_KEY"))
	content := readStdinContent()
	prefix := strings.Join(flag.Args(), " ")
	if prefix == "" && content == "" {
		printUsage()
		os.Exit(0)
	}
	if *markdownFlag {
		prefix = fmt.Sprintf("%s Format output as Markdown.", prefix)
	}

	if prefix != "" {
		content = strings.TrimSpace(prefix + "\n\n" + content)
	}

	var p *tea.Program
	if !*quietFlag {
		spinner := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(spinnerStyle))
		p = tea.NewProgram(Model{spinner: spinner}, tea.WithOutput(os.Stderr))
	}

	var output string
	errc := make(chan error, 1)
	go func() {
		defer func() {
			if !*quietFlag {
				p.Send(quitMsg{})
			}
		}()

		var err error
		output, err = startChatCompletion(*client, *typeFlag, content)
		if err != nil {
			errc <- fmt.Errorf("ChatCompletion error: %s", err)
			return
		}

		errc <- nil
	}()

	if !*quietFlag {
		_, err := p.Run()
		if err != nil {
			log.Fatalf("Bubble Tea error: %s", err)
		}
	}

	if err := <-errc; err != nil {
		log.Fatal(err)
	}

	fmt.Println(output)
}
