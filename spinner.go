package main

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const spinnerLabel = "Generating"

var (
	spinnerStyle = errRenderer.NewStyle().Foreground(lipgloss.Color("212"))
	ellipsis     = spinner.Spinner{
		Frames: []string{"", ".", "..", "..."},
		FPS:    time.Second / 3, //nolint:gomnd
	}
)

type simpleSpinner struct {
	head spinner.Model
	tail spinner.Model
}

func newSimpleSpinner() tea.Model {
	return simpleSpinner{
		head: spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(spinnerStyle)),
		tail: spinner.New(spinner.WithSpinner(ellipsis)),
	}
}

func (s simpleSpinner) Init() tea.Cmd {
	return tea.Batch(s.head.Tick, s.tail.Tick)
}

func (s simpleSpinner) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmds = make([]tea.Cmd, 2) //nolint:gomnd
	)
	s.head, cmds[0] = s.head.Update(msg)
	s.tail, cmds[1] = s.tail.Update(msg)
	return s, tea.Batch(cmds...)
}

func (s simpleSpinner) View() string {
	return s.head.View() + spinnerLabel + s.tail.View()
}
