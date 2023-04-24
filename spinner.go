package main

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	spinnerLabel = " Generating..."
	spinnerFPS   = time.Second / 10
)

var (
	spinnerRunes = []rune("⣾⣽⣻⢿⡿⣟⣯⣷")
	spinnerStyle = errRenderer.NewStyle().Foreground(lipgloss.Color("212"))
)

type stepSpinnerMsg struct{}

func stepSpinner() tea.Cmd {
	return tea.Tick(spinnerFPS, func(t time.Time) tea.Msg {
		return stepSpinnerMsg{}
	})
}

type spinner int

// Init initializes the animation.
func (s spinner) Init() tea.Cmd {
	return stepSpinner()
}

// Update handles messages.
func (s spinner) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case stepSpinnerMsg:
		s++
		if int(s) > len(spinnerRunes)-1 {
			s = 0
		}
		return s, stepSpinner()
	default:
		return s, nil
	}
}

// View renders the animation.
func (s spinner) View() string {
	var b strings.Builder
	b.WriteString(spinnerStyle.Render(string(spinnerRunes[s])))
	b.WriteString(spinnerLabel)
	return b.String()
}
