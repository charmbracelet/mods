package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/caarlos0/env/v8"
	"github.com/charmbracelet/lipgloss"
	gap "github.com/muesli/go-app-paths"
	"github.com/muesli/termenv"
	flag "github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

const configTemplate = `
# {{ index .Help "apis" }}
apis:
  openai: https://api.openai.com/v1
  localai: http://localhost:8080
# {{ index .Help "model" }}
default-model: gpt-4
# {{ index .Help "models" }}
models:
  gpt-4:
    aliases: ["4"]
    max-input-chars: 24500
    api: openai
    fallback: gpt-3.5-turbo
  gpt-4-32k:
    aliases: ["32k"]
    max-input-chars: 98000
    api: openai
    fallback: gpt-4
  gpt-3.5-turbo:
    aliases: ["35t"]
    max-input-chars: 12250
    api: openai
    fallback: gpt-3.5
  gpt-3.5:
    aliases: ["35"]
    max-input-chars: 12250
    api: openai
    fallback:
  ggml-gpt4all-j:
    aliases: ["local", "4all"]
    max-input-chars: 12250
    api: localai
    fallback:
# {{ index .Help "max-input-chars" }}
max-input-chars: 12250
# {{ index .Help "format" }}
format: false
# {{ index .Help "quiet" }}
quiet: false
# {{ index .Help "temp" }}
temp: 1.0
# {{ index .Help "topp" }}
topp: 1.0
# {{ index .Help "no-limit" }}
no-limit: false
# {{ index .Help "prompt-args" }}
include-prompt-args: false
# {{ index .Help "prompt" }}
include-prompt: 0
# {{ index .Help "max-retries" }}
max-retries: 5
# {{ index .Help "fanciness" }}
fanciness: 10
# {{ index .Help "status-text" }}
status-text: Generating
# {{ index .Help "max-tokens" }}
# max-tokens: 100
`

type config struct {
	API               string            `yaml:"api" env:"API"`
	APIs              map[string]string `yaml:"apis"`
	Model             string            `yaml:"default-model" env:"MODEL"`
	Models            map[string]Model  `yaml:"models"`
	Markdown          bool              `yaml:"format" env:"FORMAT"`
	Quiet             bool              `yaml:"quiet" env:"QUIET"`
	MaxTokens         int               `yaml:"max-tokens" env:"MAX_TOKENS"`
	MaxInputChars     int               `yaml:"max-input-chars" env:"MAX_INPUT_CHARS"`
	Temperature       float32           `yaml:"temp" env:"TEMP"`
	TopP              float32           `yaml:"topp" env:"TOPP"`
	NoLimit           bool              `yaml:"no-limit" env:"NO_LIMIT"`
	IncludePromptArgs bool              `yaml:"include-prompt-args" env:"INCLUDE_PROMPT_ARGS"`
	IncludePrompt     int               `yaml:"include-prompt" env:"INCLUDE_PROMPT"`
	MaxRetries        int               `yaml:"max-retries" env:"MAX_RETRIES"`
	Fanciness         uint              `yaml:"fanciness" env:"FANCINESS"`
	StatusText        string            `yaml:"status-text" env:"STATUS_TEXT"`
	ShowHelp          bool
	Prefix            string
	Version           bool
	Settings          bool
	SettingsPath      string
}

