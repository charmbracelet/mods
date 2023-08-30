package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glow/editor"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
)

// Build vars.
var (
	//nolint: gochecknoglobals
	version = "dev"
	commit  = ""
	date    = ""
	builtBy = ""
)

func buildVersion() string {
	result := version
	if commit != "" {
		result = fmt.Sprintf("%s\ncommit: %s", result, commit)
	}
	if date != "" {
		result = fmt.Sprintf("%s\nbuilt at: %s", result, date)
	}
	if builtBy != "" {
		result = fmt.Sprintf("%s\nbuilt by: %s", result, builtBy)
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Sum != "" {
		result = fmt.Sprintf("%s\nmodule version: %s, checksum: %s", result, info.Main.Version, info.Main.Sum)
	}
	return result
}

func exitError(mods *Mods, err error, reason string) {
	mods.Error = &modsError{reason: reason, err: err}
	fmt.Fprintln(os.Stderr, mods.ErrorView())
	exit(mods, 1)
}

func exit(mods *Mods, status int) {
	if mods != nil && db != nil {
		_ = db.Close()
	}
	os.Exit(status)
}

func init() {
	// XXX: unset error styles in Glamour dark and light styles.
	// On the glamour side, we should probably add constructors for generating
	// default styles so they can be essentially copied and altered without
	// mutating the definitions in Glamour itself (or relying on any deep
	// copying).
	glamour.DarkStyleConfig.CodeBlock.Chroma.Error.BackgroundColor = new(string)
	glamour.LightStyleConfig.CodeBlock.Chroma.Error.BackgroundColor = new(string)

	rootCmd.Version = buildVersion()
	rootCmd.SetUsageFunc(usageFunc)
	rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return flagParseError{err: err}
	})
}

var (
	config Config
	db     *convoDB
	cache  *convoCache

	stdoutRenderer = lipgloss.DefaultRenderer()
	stdoutStyles   = makeStyles(stdoutRenderer)
	stderrRenderer = lipgloss.NewRenderer(os.Stderr, termenv.WithColorCache(true))
	stderrStyles   = makeStyles(stderrRenderer)

	rootCmd = &cobra.Command{
		Use:           "mods",
		SilenceUsage:  true,
		SilenceErrors: true,
		Example: func() string {
			desc, example := randomExample()
			return fmt.Sprintf(
				"  %s\n  %s\n",
				stdoutStyles.Comment.Render("# "+desc),
				cheapHighlighting(stdoutStyles, example),
			)
		}(),
		RunE: func(cmd *cobra.Command, args []string) error {
			config.Prefix = strings.Join(args, " ")

			stdin := cmd.InOrStdin()
			stdout := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()
			opts := []tea.ProgramOption{
				tea.WithOutput(stderrRenderer.Output()),
				tea.WithoutEmptyRenders(),
			}

			if !isInputTTY() {
				opts = append(opts, tea.WithInput(nil))
			}

			mods := newMods(stderrRenderer, &config, db, cache)
			p := tea.NewProgram(mods, opts...)
			m, err := p.Run()
			if err != nil {
				exitError(mods, err, "Couldn't start Bubble Tea program.")
				return modsError{reason: "Couldn't start Bubble Tea program.", err: err}
			}

			mods = m.(*Mods)
			if mods.Error != nil {
				return mods.Error
			}

			if config.Settings {
				c := editor.Cmd(config.SettingsPath)
				c.Stdin = stdin
				c.Stdout = stdout
				c.Stderr = stderr
				if err := c.Run(); err != nil {
					return modsError{reason: "Missing $EDITOR", err: err}
				}

				fmt.Fprintln(stderr, "Wrote config file to:", config.SettingsPath)
				return nil
			}

			if config.ResetSettings {
				resetSettings(mods)
			}

			if config.ShowHelp || (mods.Input == "" &&
				config.Prefix == "" &&
				config.Show == "" &&
				config.Delete == "" &&
				!config.List) {
				return cmd.Usage()
			}

			if config.Show != "" {
				return nil
			}

			if config.List {
				listConversations(mods)
			}

			if config.Delete != "" {
				deleteConversation(mods)
			}

			if config.cacheWriteToID != "" {
				writeConversation(mods)
			}

			return nil
		},
	}
)

