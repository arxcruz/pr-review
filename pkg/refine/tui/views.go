package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the terminal UI according to current screen and modal states.
func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	var content string
	switch m.screen {
	case ScreenPicker:
		content = m.renderPickerView()
	case ScreenInterview:
		content = m.renderInterviewView()
	default:
		content = m.renderPickerView()
	}

	if m.resumeModal.Active {
		content = m.overlayModal(content, m.renderResumeModal())
	}

	return content
}

func (m Model) renderPickerView() string {
	var b strings.Builder

	// Header
	project := "Origin Project"
	if m.cfg != nil && m.cfg.Jira.OriginProject != "" {
		project = m.cfg.Jira.OriginProject
	}
	header := titleStyle.Render("Jira Refine") + " " +
		projectBadgeStyle.Render(project) + " " +
		headerInfoStyle.Render("Strategic Ticket Selection")
	b.WriteString(header + "\n\n")

	// Main content: loading or ticket list
	if m.loading {
		loadingText := fmt.Sprintf(" %s %s", m.spinner.View(), m.loadingMsg)
		b.WriteString(boxStyle.Width(m.width - 4).Height(m.height - 10).Render(loadingText))
		b.WriteString("\n\n")
	} else {
		// Table container
		tableStyle := boxStyle
		if m.pickerFocus == FocusTable {
			tableStyle = activeBoxStyle
		}
		tableView := tableStyle.Width(m.width - 4).Render(m.table.View())
		b.WriteString(tableView + "\n\n")
	}

	// Manual ticket input container
	inputBoxStyle := boxStyle
	inputHeader := "Manual Ticket Key Input"
	if m.pickerFocus == FocusInput {
		inputBoxStyle = activeBoxStyle
		inputHeader = "▶ Manual Ticket Key Input (Active - Enter to Select, Esc to unfocus)"
	}
	inputContent := fmt.Sprintf("%s\n%s",
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#D0D0D0")).Render(inputHeader),
		m.input.View(),
	)
	b.WriteString(inputBoxStyle.Width(m.width - 4).Render(inputContent) + "\n")

	// Status line
	statusLine := m.renderStatusLine()
	if statusLine != "" {
		b.WriteString(statusLine + "\n")
	}

	// Help / footer bar
	helpLine := m.renderPickerHelp()
	b.WriteString(statusBar.Width(m.width).Render(helpLine))

	return b.String()
}

func (m Model) renderInterviewView() string {
	var b strings.Builder

	key := "Unknown"
	summary := ""
	status := "new"
	rounds := 0
	if m.activeSnapshot != nil {
		key = m.activeSnapshot.Key
		summary = m.activeSnapshot.Ticket.Summary
		status = m.activeSnapshot.Status
		rounds = len(m.activeSnapshot.Rounds)
	}

	header := titleStyle.Render("Jira Refine") + " " +
		projectBadgeStyle.Render(key) + " " +
		headerInfoStyle.Render("Frontier Refinement Interview")
	b.WriteString(header + "\n\n")

	details := fmt.Sprintf(
		"Ticket:      %s\nSummary:     %s\nStatus:      %s\nRounds Done: %d\n\nTransition complete! Ready for Refinement Session.\nPress [esc] or [b] to return to Ticket Picker.",
		key, summary, status, rounds,
	)
	b.WriteString(boxStyle.Width(m.width - 4).Height(m.height - 8).Render(details) + "\n\n")

	// Status line
	statusLine := m.renderStatusLine()
	if statusLine != "" {
		b.WriteString(statusLine + "\n")
	}

	helpLine := fmt.Sprintf("%s %s  •  %s %s",
		helpKey.Render("[esc/b]"), helpDesc.Render("Back to Picker"),
		helpKey.Render("[q]"), helpDesc.Render("Quit"),
	)
	b.WriteString(statusBar.Width(m.width).Render(helpLine))

	return b.String()
}

func (m Model) renderResumeModal() string {
	key := m.resumeModal.Key
	snap := m.resumeModal.Snapshot
	status := "unknown"
	rounds := 0
	summary := ""
	if snap != nil {
		status = snap.Status
		rounds = len(snap.Rounds)
		summary = snap.Ticket.Summary
	}

	title := modalTitle.Render(fmt.Sprintf("Existing Refinement Session Found: %s", key))
	body := fmt.Sprintf(
		"Summary:         %s\nSession Status:  %s\nAnswered Rounds: %d\n\nWould you like to resume this session or start fresh?\n\n%s %s\n%s %s\n%s %s",
		summary,
		status,
		rounds,
		helpKey.Render("[r] / [Enter]"), helpDesc.Render("Resume existing session"),
		helpKey.Render("[f]"), helpDesc.Render("Start fresh (reset previous rounds)"),
		helpKey.Render("[esc] / [q]"), helpDesc.Render("Cancel and return to ticket picker"),
	)

	return modalBox.Width(64).Render(title + "\n\n" + body)
}

func (m Model) renderStatusLine() string {
	if m.statusMsg == "" {
		return ""
	}
	if m.statusIsErr {
		return statusError.Render("✖ " + m.statusMsg)
	}
	return statusSuccess.Render("✓ " + m.statusMsg)
}

func (m Model) renderPickerHelp() string {
	if m.pickerFocus == FocusInput {
		return fmt.Sprintf("%s %s  •  %s %s  •  %s %s",
			helpKey.Render("[Enter]"), helpDesc.Render("Submit Ticket Key"),
			helpKey.Render("[Esc]"), helpDesc.Render("Return to Ticket List"),
			helpKey.Render("[Ctrl+C]"), helpDesc.Render("Quit"),
		)
	}

	return fmt.Sprintf("%s %s  •  %s %s  •  %s %s  •  %s %s  •  %s %s  •  %s %s",
		helpKey.Render("[↑/↓]"), helpDesc.Render("Navigate"),
		helpKey.Render("[Enter]"), helpDesc.Render("Select Ticket"),
		helpKey.Render("[Tab / /]"), helpDesc.Render("Manual Key Input"),
		helpKey.Render("[r]"), helpDesc.Render("Refresh List"),
		helpKey.Render("[q]"), helpDesc.Render("Quit"),
		helpKey.Render("[Ctrl+C]"), helpDesc.Render("Exit"),
	)
}

// overlayModal places modal dialog in the center of background content
func (m Model) overlayModal(bg string, modal string) string {
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceChars(" "),
	)
}
