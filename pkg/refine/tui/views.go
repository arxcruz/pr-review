package tui

import (
	"fmt"
	"strings"

	"github.com/arxcruz/pr-review/pkg/session"
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
	case ScreenTree:
		content = m.renderTreeView()
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
	roundNum := 1
	if m.activeSnapshot != nil {
		key = m.activeSnapshot.Key
		roundNum = len(m.activeSnapshot.Rounds) + 1
	}

	header := titleStyle.Render("Jira Refine") + " " +
		projectBadgeStyle.Render(key) + " " +
		headerInfoStyle.Render(fmt.Sprintf("Frontier Refinement Interview (Round %d)", roundNum))
	b.WriteString(header + "\n\n")

	if m.loading {
		loadingText := fmt.Sprintf(" %s %s", m.spinner.View(), m.loadingMsg)
		b.WriteString(boxStyle.Width(m.width - 4).Height(m.height - 10).Render(loadingText))
		b.WriteString("\n\n")
	} else if m.activeSnapshot != nil && len(m.activeSnapshot.CurrentFrontier) > 0 {
		var content strings.Builder
		totalQ := len(m.activeSnapshot.CurrentFrontier)
		content.WriteString(lipgloss.NewStyle().Bold(true).Render(
			fmt.Sprintf("=== Refinement Round %d (%d Frontier Questions) ===\n\n", roundNum, totalQ),
		))

		for i, q := range m.activeSnapshot.CurrentFrontier {
			cursor := "  "
			titleSt := questionTitle
			if i == m.interviewIndex {
				cursor = "▶ "
				titleSt = questionActiveTitle
			}

			content.WriteString(cursor + titleSt.Render(fmt.Sprintf("[%s] %s", q.ID, q.Title)) + "\n")
			if q.Explanation != "" {
				content.WriteString("    " + explanationStyle.Render("Why: "+q.Explanation) + "\n")
			}
			if len(q.Options) > 0 {
				content.WriteString("    Options:\n")
				for idx, opt := range q.Options {
					content.WriteString(fmt.Sprintf("      [%d] %s\n", idx+1, opt))
				}
			}
			if q.Recommendation != "" {
				content.WriteString("    " + recommendationStyle.Render("Recommendation: "+q.Recommendation) + "\n")
			}

			if i == m.interviewIndex && m.interviewEditing {
				content.WriteString("    Answer: " + m.interviewInput.View() + "\n")
			} else if q.Answer != "" {
				content.WriteString("    Answer: " + answerStyle.Render("✓ "+q.Answer) + "\n")
			} else {
				content.WriteString("    Answer: " + explanationStyle.Render("(unanswered)") + "\n")
			}
			content.WriteString("\n")
		}

		vp := m.viewport
		vp.SetContent(content.String())
		b.WriteString(boxStyle.Width(m.width - 4).Height(m.height - 12).Render(vp.View()) + "\n")

		actionButtons := fmt.Sprintf(" %s    %s",
			activeBoxStyle.Render("[ Submit Round (Ctrl+S) ]"),
			boxStyle.Render("[ Finalize Refinement (Ctrl+F) ]"),
		)
		b.WriteString(actionButtons + "\n\n")
	} else if m.activeSnapshot != nil && m.activeSnapshot.Status == session.StatusFinalized {
		var content strings.Builder
		content.WriteString(titleStyle.Render("Refinement Session Finalized!") + "\n\n")
		content.WriteString(fmt.Sprintf("Ticket:   %s\nSummary:  %s\nStatus:   %s\n\n",
			key, m.activeSnapshot.Ticket.Summary, m.activeSnapshot.Status))
		if m.activeSnapshot.Tree != nil {
			content.WriteString(fmt.Sprintf("Decomposition Tree (%d Epics):\n\n", len(m.activeSnapshot.Tree.Epics)))
			for _, epic := range m.activeSnapshot.Tree.Epics {
				content.WriteString(fmt.Sprintf("  • [%s] %s\n", epic.Key, epic.Title))
				for _, task := range epic.Tasks {
					content.WriteString(fmt.Sprintf("      - %s\n", task.Title))
				}
			}
		}
		if m.planFile != "" {
			content.WriteString(fmt.Sprintf("\nPlan saved to: %s\n", m.planFile))
		}
		content.WriteString("\nRefinement complete! Press [esc] or [b] to return to Ticket Picker, or [q] to quit.")
		b.WriteString(boxStyle.Width(m.width - 4).Height(m.height - 10).Render(content.String()) + "\n\n")
	} else {
		details := fmt.Sprintf(
			"Ticket:      %s\nSummary:     %s\nStatus:      %s\n\nAll frontier questions resolved or ready for frontier generation.\nPress [ctrl+f] to finalize decomposition, or [r] to generate frontier.",
			key,
			m.activeSnapshot.Ticket.Summary,
			m.activeSnapshot.Status,
		)
		b.WriteString(boxStyle.Width(m.width - 4).Height(m.height - 10).Render(details) + "\n\n")
	}

	// Status line
	statusLine := m.renderStatusLine()
	if statusLine != "" {
		b.WriteString(statusLine + "\n")
	}

	helpLine := m.renderInterviewHelp()
	b.WriteString(statusBar.Width(m.width).Render(helpLine))

	return b.String()
}

