// Package main provides the mods CLI.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"runtime/pprof"
	"slices"
	"strings"

	"github.com/atotto/clipboard"
	timeago "github.com/caarlos0/timea.go"
	tea "github.com/charmbracelet/bubbletea"
	glamour "github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/mods/internal/cache"
	"github.com/charmbracelet/x/editor"
	mcobra "github.com/muesli/mango-cobra"
	"github.com/muesli/roff"
	"github.com/muesli/termenv"
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
		return newFlagParseError(err)
	})

	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
}

var (
	config = defaultConfig()
	db     *convoDB

	rootCmd = &cobra.Command{
		Use:           "mods",
		Short:         "GPT on the command line. Built for pipelines.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Example:       randomExample(),
		RunE: func(cmd *cobra.Command, args []string) error {
			config.Prefix = removeWhitespace(strings.Join(args, " "))

			opts := []tea.ProgramOption{}

			if !isInputTTY() || config.Raw {
				opts = append(opts, tea.WithInput(nil))
			}
			if isOutputTTY() && !config.Raw {
				opts = append(opts, tea.WithOutput(os.Stderr))
			} else {
				opts = append(opts, tea.WithoutRenderer())
			}
			if os.Getenv("VIMRUNTIME") != "" {
				config.Quiet = true
			}

			if isNoArgs() && isInputTTY() && config.openEditor {
				prompt, err := prefixFromEditor()
				if err != nil {
					return err
				}
				config.Prefix = prompt
			}

			if (isNoArgs() || config.AskModel) && isInputTTY() {
				if err := askInfo(); err != nil && err == huh.ErrUserAborted {
					return modsError{
						err:    err,
						reason: "User canceled.",
					}
				} else if err != nil {
					return modsError{
						err:    err,
						reason: "Prompt failed.",
					}
				}
			}

			cache, err := cache.NewConversations(config.CachePath)
			if err != nil {
				return modsError{err, "Couldn't start Bubble Tea program."}
			}
			mods := newMods(cmd.Context(), stderrRenderer(), &config, db, cache)
			p := tea.NewProgram(mods, opts...)
			m, err := p.Run()
			if err != nil {
				return modsError{err, "Couldn't start Bubble Tea program."}
			}

			mods = m.(*Mods)
			if mods.Error != nil {
				return *mods.Error
			}

			if config.Dirs {
				if len(args) > 0 {
					switch args[0] {
					case "config":
						fmt.Println(filepath.Dir(config.SettingsPath))
						return nil
					case "cache":
						fmt.Println(config.CachePath)
						return nil
					}
				}
				fmt.Printf("Configuration: %s\n", filepath.Dir(config.SettingsPath))
				//nolint:mnd
				fmt.Printf("%*sCache: %s\n", 8, " ", config.CachePath)
				return nil
			}

			if config.Settings {
				c, err := editor.Cmd("mods", config.SettingsPath)
				if err != nil {
					return modsError{
						err:    err,
						reason: "Could not edit your settings file.",
					}
				}
				c.Stdin = os.Stdin
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
				if err := c.Run(); err != nil {
					return modsError{err, fmt.Sprintf(
						"Missing %s.",
						stderrStyles().InlineCode.Render("$EDITOR"),
					)}
				}

				if !config.Quiet {
					fmt.Fprintln(os.Stderr, "Wrote config file to:", config.SettingsPath)
				}
				return nil
			}

			if config.ResetSettings {
				return resetSettings()
			}

			if mods.Input == "" && isNoArgs() {
				return modsError{
					reason: "You haven't provided any prompt input.",
					err: newUserErrorf(
						"You can give your prompt as arguments and/or pipe it from STDIN.\nExample: %s",
						stdoutStyles().InlineCode.Render("mods [prompt]"),
					),
				}
			}

			if config.ShowHelp {
				return cmd.Usage()
			}

			if config.ListRoles {
				listRoles()
				return nil
			}
			if config.List {
				return listConversations(config.Raw)
			}

			if config.MCPList {
				mcpList()
				return nil
			}

			if config.MCPListTools {
				ctx, cancel := context.WithTimeout(cmd.Context(), config.MCPTimeout)
				defer cancel()
				return mcpListTools(ctx)
			}

			if len(config.Delete) > 0 {
				return deleteConversations()
			}

			if config.DeleteOlderThan > 0 {
				return deleteConversationOlderThan()
			}

			// raw mode already prints the output, no need to print it again
			if isOutputTTY() && !config.Raw {
				switch {
				case mods.glamOutput != "":
					fmt.Print(mods.glamOutput)
				case mods.Output != "":
					fmt.Print(mods.Output)
				}
			}

			if config.Show != "" || config.ShowLast {
				return nil
			}

			if config.cacheWriteToID != "" {
				return saveConversation(mods)
			}

			return nil
		},
	}
)

