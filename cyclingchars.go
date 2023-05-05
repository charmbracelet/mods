package main

import (
	"math/rand"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/termenv"
)

const (
	cyclingCharsLabel = "Generating"
	charCyclingFPS    = time.Second / 22
	maxCyclingChars   = 120
)

var (
	charRunes = []rune("0123456789abcdefABCDEF~!@#$£€%^&*()+=_")

	ellipsisSpinner = spinner.Spinner{
		Frames: []string{"", ".", "..", "..."},
		FPS:    time.Second / 3, //nolint:gomnd
	}
)

type charState int

const (
	charInitialState charState = iota
	charCyclingState
	charEndOfLifeState
)

// cyclingChar is a single animated character.
type cyclingChar struct {
	finalValue   rune // if < 0 cycle forever
	currentValue rune
	initialDelay time.Duration
	lifetime     time.Duration
}

func (c cyclingChar) randomRune() rune {
	return (charRunes)[rand.Intn(len(charRunes))] //nolint:gosec
}

func (c cyclingChar) state(start time.Time) charState {
	now := time.Now()
	if now.Before(start.Add(c.initialDelay)) {
		return charInitialState
	}
	if c.finalValue > 0 && now.After(start.Add(c.initialDelay)) {
		return charEndOfLifeState
	}
	return charCyclingState
}

type stepCharsMsg struct{}

func stepChars() tea.Cmd {
	return tea.Tick(charCyclingFPS, func(_ time.Time) tea.Msg {
		return stepCharsMsg{}
	})
}

// cyclingChars is the model that manages the animation that displays while the
// output is being generated.
type cyclingChars struct {
	start           time.Time
	chars           []cyclingChar
	ramp            []lipgloss.Style
	label           []rune
	ellipsis        spinner.Model
	ellipsisStarted bool
	styles          styles
}

func newCyclingChars(initialCharsSize uint, r *lipgloss.Renderer, s styles) cyclingChars {
	n := int(initialCharsSize)
	if n > maxCyclingChars {
		n = maxCyclingChars
	}

	gap := " "
	if n == 0 {
		gap = ""
	}

	c := cyclingChars{
		start:    time.Now(),
		label:    []rune(gap + cyclingCharsLabel),
		ellipsis: spinner.New(spinner.WithSpinner(ellipsisSpinner)),
		styles:   s,
	}

	// If we're in truecolor mode (and there are enough cycling characters)
	// color the cycling characters with a gradient ramp.
	const (
		minRampSize = 3
		startColor  = "#F967DC"
		endColor    = "#6B50FF"
	)
	if n >= minRampSize && r.ColorProfile() == termenv.TrueColor {
		c.ramp = make([]lipgloss.Style, n)
		for i := range c.ramp {
			start, _ := colorful.Hex(startColor)
			end, _ := colorful.Hex(endColor)
			step := start.BlendLuv(end, float64(i)/float64(n))
			c.ramp[i] = r.NewStyle().Foreground(lipgloss.Color(step.Hex()))
		}
	}

	makeDelay := func(a int32, b time.Duration) time.Duration {
		return time.Duration(rand.Int31n(a)) * (time.Millisecond * b) //nolint:gosec
	}

	makeInitialDelay := func() time.Duration {
		return makeDelay(8, 60) //nolint:gomnd
	}

	c.chars = make([]cyclingChar, n+len(c.label))

	// Initial characters that cycle forever.
	for i := 0; i < n; i++ {
		c.chars[i] = cyclingChar{
			finalValue:   -1, // cycle forever
			initialDelay: makeInitialDelay(),
		}
	}

	// Label text that only cycles for a little while.
	for i, r := range c.label {
		c.chars[i+n] = cyclingChar{
			finalValue:   r,
			initialDelay: makeInitialDelay(),
			lifetime:     makeDelay(5, 180), //nolint:gomnd
		}
	}

	return c
}

// Init initializes the animation.
func (c cyclingChars) Init() tea.Cmd {
	return stepChars()
}

// Update handles messages.
func (c cyclingChars) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.(type) {
	case stepCharsMsg:
		for i, char := range c.chars {
			switch char.state(c.start) {
			case charInitialState:
				c.chars[i].currentValue = '.'
			case charCyclingState:
				c.chars[i].currentValue = char.randomRune()
			case charEndOfLifeState:
				c.chars[i].currentValue = char.finalValue
			}
		}

		if !c.ellipsisStarted {
			var eol int
			for _, char := range c.chars {
				if char.state(c.start) == charEndOfLifeState {
					eol++
				}
			}
			if eol == len(c.label) {
				// If our entire label has reached end of life, start the
				// ellipsis "spinner" after a short pause.
				c.ellipsisStarted = true
				cmd = tea.Tick(time.Millisecond*220, func(_ time.Time) tea.Msg { //nolint:gomnd
					return c.ellipsis.Tick()
				})
			}
		}

		return c, tea.Batch(stepChars(), cmd)
	case spinner.TickMsg:
		var cmd tea.Cmd
		c.ellipsis, cmd = c.ellipsis.Update(msg)
		return c, cmd
	default:
		return c, nil
	}
}

// View renders the animation.
func (c cyclingChars) View() string {
	var (
		b strings.Builder
		s = &c.styles.cyclingChars
	)
	for i, char := range c.chars {
		switch char.state(c.start) {
		case charInitialState:
			b.WriteString(s.Render(string(char.currentValue)))
		case charCyclingState:
			if char.finalValue < 0 {
				if len(c.ramp) > 0 && i < len(c.ramp) {
					s = &c.ramp[i]
				}
				b.WriteString(s.Render(string(char.currentValue)))
				continue
			}
			b.WriteRune(char.currentValue)
		case charEndOfLifeState:
			b.WriteRune(char.currentValue)
		}
	}
	return b.String() + c.ellipsis.View()
}
