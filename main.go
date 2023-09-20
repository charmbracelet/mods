package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	timeago "github.com/caarlos0/timea.go"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glow/editor"
	"github.com/spf13/cobra"
)

// Build vars.
var (
	//nolint: gochecknoglobals
	Version   = ""
	CommitSHA = ""
)

func buildVersion() {
	if len(CommitSHA) >= sha1short {
		vt := rootCmd.VersionTemplate()
		rootCmd.SetVersionTemplate(vt[:len(vt)-1] + " (" + CommitSHA[0:7] + ")\n")
	}
	if Version == "" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Sum != "" {
			Version = info.Main.Version
		} else {
			Version = "unknown (built from source)"
		}
	}
	rootCmd.Version = Version
}

func init() {
	// XXX: unset error styles in Glamour dark and light styles.
	// On the glamour side, we should probably add constructors for generating
	// default styles so they can be essentially copied and altered without
	// mutating the definitions in Glamour itself (or relying on any deep
	// copying).
	glamour.DarkStyleConfig.CodeBlock.Chroma.Error.BackgroundColor = new(string)
	glamour.LightStyleConfig.CodeBlock.Chroma.Error.BackgroundColor = new(string)

	buildVersion()
	rootCmd.SetUsageFunc(usageFunc)
	rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return flagParseError{err: err}
	})

	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
}

var (
	config Config
	db     *convoDB
	cache  *convoCache

	rootCmd = &cobra.Command{
		Use:           "mods",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			config.Prefix = strings.Join(args, " ")

			stdin := cmd.InOrStdin()
			stdout := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()
			opts := []tea.ProgramOption{
				tea.WithOutput(stderrRenderer().Output()),
			}

			if !isInputTTY() {
				opts = append(opts, tea.WithInput(nil))
			}

			mods := newMods(stderrRenderer(), &config, db, cache)
			p := tea.NewProgram(mods, opts...)
			m, err := p.Run()
			if err != nil {
				return modsError{err, "Couldn't start Bubble Tea program."}
			}

			mods = m.(*Mods)
			if mods.Error != nil {
				return *mods.Error
			}

			if config.Settings {
				c := editor.Cmd(config.SettingsPath)
				c.Stdin = stdin
				c.Stdout = stdout
				c.Stderr = stderr
				if err := c.Run(); err != nil {
					return modsError{err, fmt.Sprintf(
						"Missing %s.",
						stderrStyles().InlineCode.Render("$EDITOR"),
					)}
				}

				fmt.Fprintln(stderr, "Wrote config file to:", config.SettingsPath)
				return nil
			}

			if config.ResetSettings {
				return resetSettings()
			}

			if config.ShowHelp || (mods.Input == "" &&
				config.Prefix == "" &&
				config.Show == "" &&
				config.Delete == "" &&
				!config.List) {
				//nolint: wrapcheck
				return cmd.Usage()
			}

			if config.List {
				return listConversations()
			}

			if config.Delete != "" {
				return deleteConversation()
			}

			if isOutputTTY() {
				switch {
				case mods.glamOutput != "":
					fmt.Print(mods.glamOutput)
				case mods.Output != "":
					fmt.Print(mods.Output)
				}
			}

			if config.Show != "" {
				return nil
			}

			if config.cacheWriteToID != "" {
				return saveConversation(mods)
			}

			return nil
		},
	}
)