var memprofile bool

func initFlags() {
	flags := rootCmd.Flags()
	flags.StringVarP(&config.Model, "model", "m", config.Model, stdoutStyles().FlagDesc.Render(help["model"]))
	flags.BoolVarP(&config.AskModel, "ask-model", "M", config.AskModel, stdoutStyles().FlagDesc.Render(help["ask-model"]))
	flags.StringVarP(&config.API, "api", "a", config.API, stdoutStyles().FlagDesc.Render(help["api"]))
	flags.StringVarP(&config.HTTPProxy, "http-proxy", "x", config.HTTPProxy, stdoutStyles().FlagDesc.Render(help["http-proxy"]))
	flags.BoolVarP(&config.Format, "format", "f", config.Format, stdoutStyles().FlagDesc.Render(help["format"]))
	flags.StringVar(&config.FormatAs, "format-as", config.FormatAs, stdoutStyles().FlagDesc.Render(help["format-as"]))
	flags.BoolVarP(&config.Raw, "raw", "r", config.Raw, stdoutStyles().FlagDesc.Render(help["raw"]))
	flags.IntVarP(&config.IncludePrompt, "prompt", "P", config.IncludePrompt, stdoutStyles().FlagDesc.Render(help["prompt"]))
	flags.BoolVarP(&config.IncludePromptArgs, "prompt-args", "p", config.IncludePromptArgs, stdoutStyles().FlagDesc.Render(help["prompt-args"]))
	flags.StringVarP(&config.Continue, "continue", "c", "", stdoutStyles().FlagDesc.Render(help["continue"]))
	flags.BoolVarP(&config.ContinueLast, "continue-last", "C", false, stdoutStyles().FlagDesc.Render(help["continue-last"]))
	flags.BoolVarP(&config.List, "list", "l", config.List, stdoutStyles().FlagDesc.Render(help["list"]))
	flags.StringVarP(&config.Title, "title", "t", config.Title, stdoutStyles().FlagDesc.Render(help["title"]))
	flags.StringArrayVarP(&config.Delete, "delete", "d", config.Delete, stdoutStyles().FlagDesc.Render(help["delete"]))
	flags.Var(newDurationFlag(config.DeleteOlderThan, &config.DeleteOlderThan), "delete-older-than", stdoutStyles().FlagDesc.Render(help["delete-older-than"]))
	flags.StringVarP(&config.Show, "show", "s", config.Show, stdoutStyles().FlagDesc.Render(help["show"]))
	flags.BoolVarP(&config.ShowLast, "show-last", "S", false, stdoutStyles().FlagDesc.Render(help["show-last"]))
	flags.BoolVarP(&config.Quiet, "quiet", "q", config.Quiet, stdoutStyles().FlagDesc.Render(help["quiet"]))
	flags.BoolVarP(&config.ShowHelp, "help", "h", false, stdoutStyles().FlagDesc.Render(help["help"]))
	flags.BoolVarP(&config.Version, "version", "v", false, stdoutStyles().FlagDesc.Render(help["version"]))
	flags.IntVar(&config.MaxRetries, "max-retries", config.MaxRetries, stdoutStyles().FlagDesc.Render(help["max-retries"]))
	flags.BoolVar(&config.NoLimit, "no-limit", config.NoLimit, stdoutStyles().FlagDesc.Render(help["no-limit"]))
	flags.Int64Var(&config.MaxTokens, "max-tokens", config.MaxTokens, stdoutStyles().FlagDesc.Render(help["max-tokens"]))
	flags.IntVar(&config.WordWrap, "word-wrap", config.WordWrap, stdoutStyles().FlagDesc.Render(help["word-wrap"]))
	flags.Float64Var(&config.Temperature, "temp", config.Temperature, stdoutStyles().FlagDesc.Render(help["temp"]))
	flags.StringArrayVar(&config.Stop, "stop", config.Stop, stdoutStyles().FlagDesc.Render(help["stop"]))
	flags.Float64Var(&config.TopP, "topp", config.TopP, stdoutStyles().FlagDesc.Render(help["topp"]))
	flags.Int64Var(&config.TopK, "topk", config.TopK, stdoutStyles().FlagDesc.Render(help["topk"]))
	flags.UintVar(&config.Fanciness, "fanciness", config.Fanciness, stdoutStyles().FlagDesc.Render(help["fanciness"]))
	flags.StringVar(&config.StatusText, "status-text", config.StatusText, stdoutStyles().FlagDesc.Render(help["status-text"]))
	flags.BoolVar(&config.NoCache, "no-cache", config.NoCache, stdoutStyles().FlagDesc.Render(help["no-cache"]))
	flags.BoolVar(&config.ResetSettings, "reset-settings", config.ResetSettings, stdoutStyles().FlagDesc.Render(help["reset-settings"]))
	flags.BoolVar(&config.Settings, "settings", false, stdoutStyles().FlagDesc.Render(help["settings"]))
	flags.BoolVar(&config.Dirs, "dirs", false, stdoutStyles().FlagDesc.Render(help["dirs"]))
	flags.StringVarP(&config.Role, "role", "R", config.Role, stdoutStyles().FlagDesc.Render(help["role"]))
	flags.BoolVar(&config.ListRoles, "list-roles", config.ListRoles, stdoutStyles().FlagDesc.Render(help["list-roles"]))
	flags.StringVar(&config.Theme, "theme", "charm", stdoutStyles().FlagDesc.Render(help["theme"]))
	flags.BoolVarP(&config.openEditor, "editor", "e", false, stdoutStyles().FlagDesc.Render(help["editor"]))
	flags.BoolVar(&config.MCPList, "mcp-list", false, stdoutStyles().FlagDesc.Render(help["mcp-list"]))
	flags.BoolVar(&config.MCPListTools, "mcp-list-tools", false, stdoutStyles().FlagDesc.Render(help["mcp-list-tools"]))
	flags.StringArrayVar(&config.MCPDisable, "mcp-disable", nil, stdoutStyles().FlagDesc.Render(help["mcp-disable"]))
	flags.Lookup("prompt").NoOptDefVal = "-1"
	flags.SortFlags = false

	flags.BoolVar(&memprofile, "memprofile", false, "Write memory profiles to CWD")
	_ = flags.MarkHidden("memprofile")

	for _, name := range []string{"show", "delete", "continue"} {
		_ = rootCmd.RegisterFlagCompletionFunc(name, func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			results, _ := db.Completions(toComplete)
			return results, cobra.ShellCompDirectiveDefault
		})
	}
	_ = rootCmd.RegisterFlagCompletionFunc("role", func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return roleNames(toComplete), cobra.ShellCompDirectiveDefault
	})

	if config.FormatText == nil {
		config.FormatText = defaultConfig().FormatText
	}

	if config.Format && config.FormatAs == "" {
		config.FormatAs = "markdown"
	}

	if config.Format && config.FormatAs != "" && config.FormatText[config.FormatAs] == "" {
		config.FormatText[config.FormatAs] = defaultConfig().FormatText[config.FormatAs]
	}

	if config.MCPTimeout == 0 {
		config.MCPTimeout = defaultConfig().MCPTimeout
	}

	rootCmd.MarkFlagsMutuallyExclusive(
		"settings",
		"show",
		"show-last",
		"delete",
		"delete-older-than",
		"list",
		"continue",
		"continue-last",
		"reset-settings",
		"mcp-list",
		"mcp-list-tools",
	)
}