func newConfig() (config, error) {
	var c config
	var content []byte

	help := map[string]string{
		"api":             "Default OpenAI compatible REST API (openai, localai).",
		"apis":            "Aliases and endpoints for OpenAI compatible REST API.",
		"model":           "Default model (gpt-3.5-turbo, gpt-4, ggml-gpt4all-j...).",
		"models":          "Model details and aliases.",
		"max-input-chars": "Default character limit on input to model.",
		"format":          "Format response as markdown.",
		"prompt":          "Include the prompt from the arguments and stdin, truncate stdin to specified number of lines.",
		"prompt-args":     "Include the prompt from the arguments in the response.",
		"quiet":           "Quiet mode (hide the spinner while loading).",
		"help":            "Show help and exit.",
		"version":         "Show version and exit.",
		"max-retries":     "Maximum number of times to retry API calls.",
		"no-limit":        "Turn off the client-side limit on the size of the input into the model.",
		"max-tokens":      "Maximum number of tokens in response.",
		"temp":            "Temperature (randomness) of results, from 0.0 to 2.0.",
		"topp":            "TopP, an alternative to temperature that narrows response, from 0.0 to 1.0.",
		"fanciness":       "Number of cycling characters in the 'generating' animation.",
		"status-text":     "Text to show while generating.",
		"settings":        "Open settings in your $EDITOR.",
	}

	scope := gap.NewScope(gap.User, "mods")
	sp, err := scope.ConfigPath("mods.yml")
	if err != nil {
		return c, err
	}
	c.SettingsPath = sp
	if _, err := os.Stat(sp); os.IsNotExist(err) {
		tmpl, err := template.New("config").Parse(strings.TrimSpace(configTemplate))
		if err != nil {
			return c, err
		}
		if err := os.MkdirAll(filepath.Dir(sp), 0o700); err != nil {
			return c, err
		}

		f, err := os.Create(sp)
		if err != nil {
			return c, err
		}
		defer func() { _ = f.Close() }()

		m := struct {
			Config config
			Help   map[string]string
		}{
			Config: c,
			Help:   help,
		}
		if err := tmpl.Execute(f, m); err != nil {
			return c, err
		}
	} else if err != nil {
		return c, err
	}
	content, err = os.ReadFile(sp)
	if err != nil {
		return c, err
	}
	err = yaml.Unmarshal(content, &c)
	if err != nil {
		return c, err
	}

	// Set model aliases
	ms := make(map[string]Model)
	for k, m := range c.Models {
		m.Name = k
		ms[k] = m
		for _, am := range m.Aliases {
			ms[am] = m
		}
	}
	c.Models = ms

	err = env.ParseWithOptions(&c, env.Options{Prefix: "MODS_"})
	if err != nil {
		return c, err
	}

	flag.StringVarP(&c.Model, "model", "m", c.Model, help["model"])
	flag.StringVarP(&c.API, "api", "a", c.API, help["api"])
	flag.BoolVarP(&c.Markdown, "format", "f", c.Markdown, help["format"])
	flag.IntVarP(&c.IncludePrompt, "prompt", "P", c.IncludePrompt, help["prompt"])
	flag.BoolVarP(&c.IncludePromptArgs, "prompt-args", "p", c.IncludePromptArgs, help["prompt-args"])
	flag.BoolVarP(&c.Quiet, "quiet", "q", c.Quiet, help["quiet"])
	flag.BoolVarP(&c.Settings, "settings", "s", false, help["settings"])
	flag.BoolVarP(&c.ShowHelp, "help", "h", false, help["help"])
	flag.BoolVarP(&c.Version, "version", "v", false, help["version"])
	flag.IntVar(&c.MaxRetries, "max-retries", c.MaxRetries, help["max-retries"])
	flag.BoolVar(&c.NoLimit, "no-limit", c.NoLimit, help["no-limit"])
	flag.IntVar(&c.MaxTokens, "max-tokens", c.MaxTokens, help["max-tokens"])
	flag.Float32Var(&c.Temperature, "temp", c.Temperature, help["temp"])
	flag.Float32Var(&c.TopP, "topp", c.TopP, help["topp"])
	flag.UintVar(&c.Fanciness, "fanciness", c.Fanciness, help["fanciness"])
	flag.StringVar(&c.StatusText, "status-text", c.StatusText, help["status-text"])
	flag.Lookup("prompt").NoOptDefVal = "-1"
	flag.Usage = usage
	flag.CommandLine.SortFlags = false
	flag.Parse()
	c.Prefix = strings.Join(flag.Args(), " ")

	return c, nil
}

func usage() {
	r := lipgloss.DefaultRenderer()
	s := makeStyles(r)
	appName := filepath.Base(os.Args[0])

	if r.ColorProfile() == termenv.TrueColor {
		appName = makeGradientText(s.appName, appName)
	}

	fmt.Printf("GPT on the command line. Built for pipelines.\n\n")
	fmt.Printf(
		"Usage:\n  %s %s\n\n",
		appName,
		s.cliArgs.Render("[OPTIONS] [PREFIX TERM]"),
	)
	fmt.Println("Options:")
	flag.VisitAll(func(f *flag.Flag) {
		if f.Shorthand == "" {
			fmt.Printf(
				"  %-42s %s\n",
				s.flag.Render("--"+f.Name),
				s.flagDesc.Render(f.Usage),
			)
		} else {
			fmt.Printf(
				"  %s%s %-38s %s\n",
				s.flag.Render("-"+f.Shorthand),
				s.flagComma,
				s.flag.Render("--"+f.Name),
				s.flagDesc.Render(f.Usage),
			)
		}
	})
	desc, example := randomExample()
	fmt.Printf(
		"\nExample:\n  %s\n  %s\n",
		s.comment.Render("# "+desc),
		cheapHighlighting(s, example),
	)
}
