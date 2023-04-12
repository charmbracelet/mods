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
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
	"github.com/pkg/errors"
	openai "github.com/sashabaranov/go-openai"
)

const (
	modelTypeFlagShorthand  = "t"
	markdownFlagShorthand   = "m"
	quietFlagShorthand      = "q"
	typeFlagDescription     = "OpenAI model type (gpt-3.5-turbo, gpt-4)."
	markdownFlagDescription = "Format response as markdown."
	quietFlagDescription    = "Quiet mode (hide the spinner while loading)."
)

var errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
var codeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Background(lipgloss.Color("0")).Padding(0, 1)
var linkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Underline(true)

func printUsage() {
	lipgloss.SetColorProfile(termenv.ColorProfile())
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
	fmt.Printf("  %s\t%s\n", flagStyle.Render("-"+modelTypeFlagShorthand), descriptionStyle.Render(typeFlagDescription))
	fmt.Printf("  %s\t%s\n", flagStyle.Render("-"+markdownFlagShorthand), descriptionStyle.Render(markdownFlagDescription))
	fmt.Printf("  %s\t%s\n", flagStyle.Render("-"+quietFlagShorthand), descriptionStyle.Render(quietFlagDescription))
}

func readStdinContent() string {
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		reader := bufio.NewReader(os.Stdin)
		stdinBytes, err := io.ReadAll(reader)
		if err != nil {
			fmt.Println()
			fmt.Println(errorStyle.Render("  Unable to read stdin."))
			fmt.Println()
			fmt.Println("  " + errorStyle.Render(err.Error()))
			fmt.Println()
			os.Exit(1)
		}
		return string(stdinBytes)
	}
	return ""
}

func createClient(apiKey string) *openai.Client {
	if apiKey == "" {
		fmt.Println()
		fmt.Println(errorStyle.Render("  Error: ") + codeStyle.Render("OPENAI_API_KEY") + errorStyle.Render(" environment variabled is required."))
		fmt.Println()
		fmt.Println(errorStyle.Render("  You can grab one at ") + linkStyle.Render("https://platform.openai.com/account/api-keys."))
		fmt.Println()
		os.Exit(1)
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
		return "", errors.Wrap(err, "Chat completion error")
	}
	return resp.Choices[0].Message.Content, nil
}

func main() {
	modelTypeFlag := flag.String(modelTypeFlagShorthand, "gpt-4", typeFlagDescription)
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
		lipgloss.SetColorProfile(termenv.NewOutput(os.Stderr).ColorProfile())
		spinner := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(spinnerStyle))
		p = tea.NewProgram(Model{spinner: spinner}, tea.WithOutput(os.Stderr))
	}

	if !*quietFlag {
		go func() {
			output, err := startChatCompletion(*client, *modelTypeFlag, content)
			p.Send(quitMsg{})
			if err != nil {
				fmt.Println()
				fmt.Println(errorStyle.Render("  Error: Unable to generate response."))
				fmt.Println()
				fmt.Println("  " + errorStyle.Render(err.Error()))
				fmt.Println()
				os.Exit(1)
			}
			fmt.Println(output)
		}()
	} else {
		output, err := startChatCompletion(*client, *modelTypeFlag, content)
		if err != nil {
			fmt.Println(err.Error())
		}
		fmt.Println(output)
	}

	if !*quietFlag {
		_, _ = p.Run()
	}
}
