package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/arxcruz/pr-review/pkg/gitprovider"
	"github.com/arxcruz/pr-review/pkg/review"
)

const (
	panePRList = 0
	paneDetail = 1

	tabOverview = 0
	tabDiff     = 1
	tabReview   = 2
)

// Messages
type prsLoadedMsg struct {
	project config.ProjectConfig
	prs     []*gitprovider.PullRequest
	err     error
}

type diffLoadedMsg struct {
	prNumber int
	diff     string
	err      error
}

type reviewGeneratedMsg struct {
	prNumber int
	outcome  *review.ReviewOutcome
	err      error
}

type commentPostedMsg struct {
	prNumber int
	err      error
}

type statusClearMsg struct{}

// Model is the main Bubble Tea model for the pr-review TUI
type Model struct {
	cfg        *config.Config
	service    *review.Service
	projects   []config.ProjectConfig
	projectIdx int

	// PR Data
	allPRs      []*gitprovider.PullRequest
	filteredPRs []*gitprovider.PullRequest
	selectedPR  *gitprovider.PullRequest

	// UI State
	activePane int
	activeTab  int
	width      int
	height     int

	// Components
	table            table.Model
	overviewViewport viewport.Model
	diffViewport     viewport.Model
	reviewViewport   viewport.Model
	filterInput      textinput.Model
	spinner          spinner.Model

	// Cache & Content
	diffCache      map[int]string
	diffLoading    bool
	reviewContent  string
	reviewFilePath string
	reviewIsCached bool

	// AI Engine Selection
	selectedAIProvider string
	aiProviders        []AIProviderChoice
	aiSelectorCursor   int
	aiSelectorModal    bool

	// Multi-AI Review Selection
	availableReviews     []review.SavedReview
	selectedReviewIdx    int
	reviewSelectorModal  bool
	reviewSelectorCursor int

	// Side-by-Side Review Comparison
	compareMode          bool
	compareLeftIdx       int
	compareRightIdx      int
	compareActiveCol     int // 0 = left, 1 = right
	compareLeftViewport  viewport.Model
	compareRightViewport viewport.Model

	// Deletion Modal
	confirmDeleteModal bool
	deleteTargetAll    bool
	deleteTargetReview *review.SavedReview

	// Loading & Status
	loading          bool
	loadingMsg       string
	statusMsg        string
	statusIsErr      bool
	confirmPostModal bool
	helpModal        bool
	isFiltering      bool

	// Markdown Renderer
	renderer *glamour.TermRenderer
}

// AIProviderChoice holds provider and model display information
type AIProviderChoice struct {
	ID          string
	Provider    string
	DisplayName string
	Model       string
	Configured  bool
	Endpoint    string
}

func buildAIProviderChoices(cfg *config.Config) []AIProviderChoice {
	targets := config.GetAllAITargets(cfg)
	var choices []AIProviderChoice
	for _, t := range targets {
		choices = append(choices, AIProviderChoice{
			ID:          t.ID,
			Provider:    t.Provider,
			DisplayName: t.Name,
			Model:       t.Model,
			Endpoint:    t.BaseURL,
			Configured:  t.Configured,
		})
	}
	if len(choices) == 0 {
		choices = []AIProviderChoice{
			{
				ID:          "ollama",
				Provider:    "ollama",
				DisplayName: "Ollama (Local AI)",
				Model:       "qwen2.5-coder:latest",
				Endpoint:    "http://localhost:11434",
				Configured:  true,
			},
		}
	}
	return choices
}

// NewModel initializes the TUI model
func NewModel(cfg *config.Config, svc *review.Service) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(primaryColor)

	ti := textinput.New()
	ti.Placeholder = "Type to filter PRs (title, author, label)..."
	ti.CharLimit = 100
	ti.Width = 35

	// Table columns
	columns := []table.Column{
		{Title: "PR", Width: 7},
		{Title: "Title", Width: 38},
		{Title: "Author", Width: 14},
		{Title: "Review", Width: 8},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	tStyle := table.DefaultStyles()
	tStyle.Header = tStyle.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF"))
	tStyle.Selected = tStyle.Selected.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(primaryColor).
		Bold(true)
	t.SetStyles(tStyle)

	projects := cfg.Projects
	if len(projects) == 0 {
		projects = []config.ProjectConfig{
			{ID: "default", Provider: "github"},
		}
	}

	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	aiChoices := buildAIProviderChoices(cfg)
	selectedAI := cfg.DefaultAIProvider
	if selectedAI == "" && len(aiChoices) > 0 {
		selectedAI = aiChoices[0].ID
	}
	cursor := 0
	for i, c := range aiChoices {
		if strings.EqualFold(c.ID, selectedAI) || strings.EqualFold(c.Provider, selectedAI) {
			cursor = i
			selectedAI = c.ID
			break
		}
	}

	return Model{
		cfg:                cfg,
		service:            svc,
		projects:           projects,
		projectIdx:         0,
		activePane:         panePRList,
		activeTab:          tabOverview,
		table:              t,
		filterInput:        ti,
		spinner:            s,
		diffCache:          make(map[int]string),
		overviewViewport:   viewport.New(60, 20),
		diffViewport:         viewport.New(60, 20),
		reviewViewport:       viewport.New(60, 20),
		compareLeftViewport:  viewport.New(30, 20),
		compareRightViewport: viewport.New(30, 20),
		compareLeftIdx:       0,
		compareRightIdx:      1,
		compareActiveCol:     0,
		compareMode:          false,
		renderer:             r,
		selectedAIProvider:   selectedAI,
		aiProviders:          aiChoices,
		aiSelectorCursor:     cursor,
		aiSelectorModal:      false,
	}
}

