package tui

import (
	"fmt"
	"strings"

	"github.com/arxcruz/pr-review/pkg/refine"
	"github.com/arxcruz/pr-review/pkg/session"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// truncateTitle shortens title to fit within width columns, appending an
// ellipsis when truncated, so a rendered row never wraps onto a second line.
func truncateTitle(title string, width int) string {
	if width < 1 {
		return ""
	}
	return runewidth.Truncate(title, width, "…")
}

// titleBudget returns how many columns remain for a row's title once its
// rendered prefix and suffix are subtracted from the available row width.
func titleBudget(totalWidth int, prefix, suffix string) int {
	return totalWidth - lipgloss.Width(prefix) - lipgloss.Width(suffix)
}

// View renders the terminal UI according to current screen and modal states.
func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	if m.width < minTermWidth || m.height < minTermHeight {
		return renderTooSmallView(m.width, m.height)
	}

	var content string
	switch m.screen {
	case ScreenPicker:
		content = m.renderPickerView()
	case ScreenOverview:
		content = m.renderOverviewView()
	case ScreenInterview:
		content = m.renderInterviewView()
	case ScreenTree:
		content = m.renderTreeView()
	case ScreenSync:
		content = m.renderSyncView()
	default:
		content = m.renderPickerView()
	}

	if m.resumeModal.Active {
		content = m.overlayModal(content, m.renderResumeModal())
	}

	return content
}

// renderTooSmallView is shown instead of the normal layout when the
// terminal is smaller than minTermWidth x minTermHeight, so the layout
// never has to squeeze a fixed set of sections into a budget it wasn't
// designed for.
func renderTooSmallView(width, height int) string {
	msg := fmt.Sprintf(
		"Terminal too small\n\nJira Refine needs at least %dx%d, current size is %dx%d.\nPlease resize your terminal.",
		minTermWidth, minTermHeight, width, height,
	)
	if width <= 0 || height <= 0 {
		return msg
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render(msg))
}

func (m Model) renderPickerView() string {
	// Header
	project := "Origin Project"
	if m.cfg != nil && m.cfg.Jira.OriginProject != "" {
		project = m.cfg.Jira.OriginProject
	}
	header := titleStyle.Render("Jira Refine") + " " +
		projectBadgeStyle.Render(project) + " " +
		headerInfoStyle.Render("Strategic Ticket Selection")
	head := header + "\n\n"

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
	inputStr := inputBoxStyle.Width(m.width-4).Render(inputContent) + "\n"

	// Status line
	statusStr := ""
	if statusLine := m.renderStatusLine(); statusLine != "" {
		statusStr = statusLine + "\n"
	}

	// Help / footer bar
	helpStr := statusBar.Width(m.width).Render(m.renderPickerHelp())

	tail := inputStr + statusStr + helpStr

	// Reserve exactly the space the fixed (non-table) sections need, sized
	// off their actual rendered line counts rather than a hardcoded
	// estimate. This is what keeps the table box (including its top
	// border) fully on-screen regardless of how many lines the help bar
	// or a status message end up wrapping to on a given terminal width.
	const tableBoxChrome = 2 // box border: top + bottom
	const spacerLines = 1    // blank line between the table box and the input box
	headLines := strings.Count(head, "\n")
	tailLines := strings.Count(tail, "\n") + 1
	tableHeight := clampMin(m.height-headLines-spacerLines-tailLines-tableBoxChrome, 5)

	var b strings.Builder
	b.WriteString(head)

	// Main content: loading or ticket list
	if m.loading {
		loadingText := fmt.Sprintf(" %s %s", m.spinner.View(), m.loadingMsg)
		b.WriteString(boxStyle.Width(m.width - 4).Height(tableHeight).Render(loadingText))
		b.WriteString("\n\n")
	} else {
		// Table container
		tableStyle := boxStyle
		if m.pickerFocus == FocusTable {
			tableStyle = activeBoxStyle
		}
		m.table.SetHeight(tableHeight)
		tableView := tableStyle.Width(m.width - 4).Render(m.table.View())
		b.WriteString(tableView + "\n\n")
	}

	b.WriteString(tail)

	return b.String()
}