func main() {
	defer maybeWriteMemProfile()
	var err error
	config, err = ensureConfig()
	if err != nil {
		handleError(modsError{err, "Could not load your configuration file."})
		// if user is editing the settings, only print out the error, but do
		// not exit.
		if !slices.Contains(os.Args, "--settings") {
			os.Exit(1)
		}
	}

	// XXX: this must come after creating the config.
	initFlags()

	if !isCompletionCmd(os.Args) && !isManCmd(os.Args) && !isVersionOrHelpCmd(os.Args) {
		db, err = openDB(filepath.Join(config.CachePath, "conversations", "mods.db"))
		if err != nil {
			handleError(modsError{err, "Could not open database."})
			os.Exit(1)
		}
		defer db.Close() //nolint:errcheck
	}

	if isCompletionCmd(os.Args) {
		// XXX: since mods doesn't have any sub-commands, Cobra won't create
		// the default `completion` command. Forcefully create the completion
		// related sub-commands by adding a fake command when completions are
		// being used.
		rootCmd.AddCommand(&cobra.Command{
			Use:    "____fake_command_to_enable_completions",
			Hidden: true,
		})
		rootCmd.InitDefaultCompletionCmd()
	}

	if isManCmd(os.Args) {
		rootCmd.AddCommand(&cobra.Command{
			Use:                   "man",
			Short:                 "Generates manpages",
			SilenceUsage:          true,
			DisableFlagsInUseLine: true,
			Hidden:                true,
			Args:                  cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				manPage, err := mcobra.NewManPage(1, rootCmd)
				if err != nil {
					//nolint:wrapcheck
					return err
				}
				_, err = fmt.Fprint(os.Stdout, manPage.Build(roff.NewDocument()))
				//nolint:wrapcheck
				return err
			},
		})
	}

	if err := rootCmd.Execute(); err != nil {
		handleError(err)
		_ = db.Close()
		os.Exit(1)
	}
}

