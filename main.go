package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"math"
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
	modelTypeFlagShorthand     = "m"
	quietFlagShorthand         = "q"
	markdownFlagShorthand      = "md"
	temperatureFlagShorthand   = "temp"
	maxTokensFlagShorthand     = "max"
	topPFlagShorthand          = "top"
	typeFlagDescription        = "OpenAI model (gpt-3.5-turbo, gpt-4)."
	markdownFlagDescription    = "Format response as markdown."
	quietFlagDescription       = "Quiet mode (hide the spinner while loading)."
	temperatureFlagDescription = "Temperature (randomness) of results, from 0.0 to 2.0."
	maxTokensFlagDescription   = "Maximum number of tokens in response."
	topPFlagDescription        = "TopP, an alternative to temperature that narrows response, from 0.0 to 1.0."
)

var errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
var codeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Background(lipgloss.Color("0")).Padding(0, 1)
var linkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Underline(true)

type Config struct {
	Model       string
	Markdown    bool
	Quiet       bool
	MaxTokens   int
	Temperature float32
	TopP        float32
}

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
	fmt.Printf("  %s\t%s\n", flagStyle.Render("-"+quietFlagShorthand), descriptionStyle.Render(quietFlagDescription))
	fmt.Printf("  %s\t%s\n", flagStyle.Render("-"+markdownFlagShorthand), descriptionStyle.Render(markdownFlagDescription))
	fmt.Printf("  %s\t%s\n", flagStyle.Render("-"+temperatureFlagShorthand), descriptionStyle.Render(temperatureFlagDescription))
	fmt.Printf("  %s\t%s\n", flagStyle.Render("-"+topPFlagShorthand), descriptionStyle.Render(topPFlagDescription))
	fmt.Printf("  %s\t%s\n", flagStyle.Render("-"+maxTokensFlagShorthand), descriptionStyle.Render(maxTokensFlagDescription))
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

// flagToFloat converts a flag value to a float usable by the OpenAI client
// library, which currently uses Float32 fields in the request struct with the
// omitempty tag. This means we need to use math.SmallestNonzeroFloat32 instead
// of 0.0 so it doesn't get stripped from the request and replaced server side
// with the default values.
// Issue: https://github.com/sashabaranov/go-openai/issues/9
func flagToFloat(f *float64) float32 {
	if *f == 0.0 {
		return math.SmallestNonzeroFloat32
	}
	return float32(*f)
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

func startChatCompletion(client openai.Client, config Config, content string) (string, error) {
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       config.Model,
			Temperature: config.Temperature,
			TopP:        config.TopP,
			MaxTokens:   config.MaxTokens,
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

func configFromFlags() Config {
	modelTypeFlag := flag.String(modelTypeFlagShorthand, "gpt-4", typeFlagDescription)
	markdownFlag := flag.Bool(markdownFlagShorthand, false, markdownFlagDescription)
	quietFlag := flag.Bool(quietFlagShorthand, false, quietFlagDescription)
	temperatureFlag := flag.Float64(temperatureFlagShorthand, 1.0, temperatureFlagDescription)
	maxTokenFlag := flag.Int(maxTokensFlagShorthand, 0, maxTokensFlagDescription)
	topPFlag := flag.Float64(topPFlagShorthand, 1.0, topPFlagDescription)
	flag.Usage = printUsage
	flag.Parse()
	return Config{
		Model:       *modelTypeFlag,
		Quiet:       *quietFlag,
		MaxTokens:   *maxTokenFlag,
		Markdown:    *markdownFlag,
		Temperature: flagToFloat(temperatureFlag),
		TopP:        flagToFloat(topPFlag),
	}
}

func main() {
	config := configFromFlags()
	client := createClient(os.Getenv("OPENAI_API_KEY"))
	content := readStdinContent()
	prefix := strings.Join(flag.Args(), " ")
	if prefix == "" && content == "" {
		printUsage()
		os.Exit(0)
	}
	if config.Markdown {
		prefix = fmt.Sprintf("%s Format output as Markdown.", prefix)
	}
	if prefix != "" {
		content = strings.TrimSpace(prefix + "\n\n" + content)
	}

	var p *tea.Program
	if !config.Quiet {
		lipgloss.SetColorProfile(termenv.NewOutput(os.Stderr).ColorProfile())
		spinner := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(spinnerStyle))
		p = tea.NewProgram(Model{spinner: spinner}, tea.WithOutput(os.Stderr))
	}

	if !config.Quiet {
		go func() {
			output, err := startChatCompletion(*client, config, content)
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
		output, err := startChatCompletion(*client, config, content)
		if err != nil {
			fmt.Println(err.Error())
		}
		fmt.Println(output)
	}

	if !config.Quiet {
		_, _ = p.Run()
	}
}