func (m Model) renderOverviewView() string {
	var b strings.Builder

	key := m.overview.Key
	if key == "" {
		key = "Unknown"
	}
	summary := ""
	description := ""
	status := ""
	if m.overview.Ticket != nil {
		summary = m.overview.Ticket.Summary
		description = strings.TrimSpace(m.overview.Ticket.Description)
		status = m.overview.Ticket.Status
	}

	header := titleStyle.Render("Jira Refine") + " " +
		projectBadgeStyle.Render(key) + " " +
		headerInfoStyle.Render("Ticket Overview")
	head := header + "\n\n"
	b.WriteString(head)

	var content strings.Builder
	content.WriteString(fmt.Sprintf("Summary:  %s\nStatus:   %s\n\n", summary, status))
	content.WriteString(lipgloss.NewStyle().Bold(true).Render("Description:") + "\n")
	if description == "" {
		content.WriteString(explanationStyle.Render("(no description provided)") + "\n")
	} else {
		content.WriteString(description + "\n")
	}
	content.WriteString("\n")

	if isThinDescription(description) {
		content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(warningColor).Render(
			"⚠ This ticket's description is thin — consider adding a Context Note below to improve refinement quality.",
		) + "\n\n")
	}

	content.WriteString(lipgloss.NewStyle().Bold(true).Render("Context Note (optional):") + "\n")
	content.WriteString(explanationStyle.Render("Background the description is missing — business context, constraints, prior decisions.") + "\n")

	statusStr := ""
	if statusLine := m.renderStatusLine(); statusLine != "" {
		statusStr = statusLine + "\n"
	}
	helpStr := statusBar.Width(m.width).Render(m.renderOverviewHelp())
	plainTail := statusStr + helpStr

	const boxChrome = 2
	headLines := strings.Count(head, "\n")
	tailLines := strings.Count(plainTail, "\n") + 1
	boxHeight := clampMin(m.height-headLines-tailLines-boxChrome, 5)

	b.WriteString(boxStyle.Width(m.width-4).Height(boxHeight).Render(content.String()+"\n"+m.contextNoteArea.View()) + "\n")
	b.WriteString(plainTail)

	return b.String()
}

