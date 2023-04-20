package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
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
	spinnerStyle         = errRenderer.NewStyle().Foreground(lipgloss.Color("212"))
	errorStyle           = errRenderer.NewStyle().Foreground(lipgloss.Color("1"))
	codeStyle            = errRenderer.NewStyle().Foreground(lipgloss.Color("1")).Background(lipgloss.Color("237")).Padding(0, 1)
	codeCommentStyle     = outRenderer.NewStyle().Foreground(lipgloss.Color("244"))
	linkStyle            = outRenderer.NewStyle().Foreground(lipgloss.Color("10")).Underline(true)
	helpAppStyle         = outRenderer.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	helpFlagStyle        = outRenderer.NewStyle().Foreground(lipgloss.Color("#41ffef")).Bold(true)
	helpDescriptionStyle = outRenderer.NewStyle().Foreground(lipgloss.Color("244"))
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

func main() {
	flag.Usage = usage
	flag.CommandLine.SortFlags = false
	config := newConfig()
	flag.Parse()
	prefix := strings.Join(flag.Args(), " ")
	if *config.Markdown {
		prefix = fmt.Sprintf("%s Format output as Markdown.", prefix)
	}
	spinner := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(spinnerStyle))
	p := tea.NewProgram(Model{
		config:  config,
		prefix:  prefix,
		spinner: spinner,
	},
		tea.WithOutput(errRenderer.Output()),
	)
	m, err := p.Run()
	if err != nil {
		panic(err)
	}
	if m.(Model).hadStdin == false && prefix == "" {
		flag.Usage()
		os.Exit(0)
	}
	out := m.(Model).output
	if out != "" {
		fmt.Println(m.(Model).output)
	}
}
