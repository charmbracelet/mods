package main

import (
	"strings"

	flag "github.com/spf13/pflag"
)

type config struct {
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
	MaxRetries        int
	Fanciness         uint
	Prefix            string
	Version           bool
}

func newConfig() config {
	var c config
	flag.StringVarP(&c.Model, "model", "m", "gpt-4", "OpenAI model (gpt-3.5-turbo, gpt-4).")
	flag.BoolVarP(&c.Markdown, "format", "f", false, "Format response as markdown.")
	flag.IntVarP(&c.IncludePrompt, "prompt", "P", 0, "Include the prompt from the arguments and stdin, truncate stdin to specified number of lines.")
	flag.BoolVarP(&c.IncludePromptArgs, "prompt-args", "p", false, "Include the prompt from the arguments in the response.")
	flag.BoolVarP(&c.Quiet, "quiet", "q", false, "Quiet mode (hide the spinner while loading).")
	flag.BoolVarP(&c.ShowHelp, "help", "h", false, "show help and exit.")
	flag.BoolVarP(&c.Version, "version", "v", false, "Show version")
	flag.IntVar(&c.MaxRetries, "max-retries", 5, "Maximum number of times to retry API calls.") //nolint:gomnd
	flag.BoolVar(&c.NoLimit, "no-limit", false, "Turn off the client-side limit on the size of the input into the model.")
	flag.IntVar(&c.MaxTokens, "max", 0, "Maximum number of tokens in response.")
	flag.Float32Var(&c.Temperature, "temp", 1.0, "Temperature (randomness) of results, from 0.0 to 2.0.")
	flag.Float32Var(&c.TopP, "top", 1.0, "TopP, an alternative to temperature that narrows response, from 0.0 to 1.0.")
	flag.UintVar(&c.Fanciness, "fanciness", 10, "Number of cycling characters in the 'generating' animation.")
	flag.Lookup("prompt").NoOptDefVal = "-1"
	flag.Parse()
	c.Prefix = strings.Join(flag.Args(), " ")
	return c
}