func initFlags() {
	flags := rootCmd.Flags()
	flags.StringVarP(&config.Model, "model", "m", config.Model, stdoutStyles().FlagDesc.Render(help["model"]))
	flags.StringVarP(&config.API, "api", "a", config.API, stdoutStyles().FlagDesc.Render(help["api"]))
	flags.StringVarP(&config.HTTPProxy, "http-proxy", "x", config.HTTPProxy, stdoutStyles().FlagDesc.Render(help["http-proxy"]))
	flags.BoolVarP(&config.Format, "format", "f", config.Format, stdoutStyles().FlagDesc.Render(help["format"]))
	flags.BoolVarP(&config.Raw, "raw", "r", config.Raw, stdoutStyles().FlagDesc.Render(help["raw"]))
	flags.IntVarP(&config.IncludePrompt, "prompt", "P", config.IncludePrompt, stdoutStyles().FlagDesc.Render(help["prompt"]))
	flags.BoolVarP(&config.IncludePromptArgs, "prompt-args", "p", config.IncludePromptArgs, stdoutStyles().FlagDesc.Render(help["prompt-args"]))
	flags.StringVarP(&config.Continue, "continue", "c", "", stdoutStyles().FlagDesc.Render(help["continue"]))
	flags.BoolVarP(&config.ContinueLast, "continue-last", "C", false, stdoutStyles().FlagDesc.Render(help["continue-last"]))
	flags.BoolVarP(&config.List, "list", "l", config.List, stdoutStyles().FlagDesc.Render(help["list"]))
	flags.StringVarP(&config.Title, "title", "t", config.Title, stdoutStyles().FlagDesc.Render(help["title"]))
	flags.StringVarP(&config.Delete, "delete", "d", config.Delete, stdoutStyles().FlagDesc.Render(help["delete"]))
	flags.StringVarP(&config.Show, "show", "s", config.Show, stdoutStyles().FlagDesc.Render(help["show"]))
	flags.BoolVarP(&config.Quiet, "quiet", "q", config.Quiet, stdoutStyles().FlagDesc.Render(help["quiet"]))
	flags.BoolVarP(&config.ShowHelp, "help", "h", false, stdoutStyles().FlagDesc.Render(help["help"]))
	flags.BoolVarP(&config.Version, "version", "v", false, stdoutStyles().FlagDesc.Render(help["version"]))
	flags.IntVar(&config.MaxRetries, "max-retries", config.MaxRetries, stdoutStyles().FlagDesc.Render(help["max-retries"]))
	flags.BoolVar(&config.NoLimit, "no-limit", config.NoLimit, stdoutStyles().FlagDesc.Render(help["no-limit"]))
	flags.IntVar(&config.MaxTokens, "max-tokens", config.MaxTokens, stdoutStyles().FlagDesc.Render(help["max-tokens"]))
	flags.Float32Var(&config.Temperature, "temp", config.Temperature, stdoutStyles().FlagDesc.Render(help["temp"]))
	flags.Float32Var(&config.TopP, "topp", config.TopP, stdoutStyles().FlagDesc.Render(help["topp"]))
	flags.UintVar(&config.Fanciness, "fanciness", config.Fanciness, stdoutStyles().FlagDesc.Render(help["fanciness"]))
	flags.StringVar(&config.StatusText, "status-text", config.StatusText, stdoutStyles().FlagDesc.Render(help["status-text"]))
	flags.BoolVar(&config.NoCache, "no-cache", config.NoCache, stdoutStyles().FlagDesc.Render(help["no-cache"]))
	flags.BoolVar(&config.ResetSettings, "reset-settings", config.ResetSettings, stdoutStyles().FlagDesc.Render(help["reset-settings"]))
	flags.BoolVar(&config.Settings, "settings", false, stdoutStyles().FlagDesc.Render(help["settings"]))
	flags.Lookup("prompt").NoOptDefVal = "-1"
	flags.SortFlags = false

	for _, name := range []string{"show", "delete", "continue"} {
		_ = rootCmd.RegisterFlagCompletionFunc(name, func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			results, _ := db.Completions(toComplete)
			return results, cobra.ShellCompDirectiveDefault
		})
	}

	if config.Format && config.FormatText == "" {
		config.FormatText = "Format the response as markdown without enclosing backticks."
	}

	rootCmd.MarkFlagsMutuallyExclusive(
		"settings",
		"show",
		"delete",
		"list",
		"continue",
		"continue-last",
		"reset-settings",
	)
}

func main() {
	var err error
	config, err = ensureConfig()
	if err != nil {
		handleError(modsError{err, "Could not load your configuration file."})
		os.Exit(1)
	}

	cache = newCache(config.CachePath)
	db, err = openDB(filepath.Join(config.CachePath, "mods.db"))
	if err != nil {
		handleError(modsError{err, "Could not open database."})
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
		handleError(err)
		_ = db.Close()
		os.Exit(1)
	}
}

func handleError(err error) {
	// exhaust stdin
	if !isInputTTY() {
		_, _ = io.ReadAll(os.Stdin)
	}

	format := "\n%s\n\n"

	var args []interface{}
	var ferr flagParseError
	var merr modsError
	if errors.As(err, &ferr) {
		format += "%s\n\n"
		args = []interface{}{
			fmt.Sprintf(
				"Check out %s %s",
				stderrStyles().InlineCode.Render("mods -h"),
				stderrStyles().Comment.Render("for help."),
			),
			fmt.Sprintf(
				ferr.ReasonFormat(),
				stderrStyles().InlineCode.Render(ferr.Flag()),
			),
		}
	} else if errors.As(err, &merr) {
		format += "%s\n\n"
		args = []interface{}{
			stderrStyles().ErrPadding.Render(stderrStyles().ErrorHeader.String(), merr.reason),
			stderrStyles().ErrPadding.Render(stderrStyles().ErrorDetails.Render(err.Error())),
		}
	} else {
		args = []interface{}{
			stderrStyles().ErrPadding.Render(stderrStyles().ErrorDetails.Render(err.Error())),
		}
	}

	fmt.Fprintf(os.Stderr, format, args...)
}