func maybeWriteMemProfile() {
	if !memprofile {
		return
	}

	closers := []func() error{db.Close}
	defer func() {
		for _, cl := range closers {
			_ = cl()
		}
	}()

	handle := func(err error) {
		fmt.Println(err)
		for _, cl := range closers {
			_ = cl()
		}
		os.Exit(1)
	}

	heap, err := os.Create("mods_heap.profile")
	if err != nil {
		handle(err)
	}
	closers = append(closers, heap.Close)
	allocs, err := os.Create("mods_allocs.profile")
	if err != nil {
		handle(err)
	}
	closers = append(closers, allocs.Close)

	if err := pprof.Lookup("heap").WriteTo(heap, 0); err != nil {
		handle(err)
	}
	if err := pprof.Lookup("allocs").WriteTo(allocs, 0); err != nil {
		handle(err)
	}
}

func handleError(err error) {
	maybeWriteMemProfile()
	// exhaust stdin
	if !isInputTTY() {
		_, _ = io.ReadAll(os.Stdin)
	}

	format := "\n%s\n\n"

	var args []any
	var ferr flagParseError
	var merr modsError
	if errors.As(err, &ferr) {
		format += "%s\n\n"
		args = []any{
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
		args = []any{
			stderrStyles().ErrPadding.Render(stderrStyles().ErrorHeader.String(), merr.reason),
		}

		// Skip the error details if the user simply canceled out of huh.
		if merr.err != huh.ErrUserAborted {
			format += "%s\n\n"
			args = append(args, stderrStyles().ErrPadding.Render(stderrStyles().ErrorDetails.Render(err.Error())))
		}
	} else {
		args = []any{
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
	if !config.Quiet {
		fmt.Fprintln(os.Stderr, "\nSettings restored to defaults!")
		fmt.Fprintf(os.Stderr,
			"\n  %s %s\n\n",
			stderrStyles().Comment.Render("Your old settings have been saved to:"),
			stderrStyles().Link.Render(config.SettingsPath+".bak"),
		)
	}
	return nil
}

func deleteConversationOlderThan() error {
	conversations, err := db.ListOlderThan(config.DeleteOlderThan)
	if err != nil {
		return modsError{err, "Couldn't find conversation to delete."}
	}

	if len(conversations) == 0 {
		if !config.Quiet {
			fmt.Fprintln(os.Stderr, "No conversations found.")
			return nil
		}
		return nil
	}

	if !config.Quiet {
		printList(conversations)

		if !isOutputTTY() || !isInputTTY() {
			fmt.Fprintln(os.Stderr)
			return newUserErrorf(
				"To delete the conversations above, run: %s",
				strings.Join(append(os.Args, "--quiet"), " "),
			)
		}
		var confirm bool
		if err := huh.Run(
			huh.NewConfirm().
				Title(fmt.Sprintf("Delete conversations older than %s?", config.DeleteOlderThan)).
				Description(fmt.Sprintf("This will delete all the %d conversations listed above.", len(conversations))).
				Value(&confirm),
		); err != nil {
			return modsError{err, "Couldn't delete old conversations."}
		}
		if !confirm {
			return newUserErrorf("Aborted by user")
		}
	}

	cache, err := cache.NewConversations(config.CachePath)
	if err != nil {
		return modsError{err, "Couldn't delete conversation."}
	}
	for _, c := range conversations {
		if err := db.Delete(c.ID); err != nil {
			return modsError{err, "Couldn't delete conversation."}
		}

		if err := cache.Delete(c.ID); err != nil {
			return modsError{err, "Couldn't delete conversation."}
		}

		if !config.Quiet {
			fmt.Fprintln(os.Stderr, "Conversation deleted:", c.ID[:sha1minLen])
		}
	}

	return nil
}

func deleteConversations() error {
	for _, del := range config.Delete {
		convo, err := db.Find(del)
		if err != nil {
			return modsError{err, "Couldn't find conversation to delete."}
		}
		if err := deleteConversation(convo); err != nil {
			return err
		}
	}
	return nil
}

func deleteConversation(convo *Conversation) error {
	if err := db.Delete(convo.ID); err != nil {
		return modsError{err, "Couldn't delete conversation."}
	}

	cache, err := cache.NewConversations(config.CachePath)
	if err != nil {
		return modsError{err, "Couldn't delete conversation."}
	}
	if err := cache.Delete(convo.ID); err != nil {
		return modsError{err, "Couldn't delete conversation."}
	}

	if !config.Quiet {
		fmt.Fprintln(os.Stderr, "Conversation deleted:", convo.ID[:sha1minLen])
	}
	return nil
}

func listConversations(raw bool) error {
	conversations, err := db.List()
	if err != nil {
		return modsError{err, "Couldn't list saves."}
	}

	if len(conversations) == 0 {
		fmt.Fprintln(os.Stderr, "No conversations found.")
		return nil
	}

	if isInputTTY() && isOutputTTY() && !raw {
		selectFromList(conversations)
		return nil
	}
	printList(conversations)
	return nil
}

func roleNames(prefix string) []string {
	roles := make([]string, 0, len(config.Roles))
	for role := range config.Roles {
		if prefix != "" && !strings.HasPrefix(role, prefix) {
			continue
		}
		roles = append(roles, role)
	}
	slices.Sort(roles)
	return roles
}

func listRoles() {
	for _, role := range roleNames("") {
		s := role
		if role == config.Role {
			s = role + stdoutStyles().Timeago.Render(" (default)")
		}
		fmt.Println(s)
	}
}

func makeOptions(conversations []Conversation) []huh.Option[string] {
	opts := make([]huh.Option[string], 0, len(conversations))
	for _, c := range conversations {
		timea := stdoutStyles().Timeago.Render(timeago.Of(c.UpdatedAt))
		left := stdoutStyles().SHA1.Render(c.ID[:sha1short])
		right := stdoutStyles().ConversationList.Render(c.Title, timea)
		if c.Model != nil {
			right += stdoutStyles().Comment.Render(*c.Model)
		}
		if c.API != nil {
			right += stdoutStyles().Comment.Render(" (" + *c.API + ")")
		}
		opts = append(opts, huh.NewOption(left+" "+right, c.ID))
	}
	return opts
}

func selectFromList(conversations []Conversation) {
	var selected string
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Conversations").
				Value(&selected).
				Options(makeOptions(conversations)...),
		),
	).Run(); err != nil {
		if !errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		return
	}

	_ = clipboard.WriteAll(selected)
	termenv.Copy(selected)
	printConfirmation("COPIED", selected)
	// suggest actions to use this conversation ID
	fmt.Println(stdoutStyles().Comment.Render(
		"You can use this conversation ID with the following commands:",
	))
	suggestions := []string{"show", "continue", "delete"}
	for _, flag := range suggestions {
		fmt.Printf(
			"  %-44s %s\n",
			stdoutStyles().Flag.Render("--"+flag),
			stdoutStyles().FlagDesc.Render(help[flag]),
		)
	}
}

func printList(conversations []Conversation) {
	for _, conversation := range conversations {
		_, _ = fmt.Fprintf(
			os.Stdout,
			"%s\t%s\t%s\n",
			stdoutStyles().SHA1.Render(conversation.ID[:sha1short]),
			conversation.Title,
			stdoutStyles().Timeago.Render(timeago.Of(conversation.UpdatedAt)),
		)
	}
}

func saveConversation(mods *Mods) error {
	if config.NoCache {
		if !config.Quiet {
			fmt.Fprintf(
				os.Stderr,
				"\nConversation was not saved because %s or %s is set.\n",
				stderrStyles().InlineCode.Render("--no-cache"),
				stderrStyles().InlineCode.Render("NO_CACHE"),
			)
		}
		return nil
	}

	// if message is a sha1, use the last prompt instead.
	id := config.cacheWriteToID
	title := strings.TrimSpace(config.cacheWriteToTitle)

	if sha1reg.MatchString(title) || title == "" {
		title = firstLine(lastPrompt(mods.messages))
	}

	errReason := fmt.Sprintf(
		"There was a problem writing %s to the cache. Use %s / %s to disable it.",
		config.cacheWriteToID,
		stderrStyles().InlineCode.Render("--no-cache"),
		stderrStyles().InlineCode.Render("NO_CACHE"),
	)
	cache, err := cache.NewConversations(config.CachePath)
	if err != nil {
		return modsError{err, errReason}
	}
	if err := cache.Write(id, &mods.messages); err != nil {
		return modsError{err, errReason}
	}
	if err := db.Save(id, title, config.API, config.Model); err != nil {
		_ = cache.Delete(id) // remove leftovers
		return modsError{err, errReason}
	}

	if !config.Quiet {
		fmt.Fprintln(
			os.Stderr,
			"\nConversation saved:",
			stderrStyles().InlineCode.Render(config.cacheWriteToID[:sha1short]),
			stderrStyles().Comment.Render(title),
		)
	}
	return nil
}

func isNoArgs() bool {
	return config.Prefix == "" &&
		config.Show == "" &&
		!config.ShowLast &&
		len(config.Delete) == 0 &&
		config.DeleteOlderThan == 0 &&
		!config.ShowHelp &&
		!config.List &&
		!config.ListRoles &&
		!config.MCPList &&
		!config.MCPListTools &&
		!config.Dirs &&
		!config.Settings &&
		!config.ResetSettings
}

func askInfo() error {
	var foundModel bool
	apis := make([]huh.Option[string], 0, len(config.APIs))
	opts := map[string][]huh.Option[string]{}
	for _, api := range config.APIs {
		apis = append(apis, huh.NewOption(api.Name, api.Name))
		for name, model := range api.Models {
			opts[api.Name] = append(opts[api.Name], huh.NewOption(name, name))

			// checks if this is the model we intend to use if not using
			// `--ask-model`:
			if !config.AskModel &&
				(config.API == "" || config.API == api.Name) &&
				(config.Model == name || slices.Contains(model.Aliases, config.Model)) {
				// if it is, adjusts api and model so its cheaper later on.
				config.API = api.Name
				config.Model = name
				foundModel = true
			}
		}
	}

	if config.ContinueLast {
		found, err := db.FindHEAD()
		if err == nil && found != nil && found.Model != nil && found.API != nil {
			config.Model = *found.Model
			config.API = *found.API
			foundModel = true
		}
	}

	// wrapping is done by the caller
	//nolint:wrapcheck
	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Choose the API:").
				Options(apis...).
				Value(&config.API),
			huh.NewSelect[string]().
				TitleFunc(func() string {
					return fmt.Sprintf("Choose the model for '%s':", config.API)
				}, &config.API).
				OptionsFunc(func() []huh.Option[string] {
					return opts[config.API]
				}, &config.API).
				Value(&config.Model),
		).WithHideFunc(func() bool {
			// AskModel is true if the user is passing a flag to ask;
			// FoundModel is true if a model is found for whatever config the
			// user has (either --api/--model or default-api and
			// default-model in settings).
			// So, it'll only hide this if the user didn't run with
			// `--ask-model` AND the configuration yields a valid model.
			return !config.AskModel && foundModel
		}),
		huh.NewGroup(
			huh.NewText().
				TitleFunc(func() string {
					return fmt.Sprintf("Enter a prompt for %s/%s:", config.API, config.Model)
				}, &config.Model).
				Value(&config.Prefix),
		).WithHideFunc(func() bool {
			return config.Prefix != ""
		}),
	).
		WithTheme(themeFrom(config.Theme)).
		Run()
}