func (m Model) renderInterviewHelp() string {
	if m.interviewEditing {
		return fmt.Sprintf("%s %s  •  %s %s  •  %s %s",
			helpKey.Render("[Enter]"), helpDesc.Render("Confirm Answer"),
			helpKey.Render("[Esc]"), helpDesc.Render("Cancel Edit"),
			helpKey.Render("[Ctrl+C]"), helpDesc.Render("Quit"),
		)
	}

	return fmt.Sprintf("%s %s  •  %s %s  •  %s %s  •  %s %s  •  %s %s  •  %s %s  •  %s %s",
		helpKey.Render("[↑/↓/j/k]"), helpDesc.Render("Navigate"),
		helpKey.Render("[Enter/y]"), helpDesc.Render("Accept Recommendation"),
		helpKey.Render("[e]"), helpDesc.Render("Custom Answer"),
		helpKey.Render("[Ctrl+S]"), helpDesc.Render("Submit Round"),
		helpKey.Render("[Ctrl+F]"), helpDesc.Render("Finalize"),
		helpKey.Render("[esc/b]"), helpDesc.Render("Back"),
		helpKey.Render("[q]"), helpDesc.Render("Quit"),
	)
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

func (m Model) renderTreeView() string {
	var b strings.Builder

	key := "Unknown"
	if m.activeSnapshot != nil {
		key = m.activeSnapshot.Key
	}

	header := titleStyle.Render("Jira Refine") + " " +
		projectBadgeStyle.Render(key) + " " +
		headerInfoStyle.Render("Decomposition Tree & Plan Editor")
	b.WriteString(header + "\n\n")

	items := m.TreeItems()
	if len(items) == 0 {
		emptyMsg := "No decomposition tree available. Press [esc] or [b] to return to interview."
		b.WriteString(boxStyle.Width(m.width - 4).Height(m.height - 10).Render(emptyMsg) + "\n\n")
		b.WriteString(statusBar.Width(m.width).Render(m.renderTreeHelp()))
		return b.String()
	}

	var treeContent strings.Builder
	treeContent.WriteString(lipgloss.NewStyle().Bold(true).Render("=== Decomposition Tree Hierarchy ===\n\n"))

	for i, item := range items {
		cursor := "  "
		itemStyle := lipgloss.NewStyle()
		if i == m.treeIndex {
			cursor = "▶ "
			itemStyle = itemStyle.Bold(true).Foreground(primaryColor)
		}

		check := "[x]"
		if item.Excluded {
			check = "[ ]"
			itemStyle = itemStyle.Faint(true)
		}

		projBadge := fmt.Sprintf("[%s]", item.DeliveryProject)
		if item.DeliveryProject == "" {
			projBadge = "[Unassigned]"
		}

		if item.Kind == TreeItemEpic {
			treeContent.WriteString(fmt.Sprintf("%s%s %s %s %s\n",
				cursor,
				check,
				lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#58A6FF")).Render(fmt.Sprintf("[%s]", item.ID)),
				itemStyle.Render(item.Title),
				lipgloss.NewStyle().Foreground(lipgloss.Color("#8B949E")).Render(projBadge),
			))
		} else {
			treeContent.WriteString(fmt.Sprintf("%s  └─ %s %s %s %s\n",
				cursor,
				check,
				lipgloss.NewStyle().Foreground(lipgloss.Color("#7EE787")).Render(fmt.Sprintf("[%s]", item.ID)),
				itemStyle.Render(item.Title),
				lipgloss.NewStyle().Foreground(lipgloss.Color("#8B949E")).Render(projBadge),
			))
		}
	}

	treeContent.WriteString("\n")

	// Selected Item Details
	if m.treeIndex >= 0 && m.treeIndex < len(items) {
		curr := items[m.treeIndex]
		treeContent.WriteString(lipgloss.NewStyle().Bold(true).Render("── Item Details ──\n"))
		kindStr := "Epic"
		if curr.Kind == TreeItemTask {
			kindStr = "Task/Story"
		}
		treeContent.WriteString(fmt.Sprintf("Type:        %s (%s)\n", kindStr, curr.ID))
		treeContent.WriteString(fmt.Sprintf("Title:            %s\n", curr.Title))
		treeContent.WriteString(fmt.Sprintf("Delivery Project: %s\n", curr.DeliveryProject))
		if curr.Description != "" {
			treeContent.WriteString(fmt.Sprintf("Description:      %s\n", curr.Description))
		}
		if len(curr.DependsOn) > 0 {
			treeContent.WriteString(fmt.Sprintf("Depends On:       %s\n", strings.Join(curr.DependsOn, ", ")))
		}
	}

	vp := m.viewport
	vp.SetContent(treeContent.String())
	treeHeight := m.height - 12
	if m.treeEditing {
		treeHeight -= 4
	}
	if treeHeight < 5 {
		treeHeight = 5
	}
	b.WriteString(boxStyle.Width(m.width - 4).Height(treeHeight).Render(vp.View()) + "\n")

	if m.treeEditing && m.treeIndex >= 0 && m.treeIndex < len(items) {
		curr := items[m.treeIndex]
		fieldLabel := strings.Title(string(m.treeEditField))
		if m.treeEditField == TreeEditFieldDeliveryProject {
			fieldLabel = "Delivery Project"
		}
		editPrompt := fmt.Sprintf("▶ Editing %s for [%s]:", fieldLabel, curr.ID)
		editBox := fmt.Sprintf("%s\n%s\n%s",
			lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render(editPrompt),
			m.treeInput.View(),
			explanationStyle.Render("Press [Enter] to Save, [Esc] to Cancel"),
		)
		b.WriteString(activeBoxStyle.Width(m.width - 4).Render(editBox) + "\n")
	}

	// Action buttons
	actionButtons := fmt.Sprintf(" %s    %s",
		boxStyle.Render("[ Return to Interview (B/Esc) ]"),
		activeBoxStyle.Render("[ Proceed to Jira Sync (S) ]"),
	)
	b.WriteString(actionButtons + "\n\n")

	// Status line
	statusLine := m.renderStatusLine()
	if statusLine != "" {
		b.WriteString(statusLine + "\n")
	}

	b.WriteString(statusBar.Width(m.width).Render(m.renderTreeHelp()))
	return b.String()
}

func (m Model) renderTreeHelp() string {
	if m.treeEditing {
		return fmt.Sprintf("%s %s  •  %s %s  •  %s %s",
			helpKey.Render("[Enter]"), helpDesc.Render("Save"),
			helpKey.Render("[Esc]"), helpDesc.Render("Cancel"),
			helpKey.Render("[Ctrl+C]"), helpDesc.Render("Quit"),
		)
	}

	return fmt.Sprintf("%s %s  •  %s %s  •  %s %s  •  %s %s  •  %s %s  •  %s %s  •  %s %s  •  %s %s",
		helpKey.Render("[↑/↓/j/k]"), helpDesc.Render("Navigate"),
		helpKey.Render("[Space/x]"), helpDesc.Render("Toggle [x]/[ ]"),
		helpKey.Render("[e]"), helpDesc.Render("Edit Title"),
		helpKey.Render("[d]"), helpDesc.Render("Edit Desc"),
		helpKey.Render("[p]"), helpDesc.Render("Project"),
		helpKey.Render("[s]"), helpDesc.Render("Sync"),
		helpKey.Render("[esc/b]"), helpDesc.Render("Interview"),
		helpKey.Render("[q]"), helpDesc.Render("Quit"),
	)
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