// Init starts the TUI application and loads initial PRs
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadPRsCmd(m.currentProject()),
	)
}

func (m Model) currentProject() config.ProjectConfig {
	if len(m.projects) == 0 {
		return config.ProjectConfig{}
	}
	if m.projectIdx >= len(m.projects) {
		m.projectIdx = 0
	}
	return m.projects[m.projectIdx]
}

// loadPRsCmd fetches PRs for a project asynchronously
func (m Model) loadPRsCmd(proj config.ProjectConfig) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		provider, err := m.service.GitManager().GetProvider(proj)
		if err != nil {
			return prsLoadedMsg{project: proj, err: err}
		}

		filter := gitprovider.FilterOptions{
			Authors: proj.DefaultFilters.Authors,
			Labels:  proj.DefaultFilters.Labels,
			State:   "open",
			Limit:   50,
		}

		prs, err := provider.ListPullRequests(ctx, proj, filter)
		return prsLoadedMsg{
			project: proj,
			prs:     prs,
			err:     err,
		}
	}
}

// loadDiffCmd fetches diff for a PR asynchronously
func (m Model) loadDiffCmd(proj config.ProjectConfig, prNumber int) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		provider, err := m.service.GitManager().GetProvider(proj)
		if err != nil {
			return diffLoadedMsg{prNumber: prNumber, err: err}
		}

		diff, err := provider.GetDiff(ctx, proj, prNumber)
		return diffLoadedMsg{
			prNumber: prNumber,
			diff:     diff,
			err:      err,
		}
	}
}

// runAIReviewCmd executes the AI review pipeline
func (m Model) runAIReviewCmd(proj config.ProjectConfig, prNumber int) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		opts := review.ReviewOptions{
			AIProvider: m.selectedAIProvider,
			Force:      true, // User explicitly requested generation
		}

		outcome, err := m.service.ReviewPR(ctx, proj, prNumber, opts)
		return reviewGeneratedMsg{
			prNumber: prNumber,
			outcome:  outcome,
			err:      err,
		}
	}
}

// postCommentCmd posts the review to GitHub/GitLab/Gerrit
func (m Model) postCommentCmd(proj config.ProjectConfig, prNumber int, commentBody string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		provider, err := m.service.GitManager().GetProvider(proj)
		if err != nil {
			return commentPostedMsg{prNumber: prNumber, err: err}
		}

		err = provider.PostComment(ctx, proj, prNumber, commentBody)
		return commentPostedMsg{
			prNumber: prNumber,
			err:      err,
		}
	}
}

