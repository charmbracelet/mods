package main

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

var ellipsisSpinner = spinner.Spinner{
	Frames: []string{"", ".", "..", "..."},
	FPS:    time.Second / 3, //nolint:gomnd
}

type ellipsis struct {
	head spinner.Model
	tail spinner.Model
}

func newEllipsis() tea.Model {
	return ellipsis{
		head: spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(spinnerStyle)),
		tail: spinner.New(spinner.WithSpinner(ellipsisSpinner)),
	}
}

func (s ellipsis) Init() tea.Cmd {
	return tea.Batch(s.head.Tick, s.tail.Tick)
}

func (s ellipsis) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmds = make([]tea.Cmd, 2) //nolint:gomnd
	)
	s.head, cmds[0] = s.head.Update(msg)
	s.tail, cmds[1] = s.tail.Update(msg)
	return s, tea.Batch(cmds...)
}

func (s ellipsis) View() string {
	return s.head.View() + spinnerLabel + s.tail.View()
}
