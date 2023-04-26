package main

import (
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	charCyclingFPS     = time.Second / 22
	initialCharsLength = 10
)

var charRunes = []rune("0123456789abcdefABCDEF~!@#$£€%^&*()+=_")

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
	start time.Time
	chars []cyclingChar
	label []rune
}

func newCyclingChars() cyclingChars {
	c := cyclingChars{
		start: time.Now(),
		label: []rune(" " + spinnerLabel + "..."),
	}

	makeDelay := func(a int32, b time.Duration) time.Duration {
		return time.Duration(rand.Int31n(a)) * (time.Millisecond * b) //nolint:gosec
	}

	makeInitialDelay := func() time.Duration {
		return makeDelay(8, 80)
	}

	c.chars = make([]cyclingChar, initialCharsLength+len(c.label))

	// Initial characters that cycle forever.
	for i := 0; i < initialCharsLength; i++ {
		c.chars[i] = cyclingChar{
			finalValue:   -1,                 // cycle forever
			initialDelay: makeInitialDelay(), //nolint:gomnd
		}
	}

	// Label text that only cycles for a little while.
	for i, r := range c.label {
		c.chars[i+initialCharsLength] = cyclingChar{
			currentValue: '#',
			finalValue:   r,
			initialDelay: makeInitialDelay(), //nolint:gomnd
			lifetime:     makeDelay(5, 180),  //nolint:gomnd
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
	switch msg.(type) {
	case stepCharsMsg:
		for i, char := range c.chars {
			switch char.state(c.start) {
			case charInitialState:
				c.chars[i].currentValue = '#'
			case charCyclingState:
				c.chars[i].currentValue = char.randomRune()
			case charEndOfLifeState:
				c.chars[i].currentValue = char.finalValue
			}
		}
		return c, stepChars()
	default:
		return c, nil
	}
}

// View renders the animation.
func (c cyclingChars) View() string {
	var b strings.Builder
	for _, char := range c.chars {
		switch char.state(c.start) {
		case charInitialState:
			b.WriteString(spinnerStyle.Render(string(char.currentValue)))
		case charCyclingState:
			if char.finalValue < 0 {
				b.WriteString(spinnerStyle.Render(string(char.currentValue)))
				continue
			}
			b.WriteRune(char.currentValue)
		case charEndOfLifeState:
			b.WriteRune(char.currentValue)
		}
	}
	return b.String()
}
