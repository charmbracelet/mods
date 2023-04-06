package main

import (
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type quitMsg struct{}

var (
	modelRenderer = lipgloss.NewRenderer(os.Stderr,
		termenv.WithColorCache(true))
	spinnerStyle = modelRenderer.NewStyle().
			Foreground(lipgloss.Color("212"))
)

type Model struct {
	spinner  spinner.Model
	quitting bool
}

func (m Model) Init() tea.Cmd {
	return spinner.Tick
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case quitMsg:
		m.quitting = true
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	return m.spinner.View() + "Generating..."
}