// Update handles UI events and messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case prsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Failed to load PRs: %v", msg.err)
			m.statusIsErr = true
		} else {
			m.allPRs = msg.prs
			m.applyFilter()
			m.statusMsg = fmt.Sprintf("Loaded %d PR(s) for %s", len(msg.prs), msg.project.ID)
			m.statusIsErr = false
		}

	case diffLoadedMsg:
		m.diffLoading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Failed to load diff for PR #%d: %v", msg.prNumber, msg.err)
			m.statusIsErr = true
		} else {
			m.diffCache[msg.prNumber] = msg.diff
			if m.selectedPR != nil && m.selectedPR.Number == msg.prNumber {
				m.diffViewport.SetContent(formatDiffText(msg.diff))
			}
		}

	case reviewGeneratedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("AI review failed: %v", msg.err)
			m.statusIsErr = true
		} else {
			if m.selectedPR != nil {
				m.availableReviews = m.service.ListReviewsForPR(m.currentProject(), m.selectedPR.Number)
				m.selectedReviewIdx = 0
			}
			m.reviewContent = msg.outcome.Result.Content
			m.reviewFilePath = msg.outcome.SavedFile
			m.reviewIsCached = false
			m.updateReviewViewport()
			m.statusMsg = fmt.Sprintf("✓ AI review generated by %s & saved to %s", m.activeAIChoice().DisplayName, m.reviewFilePath)
			m.statusIsErr = false
			m.activeTab = tabReview
			m.activePane = paneDetail
			m.rebuildTableRows()
		}

	case commentPostedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Failed to post comment: %v", msg.err)
			m.statusIsErr = true
		} else {
			m.statusMsg = fmt.Sprintf("✓ Successfully posted review comment to PR #%d!", msg.prNumber)
			m.statusIsErr = false
		}

	case tea.KeyMsg:
		k := msg.String()

		// Modal handling
		if m.helpModal {
			if m.matches(k, m.cfg.Keybindings.Help) || m.matches(k, m.cfg.Keybindings.Quit) || k == "esc" {
				m.helpModal = false
				return m, nil
			}
			return m, nil
		}

		if m.reviewSelectorModal {
			switch {
			case m.matches(k, m.cfg.Keybindings.Down):
				if len(m.availableReviews) > 0 {
					m.reviewSelectorCursor = (m.reviewSelectorCursor + 1) % len(m.availableReviews)
				}
				return m, nil
			case m.matches(k, m.cfg.Keybindings.Up):
				if len(m.availableReviews) > 0 {
					m.reviewSelectorCursor = (m.reviewSelectorCursor - 1 + len(m.availableReviews)) % len(m.availableReviews)
				}
				return m, nil
			case m.matches(k, m.cfg.Keybindings.DeleteReview):
				if len(m.availableReviews) > 0 && m.reviewSelectorCursor < len(m.availableReviews) {
					target := m.availableReviews[m.reviewSelectorCursor]
					m.reviewSelectorModal = false
					m.confirmDeleteModal = true
					m.deleteTargetAll = false
					m.deleteTargetReview = &target
				}
				return m, nil
			case m.matches(k, m.cfg.Keybindings.DeleteAllReviews):
				if len(m.availableReviews) > 0 {
					m.reviewSelectorModal = false
					m.confirmDeleteModal = true
					m.deleteTargetAll = true
					m.deleteTargetReview = nil
				}
				return m, nil
			case k == "enter" || k == " ":
				m.reviewSelectorModal = false
				if len(m.availableReviews) > 0 && m.reviewSelectorCursor < len(m.availableReviews) {
					m.selectedReviewIdx = m.reviewSelectorCursor
					rev := m.availableReviews[m.selectedReviewIdx]
					m.reviewContent = rev.Content
					m.reviewFilePath = rev.FilePath
					m.reviewIsCached = true
					m.updateReviewViewport()
					m.statusMsg = fmt.Sprintf("✓ Loaded review version #%d (%s / %s)", m.selectedReviewIdx+1, rev.Provider, rev.Model)
					m.statusIsErr = false
				}
				return m, nil
			case k == "esc" || m.matches(k, m.cfg.Keybindings.SelectReview) || m.matches(k, m.cfg.Keybindings.Quit):
				m.reviewSelectorModal = false
				return m, nil
			}
			return m, nil
		}

		if m.confirmDeleteModal {
			switch k {
			case "y", "Y", "enter":
				m.confirmDeleteModal = false
				if m.selectedPR != nil {
					if m.deleteTargetAll {
						count, err := m.service.DeleteReviewsForPR(m.currentProject(), m.selectedPR.Number)
						if err != nil {
							m.statusMsg = fmt.Sprintf("Failed to delete reviews: %v", err)
							m.statusIsErr = true
						} else {
							m.availableReviews = nil
							m.selectedReviewIdx = 0
							m.reviewContent = ""
							m.reviewIsCached = false
							m.reviewFilePath = m.service.ReviewFilePathWithAI(m.currentProject(), m.selectedPR.Number, m.activeAIChoice().Provider, m.activeAIChoice().Model)
							m.updateReviewViewport()
							m.rebuildTableRows()
							m.statusMsg = fmt.Sprintf("✓ Deleted all %d review(s) for PR #%d", count, m.selectedPR.Number)
							m.statusIsErr = false
						}
					} else if m.deleteTargetReview != nil {
						target := m.deleteTargetReview
						err := m.service.DeleteReviewFile(target.FilePath)
						if err != nil {
							m.statusMsg = fmt.Sprintf("Failed to delete review: %v", err)
							m.statusIsErr = true
						} else {
							m.availableReviews = m.service.ListReviewsForPR(m.currentProject(), m.selectedPR.Number)
							if len(m.availableReviews) > 0 {
								if m.selectedReviewIdx >= len(m.availableReviews) {
									m.selectedReviewIdx = 0
								}
								rev := m.availableReviews[m.selectedReviewIdx]
								m.reviewContent = rev.Content
								m.reviewFilePath = rev.FilePath
								m.reviewIsCached = true
							} else {
								m.selectedReviewIdx = 0
								m.reviewContent = ""
								m.reviewIsCached = false
								m.reviewFilePath = m.service.ReviewFilePathWithAI(m.currentProject(), m.selectedPR.Number, m.activeAIChoice().Provider, m.activeAIChoice().Model)
							}
							m.updateReviewViewport()
							m.rebuildTableRows()
							m.statusMsg = fmt.Sprintf("✓ Deleted review file %s", target.FileName)
							m.statusIsErr = false
						}
					}
				}
				return m, nil
			case "n", "N", "esc", "q":
				m.confirmDeleteModal = false
				m.statusMsg = "Review deletion cancelled"
				return m, nil
			}
			return m, nil
		}

		if m.aiSelectorModal {
			switch {
			case m.matches(k, m.cfg.Keybindings.Down):
				m.aiSelectorCursor = (m.aiSelectorCursor + 1) % len(m.aiProviders)
				return m, nil
			case m.matches(k, m.cfg.Keybindings.Up):
				m.aiSelectorCursor = (m.aiSelectorCursor - 1 + len(m.aiProviders)) % len(m.aiProviders)
				return m, nil
			case k == "enter" || k == " ":
				m.aiSelectorModal = false
				choice := m.aiProviders[m.aiSelectorCursor]
				m.selectedAIProvider = choice.ID
				m.statusMsg = fmt.Sprintf("✓ Active AI engine changed to %s (%s)", choice.DisplayName, choice.Model)
				m.statusIsErr = false
				m.updateReviewViewport()
				return m, nil
			case k == "esc" || m.matches(k, m.cfg.Keybindings.SelectAI) || m.matches(k, m.cfg.Keybindings.Quit):
				m.aiSelectorModal = false
				return m, nil
			}
			return m, nil
		}

		if m.confirmPostModal {
			switch k {
			case "y", "Y", "enter":
				m.confirmPostModal = false
				if m.selectedPR != nil && m.reviewContent != "" {
					m.loading = true
					m.loadingMsg = fmt.Sprintf("Posting review comment for #%d...", m.selectedPR.Number)
					commentBody := fmt.Sprintf("🤖 **AI Code Review**\n\n%s", m.reviewContent)
					cmds = append(cmds, m.postCommentCmd(m.currentProject(), m.selectedPR.Number, commentBody))
				}
				return m, tea.Batch(cmds...)
			case "n", "N", "esc", "q":
				m.confirmPostModal = false
				m.statusMsg = "Post comment cancelled"
				return m, nil
			}
			return m, nil
		}

		// Filter input handling
		if m.isFiltering {
			switch k {
			case "esc":
				m.isFiltering = false
				m.filterInput.Blur()
				return m, nil
			case "enter":
				m.isFiltering = false
				m.filterInput.Blur()
				return m, nil
			default:
				var cmd tea.Cmd
				m.filterInput, cmd = m.filterInput.Update(msg)
				m.applyFilter()
				return m, cmd
			}
		}

		// Global Keybindings
		if m.matches(k, m.cfg.Keybindings.Quit) {
			return m, tea.Quit
		}
		if m.matches(k, m.cfg.Keybindings.Help) {
			m.helpModal = true
			return m, nil
		}
		if m.matches(k, m.cfg.Keybindings.SwitchPane) {
			if m.compareMode && m.activeTab == tabReview && m.activePane == paneDetail {
				m.compareActiveCol = 1 - m.compareActiveCol
				return m, nil
			}
			if m.activePane == panePRList {
				m.activePane = paneDetail
			} else {
				m.activePane = panePRList
			}
			return m, nil
		}
		if m.matches(k, m.cfg.Keybindings.Filter) {
			m.isFiltering = true
			m.filterInput.Focus()
			return m, textinput.Blink
		}
		if m.matches(k, m.cfg.Keybindings.NextProject) {
			if len(m.projects) > 1 {
				m.projectIdx = (m.projectIdx + 1) % len(m.projects)
				m.loading = true
				m.loadingMsg = fmt.Sprintf("Loading PRs for %s...", m.currentProject().ID)
				m.allPRs = nil
				m.filteredPRs = nil
				m.selectedPR = nil
				return m, m.loadPRsCmd(m.currentProject())
			}
			return m, nil
		}
		if m.matches(k, m.cfg.Keybindings.Refresh) {
			m.loading = true
			m.loadingMsg = fmt.Sprintf("Refreshing PRs for %s...", m.currentProject().ID)
			return m, m.loadPRsCmd(m.currentProject())
		}
		if m.matches(k, m.cfg.Keybindings.TabOverview) {
			m.activeTab = tabOverview
			return m, nil
		}
		if m.matches(k, m.cfg.Keybindings.TabDiff) {
			m.activeTab = tabDiff
			if m.selectedPR != nil && m.diffCache[m.selectedPR.Number] == "" {
				m.diffLoading = true
				cmds = append(cmds, m.loadDiffCmd(m.currentProject(), m.selectedPR.Number))
			}
			return m, tea.Batch(cmds...)
		}
		if m.matches(k, m.cfg.Keybindings.TabReview) {
			m.activeTab = tabReview
			return m, nil
		}
		if m.matches(k, m.cfg.Keybindings.SelectAI) {
			m.aiSelectorModal = true
			for i, c := range m.aiProviders {
				if strings.EqualFold(c.ID, m.selectedAIProvider) {
					m.aiSelectorCursor = i
					break
				}
			}
			return m, nil
		}
		if m.matches(k, m.cfg.Keybindings.SelectReview) {
			if len(m.availableReviews) > 1 {
				m.reviewSelectorModal = true
				m.reviewSelectorCursor = m.selectedReviewIdx
				return m, nil
			} else if len(m.availableReviews) == 1 {
				m.statusMsg = "Only 1 review exists for this PR. Press [r] with another AI engine to generate another."
				m.statusIsErr = false
				return m, nil
			} else {
				m.statusMsg = "No review files found for this PR yet. Press [r] to generate one."
				m.statusIsErr = true
				return m, nil
			}
		}
		if m.matches(k, m.cfg.Keybindings.PrevReview) {
			if len(m.availableReviews) > 1 {
				if m.compareMode {
					if m.compareActiveCol == 0 {
						m.compareLeftIdx = (m.compareLeftIdx - 1 + len(m.availableReviews)) % len(m.availableReviews)
					} else {
						m.compareRightIdx = (m.compareRightIdx - 1 + len(m.availableReviews)) % len(m.availableReviews)
					}
					m.updateCompareViewports()
				} else {
					m.selectedReviewIdx = (m.selectedReviewIdx - 1 + len(m.availableReviews)) % len(m.availableReviews)
					rev := m.availableReviews[m.selectedReviewIdx]
					m.reviewContent = rev.Content
					m.reviewFilePath = rev.FilePath
					m.reviewIsCached = true
					m.updateReviewViewport()
					m.statusMsg = fmt.Sprintf("✓ Active Review [%d/%d]: %s (%s)", m.selectedReviewIdx+1, len(m.availableReviews), rev.Provider, rev.Model)
					m.statusIsErr = false
				}
				return m, nil
			}
		}
		if m.matches(k, m.cfg.Keybindings.NextReview) {
			if len(m.availableReviews) > 1 {
				if m.compareMode {
					if m.compareActiveCol == 0 {
						m.compareLeftIdx = (m.compareLeftIdx + 1) % len(m.availableReviews)
					} else {
						m.compareRightIdx = (m.compareRightIdx + 1) % len(m.availableReviews)
					}
					m.updateCompareViewports()
				} else {
					m.selectedReviewIdx = (m.selectedReviewIdx + 1) % len(m.availableReviews)
					rev := m.availableReviews[m.selectedReviewIdx]
					m.reviewContent = rev.Content
					m.reviewFilePath = rev.FilePath
					m.reviewIsCached = true
					m.updateReviewViewport()
					m.statusMsg = fmt.Sprintf("✓ Active Review [%d/%d]: %s (%s)", m.selectedReviewIdx+1, len(m.availableReviews), rev.Provider, rev.Model)
					m.statusIsErr = false
				}
				return m, nil
			}
		}
		if m.matches(k, m.cfg.Keybindings.CompareReviews) {
			if len(m.availableReviews) >= 2 {
				m.compareMode = !m.compareMode
				if m.compareMode {
					m.activeTab = tabReview
					m.activePane = paneDetail
					m.updateCompareViewports()
					m.statusMsg = "⚡ Side-by-Side Review Comparison Mode (Press [c] or [esc] to exit, [tab/h/l] to switch columns, [ / ] to change versions)"
					m.statusIsErr = false
				} else {
					m.statusMsg = "Exited Compare Mode."
					m.statusIsErr = false
				}
				return m, nil
			} else {
				m.statusMsg = "Need at least 2 reviews for this PR to compare. Press [r] to generate a review with another AI engine."
				m.statusIsErr = true
				return m, nil
			}
		}
		if m.matches(k, m.cfg.Keybindings.DeleteReview) {
			if m.selectedPR != nil {
				if len(m.availableReviews) > 0 && m.selectedReviewIdx < len(m.availableReviews) {
					target := m.availableReviews[m.selectedReviewIdx]
					m.confirmDeleteModal = true
					m.deleteTargetAll = false
					m.deleteTargetReview = &target
					return m, nil
				} else if m.reviewContent != "" && m.reviewFilePath != "" {
					target := review.SavedReview{
						FilePath: m.reviewFilePath,
						FileName: filepath.Base(m.reviewFilePath),
					}
					m.confirmDeleteModal = true
					m.deleteTargetAll = false
					m.deleteTargetReview = &target
					return m, nil
				} else {
					m.statusMsg = "No cached review file to delete for this PR."
					m.statusIsErr = true
					return m, nil
				}
			}
			return m, nil
		}
		if m.matches(k, m.cfg.Keybindings.DeleteAllReviews) {
			if m.selectedPR != nil {
				if len(m.availableReviews) > 0 {
					m.confirmDeleteModal = true
					m.deleteTargetAll = true
					m.deleteTargetReview = nil
					return m, nil
				} else {
					m.statusMsg = "No cached reviews found to delete for this PR."
					m.statusIsErr = true
					return m, nil
				}
			}
			return m, nil
		}
		if m.matches(k, m.cfg.Keybindings.RunReview) {
			if m.selectedPR != nil {
				m.loading = true
				m.loadingMsg = fmt.Sprintf("🤖 Generating AI review for #%d with %s...", m.selectedPR.Number, m.activeAIChoice().DisplayName)
				m.activeTab = tabReview
				return m, m.runAIReviewCmd(m.currentProject(), m.selectedPR.Number)
			}
			return m, nil
		}
		if m.matches(k, m.cfg.Keybindings.PostReview) {
			if m.selectedPR != nil {
				if m.compareMode && len(m.availableReviews) >= 2 {
					activeIdx := m.compareLeftIdx
					if m.compareActiveCol == 1 {
						activeIdx = m.compareRightIdx
					}
					if activeIdx < len(m.availableReviews) {
						m.reviewContent = m.availableReviews[activeIdx].Content
						m.reviewFilePath = m.availableReviews[activeIdx].FilePath
					}
				}
				if strings.TrimSpace(m.reviewContent) == "" {
					m.statusMsg = "No review content available. Press [r] to generate review first."
					m.statusIsErr = true
					return m, nil
				}
				m.confirmPostModal = true
				return m, nil
			}
			return m, nil
		}

		// Pane Navigation Keybindings
		if m.activePane == panePRList {
			if m.matches(k, m.cfg.Keybindings.Down) {
				m.table.MoveDown(1)
				m.onPRSelected(m.table.Cursor())
				return m, nil
			}
			if m.matches(k, m.cfg.Keybindings.Up) {
				m.table.MoveUp(1)
				m.onPRSelected(m.table.Cursor())
				return m, nil
			}
			if m.matches(k, m.cfg.Keybindings.HalfPageDown) || m.matches(k, m.cfg.Keybindings.PageDown) {
				m.table.MoveDown(10)
				m.onPRSelected(m.table.Cursor())
				return m, nil
			}
			if m.matches(k, m.cfg.Keybindings.HalfPageUp) || m.matches(k, m.cfg.Keybindings.PageUp) {
				m.table.MoveUp(10)
				m.onPRSelected(m.table.Cursor())
				return m, nil
			}
			if m.matches(k, m.cfg.Keybindings.Top) {
				m.table.GotoTop()
				m.onPRSelected(m.table.Cursor())
				return m, nil
			}
			if m.matches(k, m.cfg.Keybindings.Bottom) {
				m.table.GotoBottom()
				m.onPRSelected(m.table.Cursor())
				return m, nil
			}

			// Fallback to internal table update for unmapped keys
			var cmd tea.Cmd
			prevCursor := m.table.Cursor()
			m.table, cmd = m.table.Update(msg)
			if m.table.Cursor() != prevCursor && len(m.filteredPRs) > 0 {
				m.onPRSelected(m.table.Cursor())
			}
			return m, cmd
		}

		if m.activePane == paneDetail {
			if m.compareMode && m.activeTab == tabReview {
				if k == "tab" || k == "h" || k == "l" || k == "left" || k == "right" {
					m.compareActiveCol = 1 - m.compareActiveCol
					return m, nil
				}
				if k == "esc" || m.matches(k, m.cfg.Keybindings.CompareReviews) {
					m.compareMode = false
					m.statusMsg = "Exited Compare Mode."
					m.statusIsErr = false
					return m, nil
				}
			}

			vp := m.activeViewport()
			if vp != nil {
				if m.matches(k, m.cfg.Keybindings.Down) {
					vp.LineDown(1)
					return m, nil
				}
				if m.matches(k, m.cfg.Keybindings.Up) {
					vp.LineUp(1)
					return m, nil
				}
				if m.matches(k, m.cfg.Keybindings.HalfPageDown) {
					vp.HalfViewDown()
					return m, nil
				}
				if m.matches(k, m.cfg.Keybindings.HalfPageUp) {
					vp.HalfViewUp()
					return m, nil
				}
				if m.matches(k, m.cfg.Keybindings.PageDown) {
					vp.ViewDown()
					return m, nil
				}
				if m.matches(k, m.cfg.Keybindings.PageUp) {
					vp.ViewUp()
					return m, nil
				}
				if m.matches(k, m.cfg.Keybindings.Top) {
					vp.GotoTop()
					return m, nil
				}
				if m.matches(k, m.cfg.Keybindings.Bottom) {
					vp.GotoBottom()
					return m, nil
				}

				var cmd tea.Cmd
				*vp, cmd = vp.Update(msg)
				return m, cmd
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) matches(key string, list config.KeyList) bool {
	for _, k := range list {
		if k == key {
			return true
		}
	}
	return false
}

func (m *Model) activeViewport() *viewport.Model {
	switch m.activeTab {
	case tabOverview:
		return &m.overviewViewport
	case tabDiff:
		return &m.diffViewport
	case tabReview:
		if m.compareMode && len(m.availableReviews) >= 2 {
			if m.compareActiveCol == 0 {
				return &m.compareLeftViewport
			}
			return &m.compareRightViewport
		}
		return &m.reviewViewport
	default:
		return nil
	}
}

func (m *Model) onPRSelected(idx int) {
	if idx < 0 || idx >= len(m.filteredPRs) {
		return
	}
	pr := m.filteredPRs[idx]
	m.selectedPR = pr

	// 1. Discover all saved reviews for this PR
	m.availableReviews = m.service.ListReviewsForPR(m.currentProject(), pr.Number)
	m.selectedReviewIdx = 0
	m.compareLeftIdx = 0
	m.compareRightIdx = 1

	if len(m.availableReviews) > 0 {
		rev := m.availableReviews[0]
		m.reviewContent = rev.Content
		m.reviewFilePath = rev.FilePath
		m.reviewIsCached = true
	} else {
		m.reviewContent = ""
		m.reviewFilePath = m.service.ReviewFilePathWithAI(m.currentProject(), pr.Number, m.activeAIChoice().Provider, m.activeAIChoice().Model)
		m.reviewIsCached = false
		m.compareMode = false
	}

	// 2. Update viewports
	m.updateOverviewViewport()
	m.updateReviewViewport()
	m.updateCompareViewports()

	// 3. Diff Viewport
	if diff, ok := m.diffCache[pr.Number]; ok {
		m.diffViewport.SetContent(formatDiffText(diff))
	} else {
		m.diffViewport.SetContent(lipgloss.NewStyle().Foreground(mutedColor).Render("Press [2] or [d] to load diff..."))
		if m.activeTab == tabDiff {
			m.diffLoading = true
		}
	}
}

func (m *Model) updateCompareViewports() {
	if len(m.availableReviews) < 2 {
		m.compareLeftViewport.SetContent("Need at least 2 saved reviews to compare.")
		m.compareRightViewport.SetContent("Need at least 2 saved reviews to compare.")
		return
	}
	if m.compareLeftIdx >= len(m.availableReviews) {
		m.compareLeftIdx = 0
	}
	if m.compareRightIdx >= len(m.availableReviews) {
		m.compareRightIdx = 1
	}
	if m.compareLeftIdx == m.compareRightIdx {
		if m.compareLeftIdx == 0 {
			m.compareRightIdx = 1
		} else {
			m.compareLeftIdx = 0
		}
	}

	leftRev := m.availableReviews[m.compareLeftIdx]
	rightRev := m.availableReviews[m.compareRightIdx]

	colW := m.compareLeftViewport.Width
	if colW < 20 {
		colW = 40
	}
	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(colW-2),
	)

	leftRendered, err := r.Render(leftRev.Content)
	if err != nil {
		leftRendered = leftRev.Content
	}
	m.compareLeftViewport.SetContent(leftRendered)

	rightRendered, err := r.Render(rightRev.Content)
	if err != nil {
		rightRendered = rightRev.Content
	}
	m.compareRightViewport.SetContent(rightRendered)
}

