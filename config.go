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

const (
	defaultMarkdownFormatText = "Format the response as markdown without enclosing backticks."
	defaultJSONFormatText     = "Format the response as json without enclosing backticks."
)

var help = map[string]string{
	"api":               "OpenAI compatible REST API (openai, localai, anthropic, ...)",
	"apis":              "Aliases and endpoints for OpenAI compatible REST API",
	"http-proxy":        "HTTP proxy to use for API requests",
	"model":             "Default model (gpt-3.5-turbo, gpt-4, ggml-gpt4all-j...)",
	"ask-model":         "Ask which model to use via interactive prompt",
	"max-input-chars":   "Default character limit on input to model",
	"format":            "Ask for the response to be formatted as markdown unless otherwise set",
	"format-text":       "Text to append when using the -f flag",
	"role":              "System role to use",
	"roles":             "List of predefined system messages that can be used as roles",
	"list-roles":        "List the roles defined in your configuration file",
	"prompt":            "Include the prompt from the arguments and stdin, truncate stdin to specified number of lines",
	"prompt-args":       "Include the prompt from the arguments in the response",
	"raw":               "Render output as raw text when connected to a TTY",
	"quiet":             "Quiet mode (hide the spinner while loading and stderr messages for success)",
	"help":              "Show help and exit",
	"version":           "Show version and exit",
	"max-retries":       "Maximum number of times to retry API calls",
	"no-limit":          "Turn off the client-side limit on the size of the input into the model",
	"word-wrap":         "Wrap formatted output at specific width (default is 80)",
	"max-tokens":        "Maximum number of tokens in response",
	"temp":              "Temperature (randomness) of results, from 0.0 to 2.0, -1.0 to disable",
	"stop":              "Up to 4 sequences where the API will stop generating further tokens",
	"topp":              "TopP, an alternative to temperature that narrows response, from 0.0 to 1.0, -1.0 to disable",
	"topk":              "TopK, only sample from the top K options for each subsequent token, -1 to disable",
	"fanciness":         "Your desired level of fanciness",
	"status-text":       "Text to show while generating",
	"settings":          "Open settings in your $EDITOR",
	"dirs":              "Print the directories in which mods store its data",
	"reset-settings":    "Backup your old settings file and reset everything to the defaults",
	"continue":          "Continue from the last response or a given save title",
	"continue-last":     "Continue from the last response",
	"no-cache":          "Disables caching of the prompt/response",
	"title":             "Saves the current conversation with the given title",
	"list":              "Lists saved conversations",
	"delete":            "Deletes one or more saved conversations with the given titles or IDs",
	"delete-older-than": "Deletes all saved conversations older than the specified duration; valid values are " + strings.EnglishJoin(duration.ValidUnits(), true),
	"show":              "Show a saved conversation with the given title or ID",
	"theme":             "Theme to use in the forms; valid choices are charm, catppuccin, dracula, and base16",
	"show-last":         "Show the last saved conversation",
	"editor":            "Edit the prompt in your $EDITOR; only taken into account if no other args and if STDIN is a TTY",
	"mcp-servers":       "MCP Servers configurations",
	"mcp-disable":       "Disable specific MCP servers",
	"mcp-list":          "List all available MCP servers",
	"mcp-list-tools":    "List all available tools from enabled MCP servers",
	"mcp-timeout":       "Timeout for MCP server calls, defaults to 15 seconds",
}

// Model represents the LLM model used in the API call.
type Model struct {
	Name           string
	API            string
	MaxChars       int64    `yaml:"max-input-chars"`
	Aliases        []string `yaml:"aliases"`
	Fallback       string   `yaml:"fallback"`
	ThinkingBudget int      `yaml:"thinking-budget,omitempty"`
}

