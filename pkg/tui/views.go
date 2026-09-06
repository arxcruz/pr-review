package tui

import (
	"fmt"
	"strings"

	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/charmbracelet/lipgloss"
)

// View renders the entire TUI
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing pr-review TUI..."
	}

	header := m.renderHeader()
	leftPane := m.renderLeftPane()
	rightPane := m.renderRightPane()

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	footer := m.renderFooter()

	baseView := lipgloss.JoinVertical(lipgloss.Left, header, mainContent, footer)

	if m.aiSelectorModal {
		return m.renderAISelectorModal(baseView)
	}

	if m.reviewSelectorModal {
		return m.renderReviewSelectorModal(baseView)
	}

	if m.confirmPostModal {
		return m.renderConfirmPostModal(baseView)
	}

	if m.confirmDeleteModal {
		return m.renderConfirmDeleteModal(baseView)
	}

	if m.helpModal {
		return m.renderHelpModal(baseView)
	}

	return baseView
}

func (m Model) renderHeader() string {
	title := titleStyle.Render("🤖 pr-review")

	var projInfo string
	currProj := m.currentProject()
	if currProj.ID != "" {
		projText := fmt.Sprintf("[%d/%d] %s (%s)", m.projectIdx+1, len(m.projects), currProj.ID, currProj.Provider)
		projInfo = projectBadgeStyle.Render(projText)
	}

	activeAI := m.activeAIChoice()
	aiInfo := headerInfoStyle.Render(fmt.Sprintf("AI: %s (%s)", activeAI.DisplayName, activeAI.Model))

	var filterStatus string
	if m.isFiltering {
		filterStatus = lipgloss.NewStyle().Foreground(warningColor).Render("🔍 " + m.filterInput.View())
	} else if m.filterInput.Value() != "" {
		filterStatus = lipgloss.NewStyle().Foreground(mutedColor).Render(fmt.Sprintf("🔍 Filter: %q (press / to edit, esc to clear)", m.filterInput.Value()))
	}

	leftHeader := lipgloss.JoinHorizontal(lipgloss.Center, title, " ", projInfo, aiInfo)
	gap := m.width - lipgloss.Width(leftHeader) - lipgloss.Width(filterStatus) - 2
	if gap < 1 {
		gap = 1
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, leftHeader, strings.Repeat(" ", gap), filterStatus)
}

func (m Model) renderLeftPane() string {
	paneW := (m.width * 42) / 100
	if paneW < 40 {
		paneW = 40
	}
	paneH := m.height - 6
	if paneH < 10 {
		paneH = 10
	}

	var paneContent string
	if m.loading && len(m.allPRs) == 0 {
		paneContent = lipgloss.Place(
			paneW-2,
			paneH-2,
			lipgloss.Center,
			lipgloss.Center,
			fmt.Sprintf("%s %s", m.spinner.View(), m.loadingMsg),
		)
	} else if len(m.filteredPRs) == 0 {
		paneContent = lipgloss.Place(
			paneW-2,
			paneH-2,
			lipgloss.Center,
			lipgloss.Center,
			"No pull requests found.",
		)
	} else {
		paneContent = m.table.View()
	}

	style := paneStyle.Width(paneW).Height(paneH)
	if m.activePane == panePRList {
		style = activePaneStyle.Width(paneW).Height(paneH)
	}

	return style.Render(paneContent)
}

