package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	primaryColor   = lipgloss.Color("#7D56F4") // Purple
	secondaryColor = lipgloss.Color("#04B575") // Green
	accentColor    = lipgloss.Color("#FF5F87") // Red / Pink
	warningColor   = lipgloss.Color("#E5C07B") // Yellow
	mutedColor     = lipgloss.Color("#626262") // Gray
	borderColor    = lipgloss.Color("#444444")
	activeBorder   = lipgloss.Color("#7D56F4")
	headerBg       = lipgloss.Color("#5A3EBC")

	// Header Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(headerBg).
			Padding(0, 1)

	headerInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D0D0D0")).
			Padding(0, 1)

	projectBadgeStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(primaryColor).
				Padding(0, 1).
				MarginRight(1)

	// Box / Container Styles
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 1)

	activeBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(activeBorder).
			Padding(0, 1)

	// Status & Help Bar
	statusBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D0D0D0")).
			Background(lipgloss.Color("#1E1E2E")).
			Padding(0, 1)

	statusSuccess = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	statusError = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)

	helpKey = lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true)

	helpDesc = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	// Modal dialog
	modalBox = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2)

	modalTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(headerBg).
			Padding(0, 1).
			MarginBottom(1)
)
