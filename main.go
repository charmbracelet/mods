package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glow/editor"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
	flag "github.com/spf13/pflag"
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
	mods.Error = &modsError{reason: reason, err: err}
	fmt.Println(mods.ErrorView())
	os.Exit(1)
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
	f, err := tea.LogToFile("/tmp/mods.log", "")
	if err != nil {
		os.Exit(1)
	}
	defer f.Close()

	renderer := lipgloss.NewRenderer(os.Stderr, termenv.WithColorCache(true))
	opts := []tea.ProgramOption{tea.WithOutput(renderer.Output())}

	if !isatty.IsTerminal(os.Stdin.Fd()) {
		opts = append(opts, tea.WithInput(nil))
	}
	mods := newMods(renderer)
	p := tea.NewProgram(mods, opts...)
	m, err := p.Run()
	if err != nil {
		exitError(mods, err, "Couldn't start Bubble Tea program.")
	}
	mods = m.(*Mods)
	if mods.Error != nil {
		os.Exit(1)
	}

	if mods.Config.Version {
		fmt.Println(buildVersion())
		os.Exit(0)
	}

	if mods.Config.ShowHelp || (mods.Input == "" && mods.Config.Prefix == "" &&
		mods.Config.Show == "" && mods.Config.Delete == "" && !mods.Config.List) {
		flag.Usage()
		os.Exit(0)
	}

	if mods.Config.Settings {
		c := editor.Cmd(mods.Config.SettingsPath)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			exitError(mods, err, "Missing $EDITOR")
		}
		fmt.Println("Wrote config file to:", mods.Config.SettingsPath)
		os.Exit(0)
	}

	if mods.Config.ResetSettings {
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
		fmt.Println("\n  Settings restored to defaults!")
		fmt.Printf(
			"\n  %s %s\n\n",
			mods.Styles.Comment.Render("Your old settings are have been saved to:"),
			mods.Styles.Link.Render(mods.Config.SettingsPath+".bak"),
		)
		os.Exit(0)
	}

	if mods.Config.Show != "" {
		os.Exit(0)
	}

	if mods.Config.List {
		conversations, err := mods.db.List()
		if err != nil {
			exitError(mods, err, "Couldn't list saves.")
		}

		if len(conversations) == 0 {
			fmt.Printf("  No conversations found.\n")
			os.Exit(0)
		}

		fmt.Printf("  Saved conversations %s:\n", mods.Styles.Comment.Render("("+fmt.Sprint(len(conversations))+")"))
		for _, conversation := range conversations {
			fmt.Printf("  %s %s: %s\n",
				mods.Styles.Comment.Render("•"),
				conversation.ID[:sha1short],
				mods.Styles.Comment.Render(conversation.Title),
			)
		}
		os.Exit(0)
	}

	if mods.Config.Delete != "" {
		id, err := mods.db.Find(mods.Config.Delete)
		if err != nil {
			exitError(mods, err, "Couldn't delete conversation.")
		}

		if err := mods.db.Delete(id); err != nil {
			exitError(mods, err, "Couldn't delete conversation.")
		}

		if err := mods.cache.delete(id); err != nil {
			exitError(mods, err, "Couldn't delete conversation.")
		}

		fmt.Println("  Conversation deleted:", mods.Config.Delete)
		os.Exit(0)
	}

	if mods.Config.cacheWriteToID != "" {
		if strings.HasPrefix(mods.Config.cacheWriteToID, mods.Config.cacheWriteToTitle) {
			mods.Config.cacheWriteToTitle = firstLine(lastPrompt(mods.messages))
		}

		if err := mods.db.Save(mods.Config.cacheWriteToID, mods.Config.cacheWriteToTitle); err != nil {
			exitError(mods, err, "Couldn't save conversation.")
		}
		fmt.Println("\n  Conversation saved:", mods.Config.cacheWriteToID[:sha1short])
		os.Exit(0)
	}

	fmt.Println(mods.FormattedOutput())
}