func (m Model) renderOverviewHelp() string {
	return fmt.Sprintf("%s %s  •  %s %s  •  %s %s",
		helpKey.Render("[Ctrl+S]"), helpDesc.Render("Submit Note & Continue"),
		helpKey.Render("[Esc]"), helpDesc.Render("Skip & Continue"),
		helpKey.Render("[Ctrl+G]"), helpDesc.Render("Back to Picker"),
	)
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
	head := header + "\n\n"
	b.WriteString(head)

	// Status line
	statusStr := ""
	if statusLine := m.renderStatusLine(); statusLine != "" {
		statusStr = statusLine + "\n"
	}
	helpStr := statusBar.Width(m.width).Render(m.renderInterviewHelp())
	plainTail := statusStr + helpStr

	// Reserve exactly the space the fixed (non-box) sections need, sized off
	// their actual rendered line counts rather than a hardcoded estimate —
	// same approach as renderPickerView (see the comment there). Without
	// this, the box could claim more height than the terminal actually has
	// left, pushing its own top (and everything above it) off-screen.
	const boxChrome = 2 // box border: top + bottom
	headLines := strings.Count(head, "\n")

	if m.loading {
		tailLines := strings.Count(plainTail, "\n") + 1
		boxHeight := clampMin(m.height-headLines-tailLines-boxChrome, 5)
		loadingText := fmt.Sprintf(" %s %s", m.spinner.View(), m.loadingMsg)
		b.WriteString(boxStyle.Width(m.width - 4).Height(boxHeight).Render(loadingText))
		b.WriteString("\n\n")
	} else if m.activeSnapshot != nil && len(m.activeSnapshot.CurrentFrontier) > 0 {
		var content strings.Builder
		totalQ := len(m.activeSnapshot.CurrentFrontier)
		// The round header's spacing is produced by the same
		// concatenate-then-append-newline path used for every question row
		// below (rather than embedding "\n\n" inside the Render() call), so
		// the first question's leading spacing matches the rest exactly.
		content.WriteString(lipgloss.NewStyle().Bold(true).Render(
			fmt.Sprintf("=== Refinement Round %d (%d Frontier Questions) ===", roundNum, totalQ),
		) + "\n\n")

		for i, q := range m.activeSnapshot.CurrentFrontier {
			cursor := "  "
			titleSt := questionTitle
			if i == m.interviewIndex {
				cursor = "▶ "
				titleSt = questionActiveTitle
			}

			idBadge := fmt.Sprintf("[%s] ", q.ID)
			titleWidth := titleBudget(m.width-6, cursor+idBadge, "")
			content.WriteString(cursor + idBadge + titleSt.Render(truncateTitle(q.Title, titleWidth)) + "\n")
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

		actionButtons := lipgloss.NewStyle().PaddingLeft(1).Render(
			lipgloss.JoinHorizontal(lipgloss.Top,
				activeBoxStyle.Render("[ Submit Round (Ctrl+S) ]"),
				"    ",
				boxStyle.Render("[ Finalize Refinement (Ctrl+F) ]"),
			),
		)
		tail := actionButtons + "\n\n" + plainTail
		tailLines := strings.Count(tail, "\n") + 1
		boxHeight := clampMin(m.height-headLines-tailLines-boxChrome, 5)

		vp := m.viewport
		vp.Height = boxHeight
		vp.SetContent(content.String())
		b.WriteString(boxStyle.Width(m.width-4).Height(boxHeight).Render(vp.View()) + "\n")
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
		tailLines := strings.Count(plainTail, "\n") + 1
		boxHeight := clampMin(m.height-headLines-tailLines-boxChrome, 5)
		b.WriteString(boxStyle.Width(m.width-4).Height(boxHeight).Render(content.String()) + "\n\n")
	} else {
		details := fmt.Sprintf(
			"Ticket:      %s\nSummary:     %s\nStatus:      %s\n\nAll frontier questions resolved or ready for frontier generation.\nPress [ctrl+f] to finalize decomposition, or [r] to generate frontier.",
			key,
			m.activeSnapshot.Ticket.Summary,
			m.activeSnapshot.Status,
		)
		tailLines := strings.Count(plainTail, "\n") + 1
		boxHeight := clampMin(m.height-headLines-tailLines-boxChrome, 5)
		b.WriteString(boxStyle.Width(m.width-4).Height(boxHeight).Render(details) + "\n\n")
	}

	b.WriteString(plainTail)

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
		b.WriteString(boxStyle.Width(m.width-4).Height(m.height-10).Render(emptyMsg) + "\n\n")
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

		idBadge := fmt.Sprintf("[%s]", item.ID)

		if item.Kind == TreeItemEpic {
			prefix := fmt.Sprintf("%s%s %s ", cursor, check, idBadge)
			suffix := " " + projBadge
			titleWidth := titleBudget(m.width-4, prefix, suffix)
			treeContent.WriteString(fmt.Sprintf("%s%s %s %s %s\n",
				cursor,
				check,
				lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#58A6FF")).Render(idBadge),
				itemStyle.Render(truncateTitle(item.Title, titleWidth)),
				lipgloss.NewStyle().Foreground(lipgloss.Color("#8B949E")).Render(projBadge),
			))
		} else {
			prefix := fmt.Sprintf("%s  └─ %s %s ", cursor, check, idBadge)
			suffix := " " + projBadge
			titleWidth := titleBudget(m.width-4, prefix, suffix)
			treeContent.WriteString(fmt.Sprintf("%s  └─ %s %s %s %s\n",
				cursor,
				check,
				lipgloss.NewStyle().Foreground(lipgloss.Color("#7EE787")).Render(idBadge),
				itemStyle.Render(truncateTitle(item.Title, titleWidth)),
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
	b.WriteString(boxStyle.Width(m.width-4).Height(treeHeight).Render(vp.View()) + "\n")

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
		b.WriteString(activeBoxStyle.Width(m.width-4).Render(editBox) + "\n")
	}

	// Action buttons
	actionButtons := lipgloss.NewStyle().PaddingLeft(1).Render(
		lipgloss.JoinHorizontal(lipgloss.Top,
			boxStyle.Render("[ Return to Interview (B/Esc) ]"),
			"    ",
			activeBoxStyle.Render("[ Proceed to Jira Sync (S) ]"),
		),
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

func (m Model) renderSyncView() string {
	var b strings.Builder

	key := "Unknown"
	if m.activeSnapshot != nil {
		key = m.activeSnapshot.Key
	}

	header := titleStyle.Render("Jira Refine") + " " +
		projectBadgeStyle.Render(key) + " " +
		headerInfoStyle.Render("Jira Synchronization & Live Progress")
	b.WriteString(header + "\n\n")

	total := len(m.syncSteps)
	completed := 0
	for _, st := range m.syncSteps {
		if st.Status == refine.StepCompleted {
			completed++
		}
	}

	percent := 0
	if total > 0 {
		percent = (completed * 100) / total
	}

	// Progress bar
	barWidth := 30
	if m.width > 60 {
		barWidth = 40
	}
	filled := 0
	if total > 0 {
		filled = (completed * barWidth) / total
	}
	bar := "[" + lipgloss.NewStyle().Foreground(secondaryColor).Bold(true).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(mutedColor).Render(strings.Repeat("░", barWidth-filled)) + "]"
	progressLine := fmt.Sprintf("Progress: %s  %d%% (%d/%d items)", bar, percent, completed, total)
	b.WriteString(lipgloss.NewStyle().Bold(true).Render(progressLine) + "\n\n")

	if m.syncState == SyncStateSuccess {
		// Final success summary view
		var summary strings.Builder
		summary.WriteString(lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("✓ Jira Synchronization Complete! All items created successfully.\n\n"))
		summary.WriteString(fmt.Sprintf("Strategic Ticket: %s\n\n", key))
		summary.WriteString(lipgloss.NewStyle().Bold(true).Render("Created Jira Issues & Clickable URLs:\n\n"))

		baseURL := ""
		if m.cfg != nil {
			baseURL = strings.TrimRight(m.cfg.Jira.URL, "/")
		}

		if m.activeSnapshot != nil && m.activeSnapshot.Tree != nil {
			for _, epic := range m.activeSnapshot.Tree.Epics {
				if epic.Excluded {
					continue
				}
				epicURL := ""
				if baseURL != "" && epic.Key != "" {
					epicURL = fmt.Sprintf("%s/browse/%s", baseURL, epic.Key)
				}
				clickableEpicKey := formatClickableURL(epic.Key, epicURL)
				summary.WriteString(fmt.Sprintf("• [%s] %s (%s) [%s]\n",
					lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#58A6FF")).Render(epic.ID),
					clickableEpicKey,
					epic.Title,
					epic.DeliveryProject,
				))
				if epicURL != "" {
					summary.WriteString(fmt.Sprintf("  URL: %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("#58A6FF")).Underline(true).Render(epicURL)))
				}
				for _, task := range epic.Tasks {
					if task.Excluded {
						continue
					}
					taskURL := ""
					if baseURL != "" && task.Key != "" {
						taskURL = fmt.Sprintf("%s/browse/%s", baseURL, task.Key)
					}
					clickableTaskKey := formatClickableURL(task.Key, taskURL)
					summary.WriteString(fmt.Sprintf("  └─ [%s] %s (%s) [%s]\n",
						lipgloss.NewStyle().Foreground(lipgloss.Color("#7EE787")).Render(task.ID),
						clickableTaskKey,
						task.Title,
						task.DeliveryProject,
					))
					if taskURL != "" {
						summary.WriteString(fmt.Sprintf("     URL: %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("#58A6FF")).Underline(true).Render(taskURL)))
					}
				}
				summary.WriteString("\n")
			}

			// Dependency links
			var depLines []string
			for _, epic := range m.activeSnapshot.Tree.Epics {
				for _, task := range epic.Tasks {
					for _, depID := range task.DependsOn {
						if depID == epic.ID {
							continue
						}
						depKey := m.syncIDMap[depID]
						if depKey == "" {
							depKey = depID
						}
						taskKey := m.syncIDMap[task.ID]
						if taskKey == "" {
							taskKey = task.Key
						}
						depLines = append(depLines, fmt.Sprintf("• %s [%s] is blocked by %s [%s]", taskKey, task.ID, depKey, depID))
					}
				}
			}
			if len(depLines) > 0 {
				summary.WriteString(lipgloss.NewStyle().Bold(true).Render("Dependency Links:\n"))
				for _, dl := range depLines {
					summary.WriteString("  " + dl + "\n")
				}
				summary.WriteString("\n")
			}
		}

		summary.WriteString(explanationStyle.Render("Session snapshot marked as completed (synced)."))

		vp := m.syncViewport
		vp.SetContent(summary.String())
		vpHeight := m.height - 12
		if vpHeight < 5 {
			vpHeight = 5
		}
		b.WriteString(boxStyle.Width(m.width-4).Height(vpHeight).Render(vp.View()) + "\n\n")

		actionButtons := lipgloss.NewStyle().PaddingLeft(1).Render(
			lipgloss.JoinHorizontal(lipgloss.Top,
				activeBoxStyle.Render("[ Return to Ticket Picker (P) ]"),
				"    ",
				boxStyle.Render("[ Return to Tree Review (B/Esc) ]"),
			),
		)
		b.WriteString(actionButtons + "\n\n")
	} else {
		// Running or Failed state
		var stepLines strings.Builder
		stepLines.WriteString(lipgloss.NewStyle().Bold(true).Render("=== Synchronization Step Execution ===\n\n"))

		for i, st := range m.syncSteps {
			var icon string
			var stStyle lipgloss.Style
			switch st.Status {
			case refine.StepCompleted:
				icon = lipgloss.NewStyle().Foreground(secondaryColor).Bold(true).Render("✓")
				stStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
			case refine.StepRunning:
				icon = lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("▶")
				stStyle = lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
			case refine.StepFailed:
				icon = lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render("✖")
				stStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
			default:
				icon = lipgloss.NewStyle().Foreground(mutedColor).Render("⏳")
				stStyle = lipgloss.NewStyle().Foreground(mutedColor)
			}

			line := fmt.Sprintf(" %s %s [%s] %s",
				icon,
				lipgloss.NewStyle().Foreground(lipgloss.Color("#8B949E")).Render(fmt.Sprintf("%2d.", i+1)),
				st.ID,
				st.Title,
			)
			if st.Key != "" {
				line += " -> " + lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render(st.Key)
			}
			if st.Status == refine.StepRunning {
				line += " " + m.spinner.View()
			}
			if st.Error != "" {
				line += "\n     " + lipgloss.NewStyle().Foreground(accentColor).Render("Error: "+st.Error)
			}
			stepLines.WriteString(stStyle.Render(line) + "\n")
		}

		vp := m.syncViewport
		vp.SetContent(stepLines.String())
		vpHeight := m.height - 14
		if m.syncState == SyncStateFailed {
			vpHeight -= 4
		}
		if vpHeight < 5 {
			vpHeight = 5
		}
		b.WriteString(boxStyle.Width(m.width-4).Height(vpHeight).Render(vp.View()) + "\n")

		if m.syncState == SyncStateFailed && m.syncError != nil {
			errText := fmt.Sprintf("✖ Error during synchronization:\n%s\n\nPress [r] to Retry failed item, [b/esc] to return to Decomposition Tree, [q] to Quit.", m.syncError.Error())
			b.WriteString(activeBoxStyle.Width(m.width-4).BorderForeground(accentColor).Render(errText) + "\n\n")
		} else {
			b.WriteString("\n")
		}
	}

	statusLine := m.renderStatusLine()
	if statusLine != "" {
		b.WriteString(statusLine + "\n")
	}

	b.WriteString(statusBar.Width(m.width).Render(m.renderSyncHelp()))
	return b.String()
}

func (m Model) renderSyncHelp() string {
	if m.syncState == SyncStateFailed {
		return fmt.Sprintf("%s %s  •  %s %s  •  %s %s",
			helpKey.Render("[r]"), helpDesc.Render("Retry Failed Item"),
			helpKey.Render("[b/esc]"), helpDesc.Render("Back to Tree Editor"),
			helpKey.Render("[q]"), helpDesc.Render("Quit"),
		)
	}
	if m.syncState == SyncStateSuccess {
		return fmt.Sprintf("%s %s  •  %s %s  •  %s %s",
			helpKey.Render("[p]"), helpDesc.Render("Ticket Picker"),
			helpKey.Render("[b/esc]"), helpDesc.Render("Back to Tree Review"),
			helpKey.Render("[q]"), helpDesc.Render("Quit"),
		)
	}

	return fmt.Sprintf("%s %s  •  %s %s",
		helpKey.Render("[↑/↓]"), helpDesc.Render("Scroll Steps"),
		helpKey.Render("[Ctrl+C]"), helpDesc.Render("Quit"),
	)
}

func formatClickableURL(key, rawURL string) string {
	if rawURL == "" || key == "" {
		return key
	}
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\\", rawURL, key)
}