func (m Model) renderRightPane() string {
	leftW := (m.width * 42) / 100
	if leftW < 40 {
		leftW = 40
	}
	paneW := m.width - leftW - 4
	if paneW < 40 {
		paneW = 40
	}
	paneH := m.height - 6
	if paneH < 10 {
		paneH = 10
	}

	// Render Tabs
	tab1 := inactiveTabStyle.Render("[1] Overview")
	if m.activeTab == tabOverview {
		tab1 = activeTabStyle.Render("[1] Overview")
	}

	tab2 := inactiveTabStyle.Render("[2] Diff")
	if m.activeTab == tabDiff {
		tab2 = activeTabStyle.Render("[2] Diff")
	}

	tab3Title := "[3] AI Review"
	if len(m.availableReviews) > 1 {
		tab3Title = fmt.Sprintf("[3] AI Review (%d)", len(m.availableReviews))
	}
	tab3 := inactiveTabStyle.Render(tab3Title)
	if m.activeTab == tabReview {
		tab3 = activeTabStyle.Render(tab3Title)
	}

	tabs := lipgloss.JoinHorizontal(lipgloss.Top, tab1, tab2, tab3)

	var content string
	switch m.activeTab {
	case tabOverview:
		content = m.overviewViewport.View()
	case tabDiff:
		if m.diffLoading {
			content = lipgloss.Place(
				paneW-2,
				paneH-5,
				lipgloss.Center,
				lipgloss.Center,
				fmt.Sprintf("%s Fetching PR diff...", m.spinner.View()),
			)
		} else {
			content = m.diffViewport.View()
		}
	case tabReview:
		if m.loading {
			content = lipgloss.Place(
				paneW-2,
				paneH-5,
				lipgloss.Center,
				lipgloss.Center,
				fmt.Sprintf("%s %s", m.spinner.View(), m.loadingMsg),
			)
		} else if m.compareMode && len(m.availableReviews) >= 2 {
			content = m.renderCompareReviewPane(paneW, paneH)
		} else {
			var subBar string
			if len(m.availableReviews) > 1 {
				var badges []string
				badges = append(badges, lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("⚡ Versions:"))
				for i, rev := range m.availableReviews {
					pLabel := rev.Provider
					if pLabel == "" {
						pLabel = "local"
					}
					mLabel := rev.Model
					if mLabel == "" {
						mLabel = "default"
					}
					badgeText := fmt.Sprintf("[%d] %s (%s)", i+1, pLabel, mLabel)
					if i == m.selectedReviewIdx {
						badges = append(badges, lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(primaryColor).Bold(true).Render(" ▶ "+badgeText+" "))
					} else {
						badges = append(badges, lipgloss.NewStyle().Foreground(lipgloss.Color("#A0A0B0")).Background(lipgloss.Color("#252535")).Render(" "+badgeText+" "))
					}
				}
				badges = append(badges,
					lipgloss.NewStyle().Foreground(secondaryColor).Bold(true).Render("  [ [ / ] ] Switch"),
					lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true).Render("  [c] Compare Side-by-Side"),
				)
				subBar = lipgloss.JoinHorizontal(lipgloss.Center, badges...) + "\n"
			}
			content = subBar + m.reviewViewport.View()
		}
	}

	body := lipgloss.JoinVertical(lipgloss.Left, tabs, "\n", content)

	style := paneStyle.Width(paneW).Height(paneH)
	if m.activePane == paneDetail {
		style = activePaneStyle.Width(paneW).Height(paneH)
	}

	return style.Render(body)
}

