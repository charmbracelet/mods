package main

import (
	"fmt"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
	flag "github.com/spf13/pflag"
)

const spinnerLabel = "Generating"

// Renderers.
var (
	outRenderer  = lipgloss.DefaultRenderer()
	errRenderer  = lipgloss.NewRenderer(os.Stderr, termenv.WithColorCache(true))
	spinnerStyle = errRenderer.NewStyle().Foreground(lipgloss.Color("212"))
)

// Styles.
var (
	errorStyle           = errRenderer.NewStyle().Foreground(lipgloss.Color("1"))
	codeStyle            = errRenderer.NewStyle().Foreground(lipgloss.Color("1")).Background(lipgloss.Color("237")).Padding(0, 1)
	codeCommentStyle     = outRenderer.NewStyle().Foreground(lipgloss.Color("244"))
	linkStyle            = outRenderer.NewStyle().Foreground(lipgloss.Color("10")).Underline(true)
	helpAppStyle         = outRenderer.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	helpFlagStyle        = outRenderer.NewStyle().Foreground(lipgloss.Color("#41ffef")).Bold(true)
	helpDescriptionStyle = outRenderer.NewStyle().Foreground(lipgloss.Color("244"))
)

// Build vars.
var (
	//nolint: gochecknoglobals
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
				"  %-42s %s\n",
				helpFlagStyle.Render("--"+f.Name),
				helpDescriptionStyle.Render(f.Usage),
			)
		} else {
			fmt.Printf(
				"  %s, %-38s %s\n",
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

type noopRead struct{}

func (nr noopRead) Read(_ []byte) (n int, err error) {
	return 0, nil
}

func init() {
	outRenderer.SetHasDarkBackground(true)
	errRenderer.SetHasDarkBackground(true)
}

func main() {
	flag.Usage = usage
	flag.CommandLine.SortFlags = false
	config := newConfig()
	if config.Version {
		fmt.Println(buildVersion())
		os.Exit(0)
	}
	opts := []tea.ProgramOption{tea.WithOutput(errRenderer.Output())}
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		opts = append(opts, tea.WithInput(noopRead{}))
	}
	p := tea.NewProgram(newMods(config), opts...)
	m, err := p.Run()
	if err != nil {
		panic(err)
	}
	mods := m.(*Mods)
	if mods.Input == "" && config.Prefix == "" {
		flag.Usage()
		os.Exit(0)
	}
	if mods.Error != nil {
		os.Exit(1)
	}
	fmt.Println(mods.FormattedOutput())
}
