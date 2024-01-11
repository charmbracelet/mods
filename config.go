package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	_ "embed"

	"github.com/adrg/xdg"
	"github.com/caarlos0/duration"
	"github.com/caarlos0/env/v9"
	"github.com/charmbracelet/x/exp/strings"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

//go:embed config_template.yml
var configTemplate string

var help = map[string]string{
	"api":               "OpenAI compatible REST API (openai, localai).",
	"apis":              "Aliases and endpoints for OpenAI compatible REST API.",
	"http-proxy":        "HTTP proxy to use for API requests.",
	"model":             "Default model (gpt-3.5-turbo, gpt-4, ggml-gpt4all-j...).",
	"max-input-chars":   "Default character limit on input to model.",
	"format":            "Ask for the response to be formatted as markdown unless otherwise set.",
	"format-text":       "Text to append when using the -f flag.",
	"prompt":            "Include the prompt from the arguments and stdin, truncate stdin to specified number of lines.",
	"prompt-args":       "Include the prompt from the arguments in the response.",
	"raw":               "Render output as raw text when connected to a TTY.",
	"quiet":             "Quiet mode (hide the spinner while loading and stderr messages for success).",
	"help":              "Show help and exit.",
	"version":           "Show version and exit.",
	"max-retries":       "Maximum number of times to retry API calls.",
	"no-limit":          "Turn off the client-side limit on the size of the input into the model.",
	"word-wrap":         "Wrap formatted output at specific width (default is 80)",
	"max-tokens":        "Maximum number of tokens in response.",
	"temp":              "Temperature (randomness) of results, from 0.0 to 2.0.",
	"topp":              "TopP, an alternative to temperature that narrows response, from 0.0 to 1.0.",
	"fanciness":         "Your desired level of fanciness.",
	"status-text":       "Text to show while generating.",
	"settings":          "Open settings in your $EDITOR.",
	"dirs":              "Print the directories in which mods store its data",
	"reset-settings":    "Backup your old settings file and reset everything to the defaults.",
	"continue":          "Continue from the last response or a given save title.",
	"continue-last":     "Continue from the last response.",
	"no-cache":          "Disables caching of the prompt/response.",
	"title":             "Saves the current conversation with the given title.",
	"list":              "Lists saved conversations.",
	"delete":            "Deletes a saved conversation with the given title or ID.",
	"delete-older-than": "Deletes all saved conversations older than the specified duration. Valid units are: " + strings.EnglishJoin(duration.ValidUnits(), true) + ".",
	"show":              "Show a saved conversation with the given title or ID.",
	"show-last":         "Show the last saved conversation.",
}

// Model represents the LLM model used in the API call.
type Model struct {
	Name     string
	API      string
	MaxChars int      `yaml:"max-input-chars"`
	Aliases  []string `yaml:"aliases"`
	Fallback string   `yaml:"fallback"`
}

// API represents an API endpoint and its models.
type API struct {
	Name      string
	APIKey    string           `yaml:"api-key"`
	APIKeyEnv string           `yaml:"api-key-env"`
	BaseURL   string           `yaml:"base-url"`
	Models    map[string]Model `yaml:"models"`
}

// APIs is a type alias to allow custom YAML decoding.
type APIs []API

// UnmarshalYAML implements sorted API YAML decoding.
func (apis *APIs) UnmarshalYAML(node *yaml.Node) error {
	for i := 0; i < len(node.Content); i += 2 {
		var api API
		if err := node.Content[i+1].Decode(&api); err != nil {
			return fmt.Errorf("error decoding YAML file: %s", err)
		}
		api.Name = node.Content[i].Value
		*apis = append(*apis, api)
	}
	return nil
}

// Config holds the main configuration and is mapped to the YAML settings file.
type Config struct {
	Model             string  `yaml:"default-model" env:"MODEL"`
	Format            bool    `yaml:"format" env:"FORMAT"`
	Raw               bool    `yaml:"raw" env:"RAW"`
	Quiet             bool    `yaml:"quiet" env:"QUIET"`
	MaxTokens         int     `yaml:"max-tokens" env:"MAX_TOKENS"`
	MaxInputChars     int     `yaml:"max-input-chars" env:"MAX_INPUT_CHARS"`
	Temperature       float32 `yaml:"temp" env:"TEMP"`
	TopP              float32 `yaml:"topp" env:"TOPP"`
	NoLimit           bool    `yaml:"no-limit" env:"NO_LIMIT"`
	CachePath         string  `yaml:"cache-path" env:"CACHE_PATH"`
	NoCache           bool    `yaml:"no-cache" env:"NO_CACHE"`
	IncludePromptArgs bool    `yaml:"include-prompt-args" env:"INCLUDE_PROMPT_ARGS"`
	IncludePrompt     int     `yaml:"include-prompt" env:"INCLUDE_PROMPT"`
	MaxRetries        int     `yaml:"max-retries" env:"MAX_RETRIES"`
	WordWrap          int     `yaml:"word-wrap" env:"WORD_WRAP"`
	Fanciness         uint    `yaml:"fanciness" env:"FANCINESS"`
	StatusText        string  `yaml:"status-text" env:"STATUS_TEXT"`
	FormatText        string  `yaml:"format-text" env:"FORMAT_TEXT"`
	HTTPProxy         string  `yaml:"http-proxy" env:"HTTP_PROXY"`
	APIs              APIs    `yaml:"apis"`
	API               string
	Models            map[string]Model
	ShowHelp          bool
	ResetSettings     bool
	Prefix            string
	Version           bool
	Settings          bool
	Dirs              bool
	SettingsPath      string
	ContinueLast      bool
	Continue          string
	Title             string
	ShowLast          bool
	Show              string
	List              bool
	Delete            string
	DeleteOlderThan   time.Duration

	cacheReadFromID, cacheWriteToID, cacheWriteToTitle string
}

