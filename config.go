package main

import flag "github.com/spf13/pflag"

type config struct {
	Model       *string
	Markdown    *bool
	Quiet       *bool
	MaxTokens   *int
	Temperature *float32
	TopP        *float32
	ShowHelp    *bool
	Version     *bool
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
		Version:     flag.BoolP("version", "v", false, "Show version"),
	}
}