func resetSettings() error {
	_, err := os.Stat(config.SettingsPath)
	if err != nil {
		return modsError{err, "Couldn't read config file."}
	}
	inputFile, err := os.Open(config.SettingsPath)
	if err != nil {
		return modsError{err, "Couldn't open config file."}
	}
	defer inputFile.Close() //nolint:errcheck
	outputFile, err := os.Create(config.SettingsPath + ".bak")
	if err != nil {
		return modsError{err, "Couldn't backup config file."}
	}
	defer outputFile.Close() //nolint:errcheck
	_, err = io.Copy(outputFile, inputFile)
	if err != nil {
		return modsError{err, "Couldn't write config file."}
	}
	// The copy was successful, so now delete the original file
	err = os.Remove(config.SettingsPath)
	if err != nil {
		return modsError{err, "Couldn't remove config file."}
	}
	err = writeConfigFile(config.SettingsPath)
	if err != nil {
		return modsError{err, "Couldn't write new config file."}
	}
	fmt.Fprintln(os.Stderr, "\nSettings restored to defaults!")
	fmt.Fprintf(os.Stderr,
		"\n  %s %s\n\n",
		stderrStyles().Comment.Render("Your old settings have been saved to:"),
		stderrStyles().Link.Render(config.SettingsPath+".bak"),
	)
	return nil
}

func deleteConversation() error {
	convo, err := db.Find(config.Delete)
	if err != nil {
		return modsError{err, "Couldn't find conversation to delete."}
	}

	if err := db.Delete(convo.ID); err != nil {
		return modsError{err, "Couldn't delete conversation."}
	}

	if err := cache.delete(convo.ID); err != nil {
		return modsError{err, "Couldn't delete conversation."}
	}

	fmt.Fprintln(os.Stderr, "Conversation deleted:", convo.ID[:sha1minLen])
	return nil
}

func listConversations() error {
	conversations, err := db.List()
	if err != nil {
		return modsError{err, "Couldn't list saves."}
	}

	if len(conversations) == 0 {
		fmt.Fprintln(os.Stderr, "No conversations found.")
		return nil
	}

	for _, conversation := range conversations {
		if isOutputTTY() {
			fmt.Fprintf(
				os.Stdout,
				"%s %s %s %s\n",
				stdoutStyles().Comment.Render("•"),
				stdoutStyles().SHA1.Render(conversation.ID[:sha1short]),
				conversation.Title,
				stdoutStyles().Timeago.Render(timeago.Of(conversation.UpdatedAt)),
			)
			continue
		}
		fmt.Fprintf(
			os.Stdout,
			"%s\t%s\t%s\n",
			conversation.ID[:sha1short],
			conversation.Title,
			timeago.Of(conversation.UpdatedAt),
		)
	}
	return nil
}

func saveConversation(mods *Mods) error {
	if config.NoCache {
		fmt.Fprintf(
			os.Stderr,
			"\nConversation was not saved because %s or %s is set.\n",
			stderrStyles().InlineCode.Render("--no-cache"),
			stderrStyles().InlineCode.Render("NO_CACHE"),
		)
		return nil
	}

	// if message is a sha1, use the last prompt instead.
	id := config.cacheWriteToID
	title := strings.TrimSpace(config.cacheWriteToTitle)

	if sha1reg.MatchString(title) || title == "" {
		title = firstLine(lastPrompt(mods.messages))
	}

	if err := cache.write(id, &mods.messages); err != nil {
		return modsError{err, fmt.Sprintf(
			"There was a problem writing %s to the cache. Use %s / %s to disable it.",
			config.cacheWriteToID,
			stderrStyles().InlineCode.Render("--no-cache"),
			stderrStyles().InlineCode.Render("NO_CACHE"),
		)}
	}
	if err := db.Save(id, title); err != nil {
		_ = cache.delete(id) // remove leftovers
		return modsError{err, fmt.Sprintf(
			"There was a problem writing %s to the cache. Use %s / %s to disable it.",
			config.cacheWriteToID,
			stderrStyles().InlineCode.Render("--no-cache"),
			stderrStyles().InlineCode.Render("NO_CACHE"),
		)}
	}

	fmt.Fprintln(
		os.Stderr,
		"\nConversation saved:",
		stderrStyles().InlineCode.Render(config.cacheWriteToID[:sha1short]),
		stderrStyles().Comment.Render(title),
	)
	return nil
}
