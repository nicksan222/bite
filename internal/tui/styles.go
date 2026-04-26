package tui

import "github.com/charmbracelet/lipgloss"

var (
	userStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7AA2F7"))

	assistantStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#9ECE6A"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F7768E"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565F89")).
			Italic(true)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#BB9AF7")).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("#414868")).
			Padding(0, 1)

	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7AA2F7")).
			Padding(0, 1)
)
