package tui

import "github.com/charmbracelet/lipgloss"

// Palette: Tokyo Night-ish.
//   user      = #7AA2F7 (blue)
//   assistant = #9ECE6A (green)
//   tool      = #BB9AF7 (purple)
//   accent    = #7DCFFF (cyan)
//   subtle    = #565F89 (muted blue-grey)
//   error     = #F7768E (pink-red)

var (
	assistantStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#9ECE6A"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F7768E")).
			Bold(true)

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

	// userBarStyle and assistantBarStyle are applied per-line as a leading
	// "bar" character. Drawing the bar manually (one styled rune per line)
	// instead of wrapping the whole block in lipgloss.Border avoids
	// lipgloss re-flowing already-styled content — the latter mangles
	// glamour-rendered ANSI and leaks raw control chars (\x01, \x02…) into
	// the transcript on some terminals.
	userBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7AA2F7"))

	userLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7AA2F7"))

	userBodyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C0CAF5"))

	assistantBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9ECE6A"))

	assistantLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#9ECE6A"))

	// toolStepStyle renders the "thinking step" lines under an assistant
	// turn (one per tool the model invoked).
	toolStepRunningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7DCFFF")).
				Italic(true)

	toolStepDoneStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#BB9AF7"))

	toolStepResultStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#565F89")).
				Italic(true)
)
