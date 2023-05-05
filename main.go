package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
	flag "github.com/spf13/pflag"
)

type styles struct {
	error,
	code,
	codeComment,
	link,
	helpApp,
	helpFlag,
	helpDesc,
	cyclingChars lipgloss.Style
}

func makeStyles(r *lipgloss.Renderer) styles {
	return styles{
		error:        r.NewStyle().Foreground(lipgloss.Color("1")),
		code:         r.NewStyle().Foreground(lipgloss.Color("1")).Background(lipgloss.Color("237")).Padding(0, 1),
		codeComment:  r.NewStyle().Foreground(lipgloss.Color("244")),
		link:         r.NewStyle().Foreground(lipgloss.Color("10")).Underline(true),
		helpApp:      r.NewStyle().Foreground(lipgloss.Color("208")).Bold(true),
		helpFlag:     r.NewStyle().Foreground(lipgloss.Color("#41ffef")).Bold(true),
		helpDesc:     r.NewStyle().Foreground(lipgloss.Color("244")),
		cyclingChars: r.NewStyle().Foreground(lipgloss.Color("212")),
	}
}

// Build vars.
var (
	//nolint: gochecknoglobals
	version = "dev"
	commit  = ""
	date    = ""
	builtBy = ""
)

func usage() {
	r := lipgloss.DefaultRenderer()
	s := makeStyles(r)

	fmt.Printf("GPT on the command line. Built for pipelines.\n\n")
	fmt.Printf("Usage:\n  %s [OPTIONS] [PREFIX TERM]\n\n", s.helpApp.Render(filepath.Base(os.Args[0])))
	fmt.Println("Options:")
	flag.VisitAll(func(f *flag.Flag) {
		if f.Shorthand == "" {
			fmt.Printf(
				"  %-42s %s\n",
				s.helpFlag.Render("--"+f.Name),
				s.helpDesc.Render(f.Usage),
			)
		} else {
			fmt.Printf(
				"  %s, %-38s %s\n",
				s.helpFlag.Render("-"+f.Shorthand),
				s.helpFlag.Render("--"+f.Name),
				s.helpDesc.Render(f.Usage),
			)
		}
	})
	desc, example := randomExample()
	fmt.Printf(
		"\nExample:\n  %s\n  %s\n",
		s.codeComment.Render("# "+desc),
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

func main() {
	flag.Usage = usage
	flag.CommandLine.SortFlags = false
	config := newConfig()
	if config.Version {
		fmt.Println(buildVersion())
		os.Exit(0)
	}
	renderer := lipgloss.NewRenderer(os.Stderr, termenv.WithColorCache(true))
	opts := []tea.ProgramOption{tea.WithOutput(renderer.Output())}
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		opts = append(opts, tea.WithInput(noopRead{}))
	}
	p := tea.NewProgram(newMods(config, renderer), opts...)
	m, err := p.Run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
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
