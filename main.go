package main

import (
	"fmt"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	flag "github.com/spf13/pflag"
)

// Renderers
var (
	outRenderer = lipgloss.DefaultRenderer()
	errRenderer = lipgloss.NewRenderer(os.Stderr, termenv.WithColorCache(true))
)

// Styles
var (
	errorStyle           = errRenderer.NewStyle().Foreground(lipgloss.Color("1"))
	codeStyle            = errRenderer.NewStyle().Foreground(lipgloss.Color("1")).Background(lipgloss.Color("237")).Padding(0, 1)
	codeCommentStyle     = outRenderer.NewStyle().Foreground(lipgloss.Color("244"))
	linkStyle            = outRenderer.NewStyle().Foreground(lipgloss.Color("10")).Underline(true)
	helpAppStyle         = outRenderer.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	helpFlagStyle        = outRenderer.NewStyle().Foreground(lipgloss.Color("#41ffef")).Bold(true)
	helpDescriptionStyle = outRenderer.NewStyle().Foreground(lipgloss.Color("244"))
)

// build vars
// nolint: gochecknoglobals
var (
	version = "dev"
	commit  = ""
	date    = ""
	builtBy = ""
)

func usage() {
	fmt.Printf("GPT on the command line. Built for pipelines.\n\n")
	fmt.Printf("Usage:\n  %s [OPTIONS] [PREFIX TERM]\n\n", helpAppStyle.Render(os.Args[0]))
	fmt.Println("Options:")
	flag.VisitAll(func(f *flag.Flag) {
		if f.Shorthand == "" {
			fmt.Printf(
				"  %-38s %s\n",
				helpFlagStyle.Render("--"+f.Name),
				helpDescriptionStyle.Render(f.Usage),
			)
		} else {
			fmt.Printf(
				"  %s, %-34s %s\n",
				helpFlagStyle.Render("-"+f.Shorthand),
				helpFlagStyle.Render("--"+f.Name),
				helpDescriptionStyle.Render(f.Usage),
			)
		}
	})
	desc, example := randomExample()
	fmt.Printf(
		"\nExample:\n  %s\n  %s\n",
		codeCommentStyle.Render("# "+desc),
		example,
	)
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

func main() {
	flag.Usage = usage
	flag.CommandLine.SortFlags = false
	config := newConfig()
	if config.Version {
		fmt.Println(buildVersion())
		os.Exit(0)
	}
	p := tea.NewProgram(
		newMods(config, config.AltSpinner),
		tea.WithOutput(errRenderer.Output()),
	)
	m, err := p.Run()
	if err != nil {
		panic(err)
	}
	if m.(mods).hadStdin == false && config.Prefix == "" {
		flag.Usage()
		os.Exit(0)
	}
	out := m.(mods).output
	if out != "" {
		fmt.Println(out)
	}
}