func (m Model) renderCompareReviewPane(paneW, paneH int) string {
	if len(m.availableReviews) < 2 {
		return m.reviewViewport.View()
	}

	leftRev := m.availableReviews[m.compareLeftIdx]
	rightRev := m.availableReviews[m.compareRightIdx]

	colW := (paneW - 6) / 2
	if colW < 20 {
		colW = 20
	}

	leftHeaderStyle := lipgloss.NewStyle().Foreground(mutedColor).Bold(true)
	rightHeaderStyle := lipgloss.NewStyle().Foreground(mutedColor).Bold(true)

	if m.compareActiveCol == 0 {
		leftHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(primaryColor).Bold(true).Padding(0, 1)
	} else {
		rightHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(primaryColor).Bold(true).Padding(0, 1)
	}

	leftHeader := leftHeaderStyle.Render(fmt.Sprintf("◀ Column 1: %s (%s) [%d/%d]", leftRev.Provider, leftRev.Model, m.compareLeftIdx+1, len(m.availableReviews)))
	rightHeader := rightHeaderStyle.Render(fmt.Sprintf("Column 2: %s (%s) [%d/%d] ▶", rightRev.Provider, rightRev.Model, m.compareRightIdx+1, len(m.availableReviews)))

	subBanner := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true).Render("⚡ SIDE-BY-SIDE COMPARE") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#A0A0A0")).Render(" • [Tab/h/l] Switch Column • [ [ / ] ] Change Version • [c/Esc] Exit")

	leftPane := lipgloss.JoinVertical(lipgloss.Left, leftHeader, m.compareLeftViewport.View())
	rightPane := lipgloss.JoinVertical(lipgloss.Left, rightHeader, m.compareRightViewport.View())

	splitContent := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(colW).Render(leftPane),
		lipgloss.NewStyle().Foreground(borderColor).Render(" │ "),
		lipgloss.NewStyle().Width(colW).Render(rightPane),
	)

	return lipgloss.JoinVertical(lipgloss.Left, subBanner, "\n", splitContent)
}

func keyHintStr(list config.KeyList) string {
	if len(list) == 0 {
		return ""
	}
	return list[0]
}

func keyListStr(list config.KeyList) string {
	return strings.Join(list, " / ")
}

func (m Model) renderFooter() string {
	// Status text
	status := m.statusMsg
	if m.statusIsErr {
		status = statusError.Render("✖ " + status)
	} else if status != "" {
		status = statusSuccess.Render(status)
	}

	// Keybindings hint
	kb := m.cfg.Keybindings
	keys := []string{
		fmt.Sprintf("%s Switch Pane", helpKey.Render(keyHintStr(kb.SwitchPane))),
		fmt.Sprintf("%s Scroll", helpKey.Render(fmt.Sprintf("%s/%s", keyHintStr(kb.Up), keyHintStr(kb.Down)))),
		fmt.Sprintf("%s Review", helpKey.Render(keyHintStr(kb.RunReview))),
		fmt.Sprintf("%s AI Engine", helpKey.Render(keyHintStr(kb.SelectAI))),
	}
	if len(m.availableReviews) > 1 {
		keys = append(keys,
			fmt.Sprintf("%s Cycle", helpKey.Render("[/]")),
			fmt.Sprintf("%s Compare", helpKey.Render(keyHintStr(kb.CompareReviews))),
			fmt.Sprintf("%s Versions", helpKey.Render(keyHintStr(kb.SelectReview))),
		)
	}
	if len(m.availableReviews) > 0 || m.reviewContent != "" {
		keys = append(keys, fmt.Sprintf("%s Delete", helpKey.Render(keyHintStr(kb.DeleteReview))))
	}
	keys = append(keys,
		fmt.Sprintf("%s Diff", helpKey.Render(keyHintStr(kb.TabDiff))),
		fmt.Sprintf("%s Post", helpKey.Render(keyHintStr(kb.PostReview))),
		fmt.Sprintf("%s Filter", helpKey.Render(keyHintStr(kb.Filter))),
		fmt.Sprintf("%s Next Repo", helpKey.Render(keyHintStr(kb.NextProject))),
		fmt.Sprintf("%s Refresh", helpKey.Render(keyHintStr(kb.Refresh))),
		fmt.Sprintf("%s Help", helpKey.Render(keyHintStr(kb.Help))),
		fmt.Sprintf("%s Quit", helpKey.Render(keyHintStr(kb.Quit))),
	)
	keyHints := strings.Join(keys, "  ")

	gap := m.width - lipgloss.Width(status) - lipgloss.Width(keyHints) - 2
	if gap < 1 {
		gap = 1
	}

	barContent := lipgloss.JoinHorizontal(lipgloss.Center, status, strings.Repeat(" ", gap), keyHints)
	return statusBar.Width(m.width).Render(barContent)
}