//nolint:mnd
func isManCmd(args []string) bool {
	if len(args) == 2 {
		return args[1] == "man"
	}
	if len(args) == 3 && args[1] == "man" {
		return args[2] == "-h" || args[2] == "--help"
	}
	return false
}

//nolint:mnd
func isCompletionCmd(args []string) bool {
	if len(args) <= 1 {
		return false
	}
	if args[1] == "__complete" {
		return true
	}
	if args[1] != "completion" {
		return false
	}
	if len(args) == 3 {
		_, ok := map[string]any{
			"bash":       nil,
			"fish":       nil,
			"zsh":        nil,
			"powershell": nil,
			"-h":         nil,
			"--help":     nil,
			"help":       nil,
		}[args[2]]
		return ok
	}
	if len(args) == 4 {
		_, ok := map[string]any{
			"-h":     nil,
			"--help": nil,
		}[args[3]]
		return ok
	}
	return false
}

//nolint:mnd
func isVersionOrHelpCmd(args []string) bool {
	if len(args) <= 1 {
		return false
	}
	for _, arg := range args[1:] {
		if arg == "--version" || arg == "-v" || arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func themeFrom(theme string) *huh.Theme {
	switch theme {
	case "dracula":
		return huh.ThemeDracula()
	case "catppuccin":
		return huh.ThemeCatppuccin()
	case "base16":
		return huh.ThemeBase16()
	default:
		return huh.ThemeCharm()
	}
}

// creates a temp file, opens it in user's editor, and then returns its contents.
func prefixFromEditor() (string, error) {
	f, err := os.CreateTemp("", "prompt")
	if err != nil {
		return "", fmt.Errorf("could not create temporary file: %w", err)
	}
	_ = f.Close()
	defer func() { _ = os.Remove(f.Name()) }()
	cmd, err := editor.Cmd(
		"mods",
		f.Name(),
	)
	if err != nil {
		return "", fmt.Errorf("could not open editor: %w", err)
	}
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("could not open editor: %w", err)
	}
	prompt, err := os.ReadFile(f.Name())
	if err != nil {
		return "", fmt.Errorf("could not read file: %w", err)
	}
	return string(prompt), nil
}
