package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Color Palette
	primaryColor   = lipgloss.Color("#7D56F4") // Purple
	secondaryColor = lipgloss.Color("#04B575") // Green
	accentColor    = lipgloss.Color("#FF5F87") // Pink/Red
	warningColor   = lipgloss.Color("#E5C07B") // Yellow
	mutedColor     = lipgloss.Color("#626262") // Gray
	subtleColor    = lipgloss.Color("#383838") // Darker gray
	borderColor    = lipgloss.Color("#444444")
	activeBorder   = lipgloss.Color("#7D56F4")
	titleBg        = lipgloss.Color("#5A3EBC")
	badgeBg        = lipgloss.Color("#2C2C3E")

	// Header Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(titleBg).
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

	// Pane Styles
	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 1)

	activePaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(activeBorder).
			Padding(0, 1)

	// Tab Styles
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primaryColor).
			Padding(0, 1).
			MarginRight(1)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Background(lipgloss.Color("#222222")).
				Padding(0, 1).
				MarginRight(1)

	// Badge Styles
	badgeOpen = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(secondaryColor).
			Bold(true).
			Padding(0, 1)

	badgeMerged = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primaryColor).
			Bold(true).
			Padding(0, 1)

	badgeClosed = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(accentColor).
			Bold(true).
			Padding(0, 1)

	badgeCached = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#3B82F6")).
			Bold(true).
			Padding(0, 1)

	badgeLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E0E0E0")).
			Background(badgeBg).
			Padding(0, 1).
			MarginRight(1)

	// Status and Help Bar
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
		Foreground(lipgloss.Color("#7D56F4")).
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
			Background(titleBg).
			Padding(0, 1).
			MarginBottom(1)
)