func padOrTruncate(s string, width int) string {
	runes := []rune(s)
	if len(runes) > width {
		if width > 3 {
			return string(runes[:width-3]) + "..."
		}
		return string(runes[:width])
	}
	return s + strings.Repeat(" ", width-len(runes))
}

func (m Model) renderAISelectorModal(background string) string {
	title := modalTitle.Render("Select AI Review Engine")

	var rows []string
	rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color("#A0A0A0")).Render("Choose which AI engine, endpoint, and model to use for reviews:\n"))

	// Header row: 5 spaces for cursor (2) + check (3), then columns
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A0A0A0"))
	headerRow := "     " +
		headerStyle.Render(padOrTruncate("PROVIDER / ENDPOINT", 26)) + "  " +
		headerStyle.Render(padOrTruncate("MODEL NAME", 38)) + "  " +
		headerStyle.Render(padOrTruncate("STATUS", 18))
	rows = append(rows, headerRow)

	dividerRow := lipgloss.NewStyle().Foreground(borderColor).Render(
		"     " + strings.Repeat("─", 26) + "  " + strings.Repeat("─", 38) + "  " + strings.Repeat("─", 18),
	)
	rows = append(rows, dividerRow)

	maxVisible := 10
	startIdx := 0
	if len(m.aiProviders) > maxVisible {
		startIdx = m.aiSelectorCursor - (maxVisible / 2)
		if startIdx < 0 {
			startIdx = 0
		}
		if startIdx+maxVisible > len(m.aiProviders) {
			startIdx = len(m.aiProviders) - maxVisible
		}
	}
	endIdx := startIdx + maxVisible
	if endIdx > len(m.aiProviders) {
		endIdx = len(m.aiProviders)
	}

	if startIdx > 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render("  ▲ ..."))
	}

	for i := startIdx; i < endIdx; i++ {
		provider := m.aiProviders[i]
		cursor := "  "
		provStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
		modelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#A0A0A0"))
		statusStyle := lipgloss.NewStyle().Foreground(secondaryColor)
		if !provider.Configured {
			statusStyle = lipgloss.NewStyle().Foreground(warningColor)
		}

		isActive := strings.EqualFold(provider.ID, m.selectedAIProvider)
		if i == m.aiSelectorCursor {
			cursor = lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("▶ ")
			provStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
			modelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
		}

		activeCheck := "   "
		if isActive {
			activeCheck = lipgloss.NewStyle().Foreground(secondaryColor).Bold(true).Render("✓  ")
		}

		statusText := "✓ Configured"
		if !provider.Configured {
			statusText = "⚠ Key missing"
		}

		provCell := provStyle.Render(padOrTruncate(provider.DisplayName, 26))
		modelCell := modelStyle.Render(padOrTruncate(provider.Model, 38))
		statusCell := statusStyle.Render(padOrTruncate(statusText, 18))

		row := cursor + activeCheck + provCell + "  " + modelCell + "  " + statusCell
		rows = append(rows, row)
	}

	if endIdx < len(m.aiProviders) {
		rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render("  ▼ ..."))
	}

	hint := lipgloss.NewStyle().Foreground(primaryColor).Render("\n[j/k or ↑/↓] Navigate   [Enter / Space] Select AI Engine   [Esc] Cancel")
	rows = append(rows, hint)

	body := lipgloss.JoinVertical(lipgloss.Left, title, "\n", lipgloss.JoinVertical(lipgloss.Left, rows...))
	dialogWidth := 96
	if m.width > 0 && m.width < 100 {
		dialogWidth = m.width - 4
	}
	dialog := modalBox.Width(dialogWidth).Render(body)
	return placeOverlay(m.width, m.height, dialog, background)
}

