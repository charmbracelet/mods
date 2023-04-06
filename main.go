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
	fmt.Printf("  %s  %s\n", flagStyle.Render("-m"), descriptionStyle.Render("OpenAI model flag (gpt-3.5-turbo, gpt-4)"))
	fmt.Printf("  %s  %s\n", flagStyle.Render("-f"), descriptionStyle.Render("Ask GPT to format the output as Markdown"))
	fmt.Printf("  %s  %s\n", flagStyle.Render("-o"), descriptionStyle.Render("Output file to save response. If not specified, prints to console"))
	fmt.Printf("  %s  %s\n", flagStyle.Render("-no-spinner"), descriptionStyle.Render("Whether to show the spinner while loading"))
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

func writeOutput(output, fileName string) {
	file, err := os.Create(fileName)
	if err != nil {
		log.Fatal("Error creating output file: ", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	_, err = writer.WriteString(output)
	if err != nil {
		log.Fatalf("Error writing to output file: %s", err)
	}
	writer.Flush()
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
	modelVersionFlag := flag.String("m", "gpt-4", "OpenAI model flag (gpt-4, gpt-3.5-turbo).")
	formatFlag := flag.Bool("f", false, "Ask GPT to format the output as Markdown.")
	outputFileFlag := flag.String("o", "", "Output file to save response. If not specified, prints to console.")
	hideSpinnerFlag := flag.Bool("no-spinner", false, "Whether to show the spinner while loading.")
	flag.Usage = printUsage
	flag.Parse()

	client := createClient(os.Getenv("OPENAI_API_KEY"))
	content := readStdinContent()
	prefix := strings.Join(flag.Args(), " ")
	if prefix == "" && content == "" {
		printUsage()
		os.Exit(0)
	}
	if *formatFlag {
		prefix = fmt.Sprintf("%s Format output as Markdown.", prefix)
	}

	if prefix != "" {
		content = strings.TrimSpace(prefix + "\n\n" + content)
	}

	var p *tea.Program
	if !*hideSpinnerFlag {
		spinner := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(spinnerStyle))
		// TODO: use termenv output instead of os.Stderr (modelRenderer.Output())
		p = tea.NewProgram(Model{spinner: spinner}, tea.WithOutput(os.Stderr))
	}

	errc := make(chan error, 1)
	go func() {
		defer func() {
			if !*hideSpinnerFlag {
				p.Send(quitMsg{})
			}
		}()
		output, err := startChatCompletion(*client, *modelVersionFlag, content)
		if err != nil {
			errc <- fmt.Errorf("ChatCompletion error: %s", err)
			return
		}
		if *outputFileFlag != "" {
			writeOutput(output, *outputFileFlag)
		} else {
			fmt.Println(output)
		}

		errc <- nil
	}()

	if !*hideSpinnerFlag {
		_, err := p.Run()
		if err != nil {
			log.Fatalf("Bubbletea error: %s", err)
		}
	}

	if err := <-errc; err != nil {
		log.Fatal(err)
	}
}
