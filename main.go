package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
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
	error,
	code,
	codeComment,
	link,
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
		helpFlag:     r.NewStyle().Foreground(lipgloss.Color("#41ffef")).Bold(true),
		helpDesc:     r.NewStyle().Foreground(lipgloss.Color("244")),
		cyclingChars: r.NewStyle().Foreground(lipgloss.Color("212")),
	}
}

func makeGradientRamp(length int) []lipgloss.Color {
	const startColor = "#F967DC"
	const endColor = "#6B50FF"
	var (
		c        = make([]lipgloss.Color, length)
		start, _ = colorful.Hex(startColor)
		end, _   = colorful.Hex(endColor)
	)
	for i := 0; i < length; i++ {
		step := start.BlendLuv(end, float64(i)/float64(length))
		c[i] = lipgloss.Color(step.Hex())
	}
	return c
}

func makeGradientText(r *lipgloss.Renderer, str string) string {
	const minSize = 3
	if len(str) < minSize {
		return str
	}
	b := strings.Builder{}
	runes := []rune(str)
	for i, c := range makeGradientRamp(len(str)) {
		b.WriteString(r.NewStyle().Foreground(c).Render(string(runes[i])))
	}
	return b.String()
}

func usage() {
	r := lipgloss.DefaultRenderer()
	s := makeStyles(r)
	appName := makeGradientText(r, filepath.Base(os.Args[0]))

	fmt.Printf("GPT on the command line. Built for pipelines.\n\n")
	fmt.Printf("Usage:\n  %s [OPTIONS] [PREFIX TERM]\n\n", string(appName))
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
	config, err := newConfig()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
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