func initFlags() {
	flags := rootCmd.Flags()
	flags.StringVarP(&config.Model, "model", "m", config.Model, stdoutStyles.FlagDesc.Render(help["model"]))
	flags.StringVarP(&config.API, "api", "a", config.API, stdoutStyles.FlagDesc.Render(help["api"]))
	flags.StringVarP(&config.HTTPProxy, "http-proxy", "x", config.HTTPProxy, stdoutStyles.FlagDesc.Render(help["http-proxy"]))
	flags.BoolVarP(&config.Format, "format", "f", config.Format, stdoutStyles.FlagDesc.Render(help["format"]))
	flags.BoolVarP(&config.Glamour, "glamour", "g", config.Glamour, stdoutStyles.FlagDesc.Render(help["glamour"]))
	flags.IntVarP(&config.IncludePrompt, "prompt", "P", config.IncludePrompt, stdoutStyles.FlagDesc.Render(help["prompt"]))
	flags.BoolVarP(&config.IncludePromptArgs, "prompt-args", "p", config.IncludePromptArgs, stdoutStyles.FlagDesc.Render(help["prompt-args"]))
	flags.BoolVarP(&config.Quiet, "quiet", "q", config.Quiet, stdoutStyles.FlagDesc.Render(help["quiet"]))
	flags.BoolVar(&config.Settings, "settings", false, stdoutStyles.FlagDesc.Render(help["settings"]))
	flags.BoolVarP(&config.ShowHelp, "help", "h", false, stdoutStyles.FlagDesc.Render(help["help"]))
	flags.BoolVarP(&config.Version, "version", "v", false, stdoutStyles.FlagDesc.Render(help["version"]))
	flags.StringVarP(&config.Continue, "continue", "c", "", stdoutStyles.FlagDesc.Render(help["continue"]))
	flags.BoolVarP(&config.ContinueLast, "continue-last", "C", false, stdoutStyles.FlagDesc.Render(help["continue-last"]))
	flags.BoolVarP(&config.List, "list", "l", config.List, stdoutStyles.FlagDesc.Render(help["list"]))
	flags.IntVar(&config.MaxRetries, "max-retries", config.MaxRetries, stdoutStyles.FlagDesc.Render(help["max-retries"]))
	flags.BoolVar(&config.NoLimit, "no-limit", config.NoLimit, stdoutStyles.FlagDesc.Render(help["no-limit"]))
	flags.IntVar(&config.MaxTokens, "max-tokens", config.MaxTokens, stdoutStyles.FlagDesc.Render(help["max-tokens"]))
	flags.Float32Var(&config.Temperature, "temp", config.Temperature, stdoutStyles.FlagDesc.Render(help["temp"]))
	flags.Float32Var(&config.TopP, "topp", config.TopP, stdoutStyles.FlagDesc.Render(help["topp"]))
	flags.UintVar(&config.Fanciness, "fanciness", config.Fanciness, stdoutStyles.FlagDesc.Render(help["fanciness"]))
	flags.StringVar(&config.StatusText, "status-text", config.StatusText, stdoutStyles.FlagDesc.Render(help["status-text"]))
	flags.BoolVar(&config.ResetSettings, "reset-settings", config.ResetSettings, stdoutStyles.FlagDesc.Render(help["reset-settings"]))
	flags.StringVarP(&config.Title, "title", "t", config.Title, stdoutStyles.FlagDesc.Render(help["title"]))
	flags.StringVar(&config.Delete, "delete", config.Delete, stdoutStyles.FlagDesc.Render(help["delete"]))
	flags.StringVarP(&config.Show, "show", "s", config.Show, stdoutStyles.FlagDesc.Render(help["show"]))

	for _, name := range []string{"show", "delete", "continue"} {
		_ = rootCmd.RegisterFlagCompletionFunc(name, func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			results, _ := db.Completions(toComplete)
			return results, cobra.ShellCompDirectiveDefault
		})
	}

	flags.BoolVar(&config.NoCache, "no-cache", config.NoCache, stdoutStyles.FlagDesc.Render(help["no-cache"]))
	flags.Lookup("prompt").NoOptDefVal = "-1"
	flags.SortFlags = false
	// flags.Init("", flag.ContinueOnError)

	if config.Format && config.FormatText == "" {
		config.FormatText = "Format the response as markdown without enclosing backticks."
	}
}

