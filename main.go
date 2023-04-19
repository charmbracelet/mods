package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	openai "github.com/sashabaranov/go-openai"
	flag "github.com/spf13/pflag"
)

var errorStyle = errRenderer.NewStyle().Foreground(lipgloss.Color("1"))
var codeStyle = errRenderer.NewStyle().Foreground(lipgloss.Color("1")).Background(lipgloss.Color("237")).Padding(0, 1)
var codeCommentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
var linkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Underline(true)
var helpAppStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
var helpFlagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#41ffef")).Bold(true)
var helpDescriptionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

type config struct {
	Model       *string
	Markdown    *bool
	Quiet       *bool
	MaxTokens   *int
	Temperature *float32
	TopP        *float32
	ShowHelp    *bool
}

func newConfig() config {
	return config{
		Model:       flag.StringP("model", "m", "gpt-4", "OpenAI model (gpt-3.5-turbo, gpt-4)."),
		Markdown:    flag.BoolP("format", "f", false, "Format response as markdown."),
		Quiet:       flag.BoolP("quiet", "q", false, "Quiet mode (hide the spinner while loading)."),
		ShowHelp:    flag.BoolP("help", "h", false, "show help and exit."),
		MaxTokens:   flag.Int("max", 0, "Maximum number of tokens in response."),
		Temperature: flag.Float32("temp", 1.0, "Temperature (randomness) of results, from 0.0 to 2.0."),
		TopP:        flag.Float32("top", 1.0, "TopP, an alternative to temperature that narrows response, from 0.0 to 1.0."),
	}
}

func readStdinContent() string {
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		reader := bufio.NewReader(os.Stdin)
		stdinBytes, err := io.ReadAll(reader)
		if err != nil {
			handleError(prettyError{err, "Unable to read stdin."})
		}
		return string(stdinBytes)
	}
	return ""
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

func usage() {
	fmt.Printf("GPT on the command line. Built for pipelines.\n\n")
	fmt.Printf("Usage:\n  %s [OPTIONS] [PREFIX TERM]\n\n", helpAppStyle.Render(os.Args[0]))
	fmt.Println("Options:")
	flag.VisitAll(func(f *flag.Flag) {
		if f.Shorthand == "" {
			fmt.Printf(
				"  %-38s %s\n",
				helpFlagStyle.Render("--"+f.Name),
				helpDescriptionStyle.Render(f.Usage),
			)
		} else {
			fmt.Printf(
				"  %s, %-34s %s\n",
				helpFlagStyle.Render("-"+f.Shorthand),
				helpFlagStyle.Render("--"+f.Name),
				helpDescriptionStyle.Render(f.Usage),
			)
		}
	})
	desc, example := randomExample()
	fmt.Printf(
		"\nExample:\n  %s\n  %s\n",
		codeCommentStyle.Render("# "+desc),
		example,
	)
}

var errEmptyKey = prettyError{
	err:    fmt.Errorf("You can grab one at %s", linkStyle.Render("https://platform.openai.com/account/api-keys.")),
	reason: codeStyle.Render("OPENAI_API_KEY") + errorStyle.Render(" environment variabled is required."),
}

func createClient(apiKey string) *openai.Client {
	if apiKey == "" {
		handleError(errEmptyKey)
	}
	return openai.NewClient(apiKey)
}

func startChatCompletion(ctx context.Context, client openai.Client, cfg config, content string) (string, error) {
	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:       *cfg.Model,
			Temperature: noOmitFloat(*cfg.Temperature),
			TopP:        noOmitFloat(*cfg.TopP),
			MaxTokens:   *cfg.MaxTokens,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: content,
				},
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("Chat completion: %w", err)
	}

	return resp.Choices[0].Message.Content, nil
}

// prettyError is a wrapper around an error that adds a reason and a pretty
// error message using lipgloss.
type prettyError struct {
	err    error
	reason string
}

func (e prettyError) Error() string {
	var sb strings.Builder
	fmt.Fprintln(&sb)
	fmt.Fprintln(&sb, errorStyle.Render("  Error:", e.reason))
	fmt.Fprintln(&sb)
	fmt.Fprintln(&sb, "  "+errorStyle.Render(e.err.Error()))
	fmt.Fprintln(&sb)
	return sb.String()
}

// handleError prints an error to stderr and exits with a non-zero exit code.
func handleError(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func main() {
	flag.Usage = usage
	flag.CommandLine.SortFlags = false
	config := newConfig()
	flag.Parse()
	client := createClient(os.Getenv("OPENAI_API_KEY"))
	content := readStdinContent()
	prefix := strings.Join(flag.Args(), " ")
	if prefix == "" && content == "" {
		flag.Usage()
		os.Exit(0)
	}
	if *config.Markdown {
		prefix = fmt.Sprintf("%s Format output as Markdown.", prefix)
	}
	if prefix != "" {
		content = strings.TrimSpace(prefix + "\n\n" + content)
	}

	var p *tea.Program
	outc := make(chan string, 1)
	done := make(chan struct{}, 1)
	errc := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize program
	if !*config.Quiet {
		spinner := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(spinnerStyle))
		p = tea.NewProgram(Model{spinner: spinner}, tea.WithOutput(errRenderer.Output()))
	}

	// Always run chat completion in a goroutine and wait for it to finish. We
	// can quit the spinner after the chat returns.
	//
	// Don't use os.Exit or handleError here. Error handling is done with errc.
	go func() {
		defer func() {
			if !*config.Quiet {
				p.Quit()
			}
		}()

		output, err := startChatCompletion(ctx, *client, config, content)
		if err != nil {
			errc <- prettyError{err: err, reason: "There was a problem with the OpenAI API."}
			return
		}

		outc <- output
	}()

	if !*config.Quiet {
		go func() {
			// Ensure the program runs and finishes before we exit.
			_, err := p.Run()
			if err != nil {
				errc <- prettyError{err: err, reason: "Can't run the Bubble Tea program."}
				return
			}

			done <- struct{}{}
		}()
	}

	select {
	case output := <-outc:
		// Wait for the program to finish.
		// TODO: use bubbletea program.Wait() from #722
		if !*config.Quiet {
			<-done
		}
		// Everything went well, print the output.
		fmt.Println(output)
	case <-done:
		// Stop OpenAI if it's still running.
		cancel()
		os.Exit(1)
	case err := <-errc:
		if err != nil {
			cancel()
			// Found error, print it and exit.
			handleError(err)
		}
	}
}
