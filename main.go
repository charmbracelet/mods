package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"
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

func main() {
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
	if mods.Config.Version {
		fmt.Println(buildVersion())
		os.Exit(0)
	}
	if mods.Config.ShowHelp || (mods.Input == "" && mods.Config.Prefix == "") {
		flag.Usage()
		os.Exit(0)
	}
	fmt.Println(mods.FormattedOutput())
}