func (m *Model) updateOverviewViewport() {
	if m.selectedPR == nil {
		m.overviewViewport.SetContent("No PR selected")
		return
	}

	pr := m.selectedPR
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# #%d: %s\n\n", pr.Number, pr.Title))
	b.WriteString(fmt.Sprintf("- **Author**: `%s`\n", pr.Author))
	b.WriteString(fmt.Sprintf("- **Source Branch**: `%s`\n", pr.SourceBranch))
	b.WriteString(fmt.Sprintf("- **Target Branch**: `%s`\n", pr.TargetBranch))
	b.WriteString(fmt.Sprintf("- **Status**: `%s`\n", pr.State))
	if len(pr.Labels) > 0 {
		b.WriteString(fmt.Sprintf("- **Labels**: %s\n", strings.Join(pr.Labels, ", ")))
	}
	b.WriteString(fmt.Sprintf("- **URL**: %s\n\n", pr.URL))

	if strings.TrimSpace(pr.Description) != "" {
		b.WriteString("## Description\n\n")
		b.WriteString(pr.Description)
		b.WriteString("\n")
	}

	rendered, err := m.renderer.Render(b.String())
	if err != nil {
		rendered = b.String()
	}
	m.overviewViewport.SetContent(rendered)
}

func (m Model) activeAIChoice() AIProviderChoice {
	for _, c := range m.aiProviders {
		if strings.EqualFold(c.ID, m.selectedAIProvider) {
			return c
		}
	}
	for _, c := range m.aiProviders {
		if strings.EqualFold(c.Provider, m.selectedAIProvider) {
			return c
		}
	}
	if len(m.aiProviders) > 0 {
		return m.aiProviders[0]
	}
	return AIProviderChoice{ID: "ollama", Provider: "ollama", DisplayName: "Ollama (Local AI)", Model: "qwen2.5-coder:latest"}
}