func (m Model) renderReviewSelectorModal(background string) string {
	prNum := 0
	if m.selectedPR != nil {
		prNum = m.selectedPR.Number
	}
	title := modalTitle.Render(fmt.Sprintf("Select Saved Review Version for PR #%d", prNum))

	var rows []string
	rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color("#A0A0A0")).Render("Choose which review version to load and view:\n"))

	// Header row: 5 spaces for cursor (2) + check (3), then columns
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A0A0A0"))
	headerRow := "     " +
		headerStyle.Render(padOrTruncate("ENGINE / MODEL", 28)) + "  " +
		headerStyle.Render(padOrTruncate("FILE NAME", 34)) + "  " +
		headerStyle.Render(padOrTruncate("CREATED", 18))
	rows = append(rows, headerRow)

	dividerRow := lipgloss.NewStyle().Foreground(borderColor).Render(
		"     " + strings.Repeat("─", 28) + "  " + strings.Repeat("─", 34) + "  " + strings.Repeat("─", 18),
	)
	rows = append(rows, dividerRow)

	for i, rev := range m.availableReviews {
		cursor := "  "
		itemStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
		fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
		dateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

		if i == m.reviewSelectorCursor {
			cursor = lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("▶ ")
			itemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
			fileStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0"))
			dateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0"))
		}

		activeCheck := "   "
		if i == m.selectedReviewIdx {
			activeCheck = lipgloss.NewStyle().Foreground(secondaryColor).Bold(true).Render("✓  ")
		}

		providerLabel := rev.Provider
		if providerLabel == "" {
			providerLabel = "local"
		}
		modelLabel := rev.Model
		if modelLabel == "" {
			modelLabel = "default"
		}

		engineDesc := fmt.Sprintf("%s (%s)", providerLabel, modelLabel)
		dateDesc := rev.CreatedAt.Format("2006-01-02 15:04")

		engineCell := itemStyle.Render(padOrTruncate(engineDesc, 28))
		fileCell := fileStyle.Render(padOrTruncate(rev.FileName, 34))
		dateCell := dateStyle.Render(padOrTruncate(dateDesc, 18))

		row := cursor + activeCheck + engineCell + "  " + fileCell + "  " + dateCell
		rows = append(rows, row)
	}

	hint := lipgloss.NewStyle().Foreground(primaryColor).Render("\n[j/k or ↑/↓] Navigate   [Enter] Select   [x] Delete Version   [X] Delete All   [Esc] Cancel")
	rows = append(rows, hint)

	body := lipgloss.JoinVertical(lipgloss.Left, title, "\n", lipgloss.JoinVertical(lipgloss.Left, rows...))
	dialogWidth := 94
	if m.width > 0 && m.width < 98 {
		dialogWidth = m.width - 4
	}
	dialog := modalBox.Width(dialogWidth).Render(body)
	return placeOverlay(m.width, m.height, dialog, background)
}



func (m Model) renderConfirmPostModal(background string) string {
	if m.selectedPR == nil {
		return background
	}

	title := modalTitle.Render("Confirm Post Review Comment")
	msg := fmt.Sprintf(
		"Are you sure you want to post the AI review comment to PR #%d (%s)?\n\nTarget Repository: %s\nReview Source: %s\n\n[y] Yes, Post Comment    [n / Esc] Cancel",
		m.selectedPR.Number,
		m.selectedPR.Title,
		m.currentProject().ID,
		m.reviewFilePath,
	)

	dialog := modalBox.Width(64).Render(lipgloss.JoinVertical(lipgloss.Left, title, msg))
	return placeOverlay(m.width, m.height, dialog, background)
}

