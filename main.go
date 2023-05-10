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

func makeGradientText(baseStyle lipgloss.Style, str string) string {
	const minSize = 3
	if len(str) < minSize {
		return str
	}
	b := strings.Builder{}
	runes := []rune(str)
	for i, c := range makeGradientRamp(len(str)) {
		b.WriteString(baseStyle.Copy().Foreground(c).Render(string(runes[i])))
	}
	return b.String()
}

func usage() {
	r := lipgloss.DefaultRenderer()
	s := makeStyles(r)
	appName := filepath.Base(os.Args[0])

	if r.ColorProfile() == termenv.TrueColor {
		appName = makeGradientText(s.appName, appName)
	}

	fmt.Printf("GPT on the command line. Built for pipelines.\n\n")
	fmt.Printf(
		"Usage:\n  %s %s\n\n",
		appName,
		s.cliArgs.Render("[OPTIONS] [PREFIX TERM]"),
	)
	fmt.Println("Options:")
	flag.VisitAll(func(f *flag.Flag) {
		if f.Shorthand == "" {
			fmt.Printf(
				"  %-42s %s\n",
				s.flag.Render("--"+f.Name),
				s.flagDesc.Render(f.Usage),
			)
		} else {
			fmt.Printf(
				"  %s%s %-38s %s\n",
				s.flag.Render("-"+f.Shorthand),
				s.flagComma,
				s.flag.Render("--"+f.Name),
				s.flagDesc.Render(f.Usage),
			)
		}
	})
	desc, example := randomExample()
	fmt.Printf(
		"\nExample:\n  %s\n  %s\n",
		s.comment.Render("# "+desc),
		cheapHighlighting(s, example),
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
		opts = append(opts, tea.WithInput(nil))
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