func (m *Model) updateReviewViewport() {
	if m.selectedPR == nil {
		m.reviewViewport.SetContent("No PR selected")
		return
	}

	activeAI := m.activeAIChoice()

	if strings.TrimSpace(m.reviewContent) == "" {
		placeholder := fmt.Sprintf(
			"## 🤖 AI Code Review\n\nNo review file found at `%s`.\n\nPress **[r]** to generate an AI review using **%s** (`%s`).\nPress **[a]** to switch AI review engine.",
			m.reviewFilePath,
			activeAI.DisplayName,
			activeAI.Model,
		)
		rendered, _ := m.renderer.Render(placeholder)
		m.reviewViewport.SetContent(rendered)
		return
	}

	var header string
	totalRevs := len(m.availableReviews)
	if totalRevs > 1 && m.selectedReviewIdx < totalRevs {
		currRev := m.availableReviews[m.selectedReviewIdx]
		providerLabel := currRev.Provider
		if providerLabel == "" {
			providerLabel = "local"
		}
		modelLabel := currRev.Model
		if modelLabel == "" {
			modelLabel = "default"
		}
		header = fmt.Sprintf("> 📄 **Review [%d/%d] from** `%s` (Engine: **%s** / **%s**)\n> Generated: %s\n> Press **[ [ / ] ]** to cycle reviews, **[c]** for side-by-side compare, **[v]** for list, **[r]** to generate new, **[P]** to post.\n\n",
			m.selectedReviewIdx+1, totalRevs, currRev.FileName, providerLabel, modelLabel, currRev.CreatedAt.Format("2006-01-02 15:04"))
	} else if m.reviewIsCached {
		header = fmt.Sprintf("> 📄 **Loaded cached review from** `%s`\n> Press **[r]** to generate new review with **%s** (`%s`), **[a]** to switch engine, **[P]** to post comment.\n\n", m.reviewFilePath, activeAI.DisplayName, activeAI.Model)
	} else {
		header = fmt.Sprintf("> ✨ **Fresh AI Review generated by %s (%s) & saved to** `%s`\n> Press **[P]** to post as a comment to PR #%d, or **[a]** to switch AI engine.\n\n", activeAI.DisplayName, activeAI.Model, m.reviewFilePath, m.selectedPR.Number)
	}

	fullMarkdown := header + m.reviewContent
	rendered, err := m.renderer.Render(fullMarkdown)
	if err != nil {
		rendered = fullMarkdown
	}
	m.reviewViewport.SetContent(rendered)
}

