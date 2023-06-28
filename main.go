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

type styles struct {
	appName      lipgloss.Style
	cliArgs      lipgloss.Style
	comment      lipgloss.Style
	cyclingChars lipgloss.Style
	errorHeader  lipgloss.Style
	errorDetails lipgloss.Style
	flag         lipgloss.Style
	flagComma    lipgloss.Style
	flagDesc     lipgloss.Style
	inlineCode   lipgloss.Style
	link         lipgloss.Style
	pipe         lipgloss.Style
	quote        lipgloss.Style
}

func makeStyles(r *lipgloss.Renderer) (s styles) {
	s.appName = r.NewStyle().Bold(true)
	s.cliArgs = r.NewStyle().Foreground(lipgloss.Color("#585858"))
	s.comment = r.NewStyle().Foreground(lipgloss.Color("#757575"))
	s.cyclingChars = r.NewStyle().Foreground(lipgloss.Color("#FF87D7"))
	s.errorHeader = r.NewStyle().Foreground(lipgloss.Color("#F1F1F1")).Background(lipgloss.Color("#FF5F87")).Bold(true).Padding(0, 1).SetString("ERROR")
	s.errorDetails = s.comment.Copy()
	s.flag = r.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#00B594", Dark: "#3EEFCF"}).Bold(true)
	s.flagComma = r.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#5DD6C0", Dark: "#427C72"}).SetString(",")
	s.flagDesc = s.comment.Copy()
	s.inlineCode = r.NewStyle().Foreground(lipgloss.Color("#FF5F87")).Background(lipgloss.Color("#3A3A3A")).Padding(0, 1)
	s.link = r.NewStyle().Foreground(lipgloss.Color("#00AF87")).Underline(true)
	s.quote = r.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#FF71D0", Dark: "#FF78D2"})
	s.pipe = r.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#8470FF", Dark: "#745CFF"})
	return s
}

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
		fmt.Println(err)
		os.Exit(1)
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
		fmt.Printf("Settings restored to defaults! Your old settings are backed up at: %s.bak\n", mods.Config.SettingsPath)
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