func ensureConfig() (Config, error) {
	var c Config
	sp, err := xdg.ConfigFile(filepath.Join("mods", "mods.yml"))
	if err != nil {
		return c, modsError{err, "Could not find settings path."}
	}
	c.SettingsPath = sp

	dir := filepath.Dir(sp)
	if dirErr := os.MkdirAll(dir, 0o700); dirErr != nil { //nolint:gomnd
		return c, modsError{dirErr, "Could not create cache directory."}
	}

	if dirErr := writeConfigFile(sp); dirErr != nil {
		return c, dirErr
	}
	content, err := os.ReadFile(sp)
	if err != nil {
		return c, modsError{err, "Could not read settings file."}
	}
	if err := yaml.Unmarshal(content, &c); err != nil {
		return c, modsError{err, "Could not parse settings file."}
	}
	ms := make(map[string]Model)
	for _, api := range c.APIs {
		for mk, mv := range api.Models {
			mv.Name = mk
			mv.API = api.Name
			// only set the model key and aliases if they haven't already been used
			_, ok := ms[mk]
			if !ok {
				ms[mk] = mv
			}
			for _, a := range mv.Aliases {
				_, ok := ms[a]
				if !ok {
					ms[a] = mv
				}
			}
		}
	}
	c.Models = ms

	if err := env.ParseWithOptions(&c, env.Options{Prefix: "MODS_"}); err != nil {
		return c, modsError{err, "Could not parse environment into settings file."}
	}

	if c.CachePath == "" {
		c.CachePath = filepath.Join(xdg.DataHome, "mods", "conversations")
	}

	if err := os.MkdirAll(c.CachePath, 0o700); err != nil { //nolint:gomnd
		return c, modsError{err, "Could not create cache directory."}
	}

	if c.WordWrap == 0 {
		c.WordWrap = 80
	}

	return c, nil
}

func writeConfigFile(path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return createConfigFile(path)
	} else if err != nil {
		return modsError{err, "Could not stat path."}
	}
	return nil
}

func createConfigFile(path string) error {
	tmpl := template.Must(template.New("config").Parse(configTemplate))

	var c Config
	f, err := os.Create(path)
	if err != nil {
		return modsError{err, "Could not create configuration file."}
	}
	defer func() { _ = f.Close() }()

	m := struct {
		Config Config
		Help   map[string]string
	}{
		Config: c,
		Help:   help,
	}
	if err := tmpl.Execute(f, m); err != nil {
		return modsError{err, "Could not render template."}
	}
	return nil
}

func useLine() string {
	appName := filepath.Base(os.Args[0])

	if stdoutRenderer().ColorProfile() == termenv.TrueColor {
		appName = makeGradientText(stdoutStyles().AppName, appName)
	}

	return fmt.Sprintf(
		"%s %s",
		appName,
		stdoutStyles().CliArgs.Render("[OPTIONS] [PREFIX TERM]"),
	)
}

func usageFunc(cmd *cobra.Command) error {
	fmt.Printf("GPT on the command line. Built for pipelines.\n\n")
	fmt.Printf(
		"Usage:\n  %s\n\n",
		useLine(),
	)
	fmt.Println("Options:")
	cmd.Flags().VisitAll(func(f *flag.Flag) {
		if f.Shorthand == "" {
			fmt.Printf(
				"  %-44s %s\n",
				stdoutStyles().Flag.Render("--"+f.Name),
				stdoutStyles().FlagDesc.Render(f.Usage),
			)
		} else {
			fmt.Printf(
				"  %s%s %-40s %s\n",
				stdoutStyles().Flag.Render("-"+f.Shorthand),
				stdoutStyles().FlagComma,
				stdoutStyles().Flag.Render("--"+f.Name),
				stdoutStyles().FlagDesc.Render(f.Usage),
			)
		}
	})
	desc, example := randomExample()
	fmt.Printf(
		"\nExample:\n  %s\n  %s\n",
		stdoutStyles().Comment.Render("# "+desc),
		cheapHighlighting(stdoutStyles(), example),
	)

	return nil
}
