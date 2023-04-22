package main

import (
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Runes to use while animating to the final state.
var (
	oneCellRunes = []rune("0123456789abcdefABCDEF~!@#$£€%^&*()+=_")
	twoCellRunes = []rune("アイウエオカキクケコガギグゲゴサシスセソザジズゼゾタチツテトダヂヅデドナニヌネノハヒフへホバビブベボパピプペポマミムメモヤユヨラリルレロワヲン")
	runeSets     = [][]rune{oneCellRunes, oneCellRunes, oneCellRunes, twoCellRunes} // dups for cheap weighting
)

// Styles.
var (
	initialCharStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	prefixStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
)

// charState indicates the lifetime state of a character.
type charState int

// Character states.
const (
	charUnbornState charState = iota
	charNewbornState
	charCyclingState
	charEndOfLifeState
)

// animChar is a single character in the animation
type animChar struct {
	currentValue rune
	finalValue   rune // if less than zero cycle forever
	runes        *[]rune
	birthDelay   time.Duration
	initialDelay time.Duration
	lifetime     time.Duration
}

// randomRune returns an appropriate random rune for a character.
func (c animChar) randomRune() rune {
	return (*c.runes)[rand.Intn(len(*c.runes))]
}

// state returns the character's current state.
func (c animChar) state(start time.Time) charState {
	now := time.Now()
	if now.Before(start.Add(c.birthDelay)) {
		return charUnbornState
	}
	if now.Before(start.Add(c.birthDelay).Add(c.initialDelay)) {
		return charNewbornState
	}
	if c.finalValue > 0 && now.After(start.Add(c.birthDelay).Add(c.initialDelay)) {
		return charEndOfLifeState
	}
	return charCyclingState
}

// animStepMsg signals to step the animation.
type animStepMsg struct{}

// anim manages the animation that displays while the output is being generated.
type anim struct {
	start      time.Time
	chars      []animChar
	fps        time.Duration
	label      []rune
	prefixSize int
}

// new anim initializes the anim model.
func newAnim() anim {
	const label = "Generating..."

	c := anim{
		start:      time.Now(),
		fps:        time.Second / 22,
		label:      []rune(" " + label),
		prefixSize: 10,
	}

	makeDelay := func(a int32, b time.Duration) time.Duration {
		return time.Duration(rand.Int31n(a)) * (time.Millisecond * b)
	}

	c.chars = make([]animChar, c.prefixSize+len(c.label))

	// Prefix characters that cyclce forever.
	for i := 0; i < c.prefixSize; i++ {
		c.chars[i] = animChar{
			finalValue:   -1, // cycle forever
			runes:        &runeSets[rand.Intn(len(runeSets))],
			birthDelay:   makeDelay(25, 20),
			initialDelay: makeDelay(5, 100),
		}
	}

	// Label text that only cycles for a little while.
	for i, r := range c.label {
		c.chars[i+c.prefixSize] = animChar{
			currentValue: '#',
			finalValue:   r,
			runes:        &oneCellRunes,
			birthDelay:   makeDelay(2, 100),
			lifetime:     makeDelay(5, 180),
			initialDelay: makeDelay(5, 100),
		}
	}

	return c
}

// Init initializes the animation.
func (a anim) Init() tea.Cmd {
	return a.step()
}

// Update handles messages.
func (a anim) Update(msg tea.Msg) (anim, tea.Cmd) {
	switch msg.(type) {
	case animStepMsg:
		for i, c := range a.chars {
			switch c.state(a.start) {
			case charUnbornState:
				continue
			case charNewbornState:
				a.chars[i].currentValue = '#'
			case charCyclingState:
				a.chars[i].currentValue = c.randomRune()
			case charEndOfLifeState:
				a.chars[i].currentValue = c.finalValue
			}
		}
		return a, a.step()
	default:
		return a, nil
	}
}

// View renders the animation.
func (a anim) View() string {
	var b strings.Builder
	for _, c := range a.chars {
		switch c.state(a.start) {
		case charUnbornState:
			continue
		case charNewbornState:
			b.WriteString(initialCharStyle.Render(string(c.currentValue)))
		case charCyclingState:
			if c.finalValue < 0 {
				b.WriteString(prefixStyle.Render(string(c.currentValue)))
				continue
			}
			b.WriteRune(c.currentValue)
		case charEndOfLifeState:
			b.WriteRune(c.currentValue)
		}
	}
	return b.String()
}

// step steps the animation one frame.
func (a anim) step() tea.Cmd {
	return tea.Tick(a.fps, func(t time.Time) tea.Msg {
		return animStepMsg{}
	})
}