func (m *Model) applyFilter() {
	term := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	if term == "" {
		m.filteredPRs = m.allPRs
	} else {
		var filtered []*gitprovider.PullRequest
		for _, pr := range m.allPRs {
			prStr := fmt.Sprintf("%d %s %s %s", pr.Number, pr.Title, pr.Author, strings.Join(pr.Labels, " "))
			if strings.Contains(strings.ToLower(prStr), term) {
				filtered = append(filtered, pr)
			}
		}
		m.filteredPRs = filtered
	}

	m.rebuildTableRows()
	if len(m.filteredPRs) > 0 {
		m.table.SetCursor(0)
		m.onPRSelected(0)
	} else {
		m.selectedPR = nil
		m.overviewViewport.SetContent("No PRs match filter")
		m.diffViewport.SetContent("")
		m.reviewViewport.SetContent("")
	}
}

func (m *Model) rebuildTableRows() {
	rows := make([]table.Row, len(m.filteredPRs))
	for i, pr := range m.filteredPRs {
		reviews := m.service.ListReviewsForPR(m.currentProject(), pr.Number)
		reviewIndicator := "  -"
		if len(reviews) == 1 {
			reviewIndicator = "  📄"
		} else if len(reviews) > 1 {
			reviewIndicator = fmt.Sprintf(" 📄(%d)", len(reviews))
		}
		title := pr.Title
		if len(title) > 36 {
			title = title[:33] + "..."
		}
		rows[i] = table.Row{
			fmt.Sprintf("#%d", pr.Number),
			title,
			pr.Author,
			reviewIndicator,
		}
	}
	m.table.SetRows(rows)
}

