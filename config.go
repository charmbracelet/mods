package main

import (
	"strings"

	flag "github.com/spf13/pflag"
)

type config struct {
	Model       string
	Markdown    bool
	Quiet       bool
	AltSpinner  bool
	MaxTokens   int
	Temperature float32
	TopP        float32
	ShowHelp    bool
	Prefix      string
	Version     bool
}

func newConfig() config {
	model := flag.StringP("model", "m", "gpt-4", "OpenAI model (gpt-3.5-turbo, gpt-4).")
	markdown := flag.BoolP("format", "f", false, "Format response as markdown.")
	quiet := flag.BoolP("quiet", "q", false, "Quiet mode (hide the spinner while loading).")
	altSpinner := flag.BoolP("spinner", "s", false, "Use alternate spinner.")
	showHelp := flag.BoolP("help", "h", false, "Show help and exit.")
	maxTokens := flag.Int("max", 0, "Maximum number of tokens in response.")
	temperature := flag.Float32("temp", 1.0, "Temperature (randomness) of results, from 0.0 to 2.0.")
	topP := flag.Float32("top", 1.0, "TopP, an alternative to temperature that narrows response, from 0.0 to 1.0.")
	version := flag.BoolP("version", "v", false, "Show version and exit.")
	flag.Parse()
	prefix := strings.Join(flag.Args(), " ")
	return config{
		Model:       *model,
		Markdown:    *markdown,
		Quiet:       *quiet,
		AltSpinner:  *altSpinner,
		ShowHelp:    *showHelp,
		MaxTokens:   *maxTokens,
		Temperature: *temperature,
		TopP:        *topP,
		Prefix:      prefix,
		Version:     *version,
	}
}