func main() {
	var err error
	config, err = newConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "There was an error loading your config file.\n")
		os.Exit(1)
	}

	cache = newCache(config.CachePath)
	db, err = openDB(filepath.Join(config.CachePath, "mods.db"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open db: %s\n", err)
		os.Exit(1)
	}

	defer db.Close() //nolint:errcheck

	// XXX: this must come after creating the config.
	initFlags()

	// XXX: since mods doesn't have any subcommands, Cobra won't create the
	// default `completion` command.
	// Forcefully create the completion related subcommands by adding a fake
	// command when completions are being used.
	if os.Getenv("__MODS_CMP_ENABLED") == "1" || (len(os.Args) > 1 && os.Args[1] == "__complete") {
		rootCmd.AddCommand(&cobra.Command{Use: "____fake_command_to_enable_completions"})
		rootCmd.InitDefaultCompletionCmd()
	}

	if err := rootCmd.Execute(); err != nil {
		const horizontalEdgePadding = 2
		s := stderrRenderer.NewStyle().Padding(0, horizontalEdgePadding)
		format := "%s\n\n"

		var args []interface{}
		if ferr, ok := err.(flagParseError); ok {
			format += "%s\n\n"
			args = []interface{}{
				fmt.Sprintf("Check out %s %s", stderrStyles.InlineCode.Render("mods -h"), stderrStyles.Comment.Render("for help.")),
				fmt.Sprintf("Missing flag: %s", stderrStyles.InlineCode.Render(ferr.Flag())),
			}
		} else if merr, ok := err.(modsError); ok {
			format += "%s\n\n"
			args = []interface{}{
				s.Render(stderrStyles.ErrorHeader.String(), merr.reason),
				s.Render(stderrStyles.ErrorDetails.Render(err.Error())),
			}
		} else {
			args = []interface{}{
				s.Render(stderrStyles.ErrorDetails.Render(err.Error())),
			}
		}

		fmt.Fprintf(os.Stderr,
			format,
			args...,
		)

		os.Exit(1)
	}
}

func resetSettings(mods *Mods) {
	_, err := os.Stat(config.SettingsPath)
	if err != nil {
		exitError(mods, err, "Couldn't read config file.")
	}
	inputFile, err := os.Open(config.SettingsPath)
	if err != nil {
		exitError(mods, err, "Couldn't open config file.")
	}
	defer inputFile.Close() //nolint:errcheck
	outputFile, err := os.Create(config.SettingsPath + ".bak")
	if err != nil {
		exitError(mods, err, "Couldn't backup config file.")
	}
	defer outputFile.Close() //nolint:errcheck
	_, err = io.Copy(outputFile, inputFile)
	if err != nil {
		exitError(mods, err, "Couldn't write config file.")
	}
	// The copy was successful, so now delete the original file
	err = os.Remove(config.SettingsPath)
	if err != nil {
		exitError(mods, err, "Couldn't remove config file.")
	}
	err = writeConfigFile(config.SettingsPath)
	if err != nil {
		exitError(mods, err, "Couldn't write new config file.")
	}
	fmt.Fprintln(os.Stderr, "\nSettings restored to defaults!")
	fmt.Fprintf(os.Stderr,
		"\n  %s %s\n\n",
		mods.Styles.Comment.Render("Your old settings have been saved to:"),
		mods.Styles.Link.Render(config.SettingsPath+".bak"),
	)
	exit(mods, 0)
}

func deleteConversation(mods *Mods) {
	convo, err := db.Find(config.Delete)
	if err != nil {
		exitError(mods, err, "Couldn't delete conversation.")
	}

	if err := db.Delete(convo.ID); err != nil {
		exitError(mods, err, "Couldn't delete conversation.")
	}

	if err := cache.delete(convo.ID); err != nil {
		exitError(mods, err, "Couldn't delete conversation.")
	}

	fmt.Fprintln(os.Stderr, "Conversation deleted:", convo.ID[:sha1minLen])
	exit(mods, 0)
}

func listConversations(mods *Mods) {
	styles := makeStyles(stdoutRenderer)
	conversations, err := db.List()
	if err != nil {
		exitError(mods, err, "Couldn't list saves.")
	}

	if len(conversations) == 0 {
		fmt.Fprintln(os.Stderr, "No conversations found.")
		exit(mods, 0)
	}

	fmt.Fprintf(
		os.Stderr,
		"Saved conversations %s:\n",
		mods.Styles.Comment.Render(
			"("+fmt.Sprint(len(conversations))+")",
		),
	)
	format := "%s %s\n"
	if isOutputTTY() {
		format = styles.Comment.Render("•") + " %s %s\n"
	}
	for _, conversation := range conversations {
		fmt.Fprintf(os.Stdout, format,
			conversation.ID[:sha1short],
			styles.Comment.Render(conversation.Title),
		)
	}

	exit(mods, 0)
}

func writeConversation(mods *Mods) {
	// if message is a sha1, use the last prompt instead.
	id := config.cacheWriteToID
	title := strings.TrimSpace(config.cacheWriteToTitle)

	if sha1reg.MatchString(title) || title == "" {
		title = firstLine(lastPrompt(mods.messages))
	}

	// conversation would already have been written to the file storage, just
	// need to write to db too.
	if err := db.Save(id, title); err != nil {
		exitError(mods, err, "Couldn't save conversation.")
	}

	fmt.Fprintln(
		os.Stderr,
		"\nConversation saved:",
		config.cacheWriteToID[:sha1short],
		stderrStyles.Comment.Render(config.cacheWriteToTitle),
	)
}
