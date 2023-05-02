package main

import (
	"strings"

	flag "github.com/spf13/pflag"
)

type config struct {
	SimpleSpinner     bool
	Model             string
	Markdown          bool
	Quiet             bool
	MaxTokens         int
	Temperature       float32
	TopP              float32
	ShowHelp          bool
	NoLimit           bool
	IncludePromptArgs bool
	IncludePrompt     int
	Prefix            string
	Version           bool
}

func newConfig() config {
	model := flag.StringP("model", "m", "gpt-4", "OpenAI model (gpt-3.5-turbo, gpt-4).")
	markdown := flag.BoolP("format", "f", false, "Format response as markdown.")
	includePrompt := flag.IntP("prompt", "P", 0, "Include the prompt from the arguments and stdin, truncate stdin to specified number of lines.")
	includePromptArgs := flag.BoolP("prompt-args", "p", false, "Include the prompt from the arguments in the response.")
	quiet := flag.BoolP("quiet", "q", false, "Quiet mode (hide the spinner while loading).")
	simpleSpinner := flag.BoolP("spinner", "s", false, "Use simple spinner.")
	showHelp := flag.BoolP("help", "h", false, "show help and exit.")
	version := flag.BoolP("version", "v", false, "Show version")
	noLimit := flag.Bool("no-limit", false, "Turn off the client-side limit on the size of the input into the model.")
	maxTokens := flag.Int("max", 0, "Maximum number of tokens in response.")
	temperature := flag.Float32("temp", 1.0, "Temperature (randomness) of results, from 0.0 to 2.0.")
	topP := flag.Float32("top", 1.0, "TopP, an alternative to temperature that narrows response, from 0.0 to 1.0.")
	flag.Lookup("prompt").NoOptDefVal = "-1"
	flag.Parse()
	prefix := strings.Join(flag.Args(), " ")
	return config{
		Model:             *model,
		Markdown:          *markdown,
		Quiet:             *quiet,
		ShowHelp:          *showHelp,
		NoLimit:           *noLimit,
		MaxTokens:         *maxTokens,
		Temperature:       *temperature,
		TopP:              *topP,
		Prefix:            prefix,
		IncludePrompt:     *includePrompt,
		IncludePromptArgs: *includePromptArgs,
		Version:           *version,
		SimpleSpinner:     *simpleSpinner,
	}
}
