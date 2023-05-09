package main

import (
	"strings"

	"github.com/caarlos0/env/v8"
	flag "github.com/spf13/pflag"
)

type config struct {
	Model             string  `env:"MODEL" envDefault:"gpt-4"`
	Markdown          bool    `env:"FORMAT"`
	Quiet             bool    `env:"QUIET"`
	MaxTokens         int     `env:"MAX_TOKENS"`
	Temperature       float32 `env:"TEMP" envDefault:"1.0"`
	TopP              float32 `env:"TOPP" envDefault:"1.0"`
	ShowHelp          bool
	NoLimit           bool `env:"NO_LIMIT"`
	IncludePromptArgs bool `env:"INCLUDE_PROMPT_ARGS"`
	IncludePrompt     int  `env:"INCLUDE_PROMPT"`
	MaxRetries        int  `env:"MAX_RETRIES" envDefault:"5"`
	Fanciness         uint `env:"FANCINESS" envDefault:"10"`
	Prefix            string
	Version           bool
}

func newConfig() (config, error) {
	var c config
	err := env.ParseWithOptions(&c, env.Options{Prefix: "MODS_"})
	if err != nil {
		return c, err
	}

	// Parse flags, overriding any values set in the environment.
	flag.StringVarP(&c.Model, "model", "m", c.Model, "OpenAI model (gpt-3.5-turbo, gpt-4).")
	flag.BoolVarP(&c.Markdown, "format", "f", c.Markdown, "Format response as markdown.")
	flag.IntVarP(&c.IncludePrompt, "prompt", "P", c.IncludePrompt, "Include the prompt from the arguments and stdin, truncate stdin to specified number of lines.")
	flag.BoolVarP(&c.IncludePromptArgs, "prompt-args", "p", c.IncludePromptArgs, "Include the prompt from the arguments in the response.")
	flag.BoolVarP(&c.Quiet, "quiet", "q", c.Quiet, "Quiet mode (hide the spinner while loading).")
	flag.BoolVarP(&c.ShowHelp, "help", "h", false, "show help and exit.")
	flag.BoolVarP(&c.Version, "version", "v", false, "Show version")
	flag.IntVar(&c.MaxRetries, "max-retries", 5, "Maximum number of times to retry API calls.") //nolint:gomnd
	flag.BoolVar(&c.NoLimit, "no-limit", c.NoLimit, "Turn off the client-side limit on the size of the input into the model.")
	flag.IntVar(&c.MaxTokens, "max-tokens", c.MaxTokens, "Maximum number of tokens in response.")
	flag.Float32Var(&c.Temperature, "temp", c.Temperature, "Temperature (randomness) of results, from 0.0 to 2.0.")
	flag.Float32Var(&c.TopP, "topp", c.TopP, "TopP, an alternative to temperature that narrows response, from 0.0 to 1.0.")
	flag.UintVar(&c.Fanciness, "fanciness", c.Fanciness, "Number of cycling characters in the 'generating' animation.") //nolint:gomnd
	flag.Lookup("prompt").NoOptDefVal = "-1"
	flag.Parse()
	c.Prefix = strings.Join(flag.Args(), " ")

	return c, nil
}
