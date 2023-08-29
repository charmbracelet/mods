package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glow/editor"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	flag "github.com/spf13/pflag"
	"golang.org/x/term"
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
	result := "mods version " + version
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
	me := &modsError{reason: reason, err: err}
	exitErrorFn(
		mods,
		func(_ styles) *modsError { return me },
	)
}

func exitErrorFn(mods *Mods, fn func(styles styles) *modsError) {
	if mods != nil && mods.db != nil {
		_ = mods.db.Close()
	}

	if !isErrTTY() {
		me := fn(styles{})
		fmt.Fprintf(os.Stderr, "%s\n%s\n", me.reason, me.err.Error())
		os.Exit(1)
	}

	renderer := lipgloss.NewRenderer(os.Stderr)
	width, _, _ := term.GetSize(int(os.Stderr.Fd()))
	styles := makeStyles(renderer)
	me := fn(styles)
	fmt.Fprintln(
		os.Stderr,
		errorView(me, renderer, styles, width),
	)
	os.Exit(1)
}

func exit(mods *Mods, status int) {
	if mods.db == nil {
		os.Exit(status)
	}
	_ = mods.db.Close()
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
}

func main() {
	renderer := lipgloss.NewRenderer(os.Stderr, termenv.WithColorCache(true))
	opts := []tea.ProgramOption{
		tea.WithOutput(renderer.Output()),
		tea.WithoutEmptyRenders(),
	}

	if !isInputTTY() {
		opts = append(opts, tea.WithInput(nil))
	}

	cfg, err := newConfig()
	if err != nil {
		exitErrorFn(nil, func(styles styles) *modsError {
			me := &modsError{}
			var fpe flagParseError
			switch {
			case errors.As(err, &fpe):
				me.reason = fmt.Sprintf("Missing flag: %s", styles.InlineCode.Render(fpe.Flag()))
				me.err = fmt.Errorf(
					"check out %s %s",
					styles.InlineCode.Render("mods -h"),
					styles.Comment.Render("for help."),
				)
			default:
				me.err = err
				me.reason = "There was an error loading your config file."
			}
			return me
		})
	}
	mods := newMods(renderer, cfg)
	p := tea.NewProgram(mods, opts...)
	m, err := p.Run()
	if err != nil {
		exitError(mods, err, "Couldn't start Bubble Tea program.")
	}
	mods = m.(*Mods)
	if mods.Error != nil {
		exit(mods, 1)
	}

	if mods.Config.Version {
		fmt.Fprintln(os.Stderr, buildVersion())
		exit(mods, 0)
	}

	if mods.Config.Settings {
		c := editor.Cmd(mods.Config.SettingsPath)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			exitError(mods, err, "Missing $EDITOR")
		}
		fmt.Fprintln(os.Stderr, "Wrote config file to:", mods.Config.SettingsPath)
		exit(mods, 0)
	}

	if mods.Config.ResetSettings {
		resetSettings(mods)
	}

	if mods.Config.ShowHelp || (mods.Input == "" &&
		mods.Config.Prefix == "" &&
		mods.Config.Show == "" &&
		mods.Config.Delete == "" &&
		!mods.Config.List) {
		flag.Usage()
		exit(mods, 0)
	}

	if mods.Config.Show != "" {
		exit(mods, 0)
	}

	if mods.Config.List {
		listConversations(mods)
	}

	if mods.Config.Delete != "" {
		deleteConversation(mods)
	}

	if mods.Config.cacheWriteToID != "" {
		writeConversation(mods)
	}

	exit(mods, 0)
}

func resetSettings(mods *Mods) {
	_, err := os.Stat(mods.Config.SettingsPath)
	if err != nil {
		exitError(mods, err, "Couldn't read config file.")
	}
	inputFile, err := os.Open(mods.Config.SettingsPath)
	if err != nil {
		exitError(mods, err, "Couldn't open config file.")
	}
	defer inputFile.Close() //nolint:errcheck
	outputFile, err := os.Create(mods.Config.SettingsPath + ".bak")
	if err != nil {
		exitError(mods, err, "Couldn't backup config file.")
	}
	defer outputFile.Close() //nolint:errcheck
	_, err = io.Copy(outputFile, inputFile)
	if err != nil {
		exitError(mods, err, "Couldn't write config file.")
	}
	// The copy was successful, so now delete the original file
	err = os.Remove(mods.Config.SettingsPath)
	if err != nil {
		exitError(mods, err, "Couldn't remove config file.")
	}
	err = writeConfigFile(mods.Config.SettingsPath)
	if err != nil {
		exitError(mods, err, "Couldn't write new config file.")
	}
	fmt.Fprintln(os.Stderr, "\nSettings restored to defaults!")
	fmt.Fprintf(os.Stderr,
		"\n  %s %s\n\n",
		mods.Styles.Comment.Render("Your old settings have been saved to:"),
		mods.Styles.Link.Render(mods.Config.SettingsPath+".bak"),
	)
	exit(mods, 0)
}

func deleteConversation(mods *Mods) {
	convo, err := mods.db.Find(mods.Config.Delete)
	if err != nil {
		exitError(mods, err, "Couldn't delete conversation.")
	}

	if err := mods.db.Delete(convo.ID); err != nil {
		exitError(mods, err, "Couldn't delete conversation.")
	}

	if err := mods.cache.delete(convo.ID); err != nil {
		exitError(mods, err, "Couldn't delete conversation.")
	}

	fmt.Fprintln(os.Stderr, "Conversation deleted:", convo.ID[:sha1minLen])
	exit(mods, 0)
}

func listConversations(mods *Mods) {
	conversations, err := mods.db.List()
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
	for _, conversation := range conversations {
		fmt.Fprintf(os.Stderr, "%s %s %s\n",
			mods.Styles.Comment.Render("•"),
			conversation.ID[:sha1short],
			mods.Styles.Comment.Render(conversation.Title),
		)
	}
	exit(mods, 0)
}

func writeConversation(mods *Mods) {
	// if message is a sha1, use the last prompt instead.
	id := mods.Config.cacheWriteToID
	title := strings.TrimSpace(mods.Config.cacheWriteToTitle)

	if sha1reg.MatchString(title) || title == "" {
		title = firstLine(lastPrompt(mods.messages))
	}

	// conversation would already have been written to the file storage, just
	// need to write to db too.
	if err := mods.db.Save(id, title); err != nil {
		exitError(mods, err, "Couldn't save conversation.")
	}

	fmt.Fprintln(
		os.Stderr,
		"\nConversation saved:",
		mods.Config.cacheWriteToID[:sha1short],
		mods.Styles.Comment.Render(mods.Config.cacheWriteToTitle),
	)
}