func (m *Model) updateLayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	headerHeight := 3
	footerHeight := 2
	contentHeight := m.height - headerHeight - footerHeight - 2
	if contentHeight < 10 {
		contentHeight = 10
	}

	// Split 40% left, 60% right
	leftWidth := (m.width * 42) / 100
	if leftWidth < 40 {
		leftWidth = 40
	}
	rightWidth := m.width - leftWidth - 4
	if rightWidth < 40 {
		rightWidth = 40
	}

	// Update table dimensions
	m.table.SetWidth(leftWidth - 4)
	m.table.SetHeight(contentHeight - 2)

	// Adjust column widths based on left pane
	col0 := 8
	col2 := 13
	col3 := 8
	col1 := leftWidth - col0 - col2 - col3 - 6
	if col1 < 15 {
		col1 = 15
	}
	m.table.SetColumns([]table.Column{
		{Title: "PR", Width: col0},
		{Title: "Title", Width: col1},
		{Title: "Author", Width: col2},
		{Title: "Review", Width: col3},
	})

	// Viewports
	vpHeight := contentHeight - 4
	if vpHeight < 5 {
		vpHeight = 5
	}
	m.overviewViewport.Width = rightWidth - 2
	m.overviewViewport.Height = vpHeight
	m.diffViewport.Width = rightWidth - 2
	m.diffViewport.Height = vpHeight
	m.reviewViewport.Width = rightWidth - 2
	m.reviewViewport.Height = vpHeight

	// Split compare viewports
	compareColW := (rightWidth - 6) / 2
	if compareColW < 20 {
		compareColW = 20
	}
	m.compareLeftViewport.Width = compareColW
	m.compareLeftViewport.Height = vpHeight - 2
	m.compareRightViewport.Width = compareColW
	m.compareRightViewport.Height = vpHeight - 2

	if m.renderer != nil {
		r, _ := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(rightWidth-6),
		)
		m.renderer = r
	}

	if m.selectedPR != nil {
		m.updateOverviewViewport()
		m.updateReviewViewport()
		m.updateCompareViewports()
	}
}

func formatDiffText(rawDiff string) string {
	lines := strings.Split(rawDiff, "\n")
	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
	delStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F87"))
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true)
	hunkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))

	var formatted strings.Builder
	for _, line := range lines {
		if strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			formatted.WriteString(headerStyle.Render(line) + "\n")
		} else if strings.HasPrefix(line, "@@") {
			formatted.WriteString(hunkStyle.Render(line) + "\n")
		} else if strings.HasPrefix(line, "+") {
			formatted.WriteString(addStyle.Render(line) + "\n")
		} else if strings.HasPrefix(line, "-") {
			formatted.WriteString(delStyle.Render(line) + "\n")
		} else {
			formatted.WriteString(line + "\n")
		}
	}
	return formatted.String()
}