func (m Model) renderConfirmDeleteModal(background string) string {
	if m.selectedPR == nil {
		return background
	}

	title := modalTitle.Render("Confirm Delete Cached Review")
	var msg string
	if m.deleteTargetAll {
		msg = fmt.Sprintf(
			"⚠️ Are you sure you want to delete ALL %d review file(s) for PR #%d (%s)?\n\nTarget Repository: %s\n\n[y] Yes, Delete All    [n / Esc] Cancel",
			len(m.availableReviews),
			m.selectedPR.Number,
			m.selectedPR.Title,
			m.currentProject().ID,
		)
	} else if m.deleteTargetReview != nil {
		target := m.deleteTargetReview
		msg = fmt.Sprintf(
			"⚠️ Are you sure you want to delete this cached review?\n\nFile: %s\nProvider: %s / Model: %s\nTarget: PR #%d (%s)\n\n[y] Yes, Delete Review    [n / Esc] Cancel",
			target.FilePath,
			target.Provider,
			target.Model,
			m.selectedPR.Number,
			m.selectedPR.Title,
		)
	} else {
		return background
	}

	dialog := modalBox.Width(66).Render(lipgloss.JoinVertical(lipgloss.Left, title, msg))
	return placeOverlay(m.width, m.height, dialog, background)
}

func (m Model) renderHelpModal(background string) string {
	title := modalTitle.Render("pr-review Keyboard Shortcuts")
	kb := m.cfg.Keybindings

	helpContent := fmt.Sprintf(`
Navigation & Scrolling:
  %-20s Switch between PR List & Details View
  %-20s Scroll up / down (line by line in viewport or PR table)
  %-20s Scroll half-page up / down
  %-20s Scroll full page up / down
  %-20s Jump to top / bottom
  %-20s Switch to Overview / Diff / AI Review tab
  %-20s Switch to next configured project/repo
  %-20s Refresh PR list from remote git provider
  %-20s Filter / Search PRs (Esc to clear)

Review & AI Actions:
  %-20s Generate or regenerate AI Review
  %-20s Select AI Review Engine (Ollama, Claude, Gemini, OpenAI, llama.cpp)
  %-20s Cycle to previous / next saved review version
  %-20s Toggle side-by-side review comparison mode
  %-20s Select saved AI review version from list
  %-20s Delete active cached review
  %-20s Delete all cached reviews for this PR
  %-20s Post AI Review comment to GitHub / GitLab / Gerrit

General:
  %-20s Toggle this help modal
  %-20s Quit pr-review

Press [Esc] or [%s] to close this help.`,
		keyListStr(kb.SwitchPane),
		fmt.Sprintf("%s, %s", keyListStr(kb.Up), keyListStr(kb.Down)),
		fmt.Sprintf("%s, %s", keyListStr(kb.HalfPageUp), keyListStr(kb.HalfPageDown)),
		fmt.Sprintf("%s, %s", keyListStr(kb.PageUp), keyListStr(kb.PageDown)),
		fmt.Sprintf("%s, %s", keyListStr(kb.Top), keyListStr(kb.Bottom)),
		fmt.Sprintf("%s, %s, %s", keyListStr(kb.TabOverview), keyListStr(kb.TabDiff), keyListStr(kb.TabReview)),
		keyListStr(kb.NextProject),
		keyListStr(kb.Refresh),
		keyListStr(kb.Filter),
		keyListStr(kb.RunReview),
		keyListStr(kb.SelectAI),
		fmt.Sprintf("%s, %s", keyListStr(kb.PrevReview), keyListStr(kb.NextReview)),
		keyListStr(kb.CompareReviews),
		keyListStr(kb.SelectReview),
		keyListStr(kb.DeleteReview),
		keyListStr(kb.DeleteAllReviews),
		keyListStr(kb.PostReview),
		keyListStr(kb.Help),
		keyListStr(kb.Quit),
		keyHintStr(kb.Help),
	)

	dialog := modalBox.Width(76).Render(lipgloss.JoinVertical(lipgloss.Left, title, helpContent))
	return placeOverlay(m.width, m.height, dialog, background)
}

func placeOverlay(width, height int, overlay, background string) string {
	overlayW := lipgloss.Width(overlay)
	overlayH := lipgloss.Height(overlay)

	topPadding := (height - overlayH) / 2
	if topPadding < 0 {
		topPadding = 0
	}
	leftPadding := (width - overlayW) / 2
	if leftPadding < 0 {
		leftPadding = 0
	}

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay)
}