// API represents an API endpoint and its models.
type API struct {
	Name      string
	APIKey    string           `yaml:"api-key"`
	APIKeyEnv string           `yaml:"api-key-env"`
	APIKeyCmd string           `yaml:"api-key-cmd"`
	Version   string           `yaml:"version"` // XXX: not used anywhere
	BaseURL   string           `yaml:"base-url"`
	Models    map[string]Model `yaml:"models"`
	User      string           `yaml:"user"`
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

// FormatText is a map[format]formatting_text.
type FormatText map[string]string

// UnmarshalYAML conforms with yaml.Unmarshaler.
func (ft *FormatText) UnmarshalYAML(unmarshal func(any) error) error {
	var text string
	if err := unmarshal(&text); err != nil {
		var formats map[string]string
		if err := unmarshal(&formats); err != nil {
			return err
		}
		*ft = (FormatText)(formats)
		return nil
	}

	*ft = map[string]string{
		"markdown": text,
	}
	return nil
}

// Config holds the main configuration and is mapped to the YAML settings file.
type Config struct {
	API                 string     `yaml:"default-api" env:"API"`
	Model               string     `yaml:"default-model" env:"MODEL"`
	Format              bool       `yaml:"format" env:"FORMAT"`
	FormatText          FormatText `yaml:"format-text"`
	FormatAs            string     `yaml:"format-as" env:"FORMAT_AS"`
	Raw                 bool       `yaml:"raw" env:"RAW"`
	Quiet               bool       `yaml:"quiet" env:"QUIET"`
	MaxTokens           int64      `yaml:"max-tokens" env:"MAX_TOKENS"`
	MaxCompletionTokens int64      `yaml:"max-completion-tokens" env:"MAX_COMPLETION_TOKENS"`
	MaxInputChars       int64      `yaml:"max-input-chars" env:"MAX_INPUT_CHARS"`
	Temperature         float64    `yaml:"temp" env:"TEMP"`
	Stop                []string   `yaml:"stop" env:"STOP"`
	TopP                float64    `yaml:"topp" env:"TOPP"`
	TopK                int64      `yaml:"topk" env:"TOPK"`
	NoLimit             bool       `yaml:"no-limit" env:"NO_LIMIT"`
	CachePath           string     `yaml:"cache-path" env:"CACHE_PATH"`
	NoCache             bool       `yaml:"no-cache" env:"NO_CACHE"`
	IncludePromptArgs   bool       `yaml:"include-prompt-args" env:"INCLUDE_PROMPT_ARGS"`
	IncludePrompt       int        `yaml:"include-prompt" env:"INCLUDE_PROMPT"`
	MaxRetries          int        `yaml:"max-retries" env:"MAX_RETRIES"`
	WordWrap            int        `yaml:"word-wrap" env:"WORD_WRAP"`
	Fanciness           uint       `yaml:"fanciness" env:"FANCINESS"`
	StatusText          string     `yaml:"status-text" env:"STATUS_TEXT"`
	HTTPProxy           string     `yaml:"http-proxy" env:"HTTP_PROXY"`
	APIs                APIs       `yaml:"apis"`
	System              string     `yaml:"system"`
	Role                string     `yaml:"role" env:"ROLE"`
	AskModel            bool
	Roles               map[string][]string
	ShowHelp            bool
	ResetSettings       bool
	Prefix              string
	Version             bool
	Settings            bool
	Dirs                bool
	Theme               string
	SettingsPath        string
	ContinueLast        bool
	Continue            string
	Title               string
	ShowLast            bool
	Show                string
	List                bool
	ListRoles           bool
	Delete              []string
	DeleteOlderThan     time.Duration
	User                string

	MCPServers   map[string]MCPServerConfig `yaml:"mcp-servers"`
	MCPList      bool
	MCPListTools bool
	MCPDisable   []string
	MCPTimeout   time.Duration `yaml:"mcp-timeout" env:"MCP_TIMEOUT"`

	openEditor                                         bool
	cacheReadFromID, cacheWriteToID, cacheWriteToTitle string
}

// MCPServerConfig holds configuration for an MCP server.
type MCPServerConfig struct {
	Command string   `yaml:"command"`
	Env     []string `yaml:"env"`
	Args    []string `yaml:"args"`
}

func ensureConfig() (Config, error) {
	var c Config
	sp, err := xdg.ConfigFile(filepath.Join("mods", "mods.yml"))
	if err != nil {
		return c, modsError{err, "Could not find settings path."}
	}
	c.SettingsPath = sp

	dir := filepath.Dir(sp)
	if dirErr := os.MkdirAll(dir, 0o700); dirErr != nil { //nolint:mnd
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

	if err := env.ParseWithOptions(&c, env.Options{Prefix: "MODS_"}); err != nil {
		return c, modsError{err, "Could not parse environment into settings file."}
	}

	if c.CachePath == "" {
		c.CachePath = filepath.Join(xdg.DataHome, "mods")
	}

	if err := os.MkdirAll(
		filepath.Join(c.CachePath, "conversations"),
		0o700,
	); err != nil { //nolint:mnd
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

	f, err := os.Create(path)
	if err != nil {
		return modsError{err, "Could not create configuration file."}
	}
	defer func() { _ = f.Close() }()

	m := struct {
		Config Config
		Help   map[string]string
	}{
		Config: defaultConfig(),
		Help:   help,
	}
	if err := tmpl.Execute(f, m); err != nil {
		return modsError{err, "Could not render template."}
	}
	return nil
}

func defaultConfig() Config {
	return Config{
		FormatAs: "markdown",
		FormatText: FormatText{
			"markdown": defaultMarkdownFormatText,
			"json":     defaultJSONFormatText,
		},
		MCPTimeout: 15 * time.Second,
	}
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
	fmt.Printf(
		"Usage:\n  %s\n\n",
		useLine(),
	)
	fmt.Println("Options:")
	cmd.Flags().VisitAll(func(f *flag.Flag) {
		if f.Hidden {
			return
		}
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
	if cmd.HasExample() {
		fmt.Printf(
			"\nExample:\n  %s\n  %s\n",
			stdoutStyles().Comment.Render("# "+cmd.Example),
			cheapHighlighting(stdoutStyles(), examples[cmd.Example]),
		)
	}

	return nil
}
