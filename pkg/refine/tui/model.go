package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/arxcruz/pr-review/pkg/ai"
	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/arxcruz/pr-review/pkg/jira"
	"github.com/arxcruz/pr-review/pkg/refine"
	"github.com/arxcruz/pr-review/pkg/session"
)

// Screen represents the active TUI screen.
type Screen int

const (
	ScreenPicker Screen = iota
	ScreenInterview
)

// PickerFocus indicates whether keyboard focus is on the ticket table or manual key input.
type PickerFocus int

const (
	FocusTable PickerFocus = iota
	FocusInput
)

// Messages
type recentTicketsLoadedMsg struct {
	tickets []jira.RecentTicket
	err     error
}

type snapshotCheckResultMsg struct {
	key       string
	ticket    *jira.Ticket
	snapshot  *session.Snapshot
	hasSnap   bool
	err       error
}

type frontierGeneratedMsg struct {
	questions []session.Question
	err       error
}

type refinementFinalizedMsg struct {
	tree *session.DecompositionTree
	err  error
}

// ResumeModalState manages the state of the resume-or-fresh confirmation dialog.
type ResumeModalState struct {
	Active   bool
	Key      string
	Snapshot *session.Snapshot
	Ticket   *jira.Ticket
}

// Model is the main Bubble Tea model for the Jira Refine TUI.
type Model struct {
	cfg          *config.Config
	jiraClient   jira.Client
	sessionStore session.Store
	aiEngine     ai.Engine
	refineEngine *refine.Engine
	router       *refine.Router
	planFile     string

	// UI layout & screen
	screen      Screen
	pickerFocus PickerFocus
	width       int
	height      int
	initialKey  string

	// Ticket picker data & widgets
	tickets    []jira.RecentTicket
	table      table.Model
	input      textinput.Model
	spinner    spinner.Model
	loading    bool
	loadingMsg string

	// Resume confirmation dialog
	resumeModal ResumeModalState

	// Active session (Refinement interview)
	activeSnapshot   *session.Snapshot
	interviewIndex   int
	interviewEditing bool
	interviewInput   textinput.Model
	viewport         viewport.Model
	frontierOpts     refine.FrontierOptions

	// Status messages
	statusMsg   string
	statusIsErr bool
}

// Option configures a Model.
type Option func(*Model)

// WithAIEngine sets the AI engine on the model.
func WithAIEngine(eng ai.Engine) Option {
	return func(m *Model) {
		m.aiEngine = eng
		if eng != nil {
			m.refineEngine = refine.NewEngine(eng)
		}
	}
}

// WithRouter sets the delivery project router on the model.
func WithRouter(r *refine.Router) Option {
	return func(m *Model) {
		m.router = r
	}
}

// WithFrontierOptions sets frontier generation options.
func WithFrontierOptions(opts refine.FrontierOptions) Option {
	return func(m *Model) {
		m.frontierOpts = opts
	}
}

// WithPlanFile sets the output plan file path.
func WithPlanFile(planFile string) Option {
	return func(m *Model) {
		m.planFile = planFile
	}
}

// WithInitialKey pre-selects a ticket key on startup.
func WithInitialKey(key string) Option {
	return func(m *Model) {
		m.initialKey = strings.TrimSpace(key)
	}
}

// NewModel creates an initialized Jira Refine TUI model.
func NewModel(cfg *config.Config, client jira.Client, store session.Store, opts ...Option) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(primaryColor)

	ti := textinput.New()
	ti.Placeholder = "Enter any Jira issue key (e.g. STRAT-123) and press Enter..."
	ti.CharLimit = 64
	ti.Width = 50

	interviewTi := textinput.New()
	interviewTi.Placeholder = "Enter custom answer or edit..."
	interviewTi.CharLimit = 256
	interviewTi.Width = 60

	vp := viewport.New(80, 20)

	columns := []table.Column{
		{Title: "Key", Width: 14},
		{Title: "Summary", Width: 46},
		{Title: "Status", Width: 14},
		{Title: "Assignee", Width: 18},
	}

	tbl := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(14),
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
	tbl.SetStyles(tStyle)

	m := Model{
		cfg:            cfg,
		jiraClient:     client,
		sessionStore:   store,
		screen:         ScreenPicker,
		pickerFocus:    FocusTable,
		table:          tbl,
		input:          ti,
		interviewInput: interviewTi,
		viewport:       vp,
		spinner:        s,
		loading:        true,
		loadingMsg:     "Loading recent tickets from Origin Project...",
		width:          80,
		height:         24,
	}

	for _, opt := range opts {
		opt(&m)
	}

	return m
}

// Accessor methods for inspectability and testing
func (m Model) Screen() Screen                        { return m.screen }
func (m Model) Loading() bool                         { return m.loading }
func (m Model) ResumeModalActive() bool               { return m.resumeModal.Active }
func (m Model) ActiveSnapshot() *session.Snapshot     { return m.activeSnapshot }
func (m Model) CurrentQuestionIndex() int             { return m.interviewIndex }
func (m Model) InterviewEditing() bool                { return m.interviewEditing }
func (m Model) Tickets() []jira.RecentTicket          { return m.tickets }
func (m Model) PickerFocus() PickerFocus              { return m.pickerFocus }
func (m Model) InputValue() string                    { return m.input.Value() }
func (m Model) StatusMsg() (string, bool)             { return m.statusMsg, m.statusIsErr }

// Init kicks off spinner and loading of recent tickets
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.spinner.Tick,
		m.loadRecentTicketsCmd(),
	}
	if m.initialKey != "" {
		cmds = append(cmds, m.checkSnapshotCmd(m.initialKey, nil))
	}
	return tea.Batch(cmds...)
}

// Commands
func (m Model) loadRecentTicketsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.jiraClient == nil {
			return recentTicketsLoadedMsg{err: errors.New("jira client not configured")}
		}
		ctx := context.Background()
		project := ""
		if m.cfg != nil {
			project = m.cfg.Jira.OriginProject
		}
		tickets, err := m.jiraClient.GetRecentTickets(ctx, project)
		return recentTicketsLoadedMsg{
			tickets: tickets,
			err:     err,
		}
	}
}

func (m Model) checkSnapshotCmd(key string, recentTicket *jira.RecentTicket) tea.Cmd {
	return func() tea.Msg {
		key = strings.ToUpper(strings.TrimSpace(key))
		if key == "" {
			return snapshotCheckResultMsg{key: key, err: errors.New("ticket key cannot be empty")}
		}

		ctx := context.Background()
		var existingSnap *session.Snapshot
		hasSnap := false

		if m.sessionStore != nil {
			snap, err := m.sessionStore.Load(key)
			if err == nil && snap != nil {
				existingSnap = snap
				hasSnap = true
			}
		}

		var ticket *jira.Ticket
		if hasSnap {
			ticket = &existingSnap.Ticket
		} else if m.jiraClient != nil {
			fetched, err := m.jiraClient.GetTicket(ctx, key)
			if err != nil && recentTicket != nil {
				ticket = &jira.Ticket{
					Key:     recentTicket.Key,
					Summary: recentTicket.Summary,
					Status:  recentTicket.Status,
				}
			} else if err != nil {
				return snapshotCheckResultMsg{key: key, err: fmt.Errorf("failed to fetch ticket %s: %w", key, err)}
			} else {
				ticket = fetched
			}
		} else if recentTicket != nil {
			ticket = &jira.Ticket{
				Key:     recentTicket.Key,
				Summary: recentTicket.Summary,
				Status:  recentTicket.Status,
			}
		} else {
			ticket = &jira.Ticket{
				Key:     key,
				Summary: key,
			}
		}

		return snapshotCheckResultMsg{
			key:      key,
			ticket:   ticket,
			snapshot: existingSnap,
			hasSnap:  hasSnap,
		}
	}
}

func (m Model) generateFrontierCmd() tea.Cmd {
	return func() tea.Msg {
		if m.refineEngine == nil {
			return frontierGeneratedMsg{err: errors.New("refinement AI engine not configured")}
		}
		if m.activeSnapshot == nil {
			return frontierGeneratedMsg{err: errors.New("no active session snapshot")}
		}
		ctx := context.Background()
		questions, err := m.refineEngine.GenerateFrontier(ctx, m.activeSnapshot, m.frontierOpts)
		return frontierGeneratedMsg{
			questions: questions,
			err:       err,
		}
	}
}

func (m Model) finalizeRefinementCmd() tea.Cmd {
	return func() tea.Msg {
		if m.activeSnapshot == nil {
			return refinementFinalizedMsg{err: errors.New("no active session snapshot")}
		}
		ctx := context.Background()
		var tree *session.DecompositionTree
		if m.activeSnapshot.Tree != nil {
			tree = m.activeSnapshot.Tree
		} else if m.refineEngine != nil {
			var err error
			tree, err = m.refineEngine.GenerateDecompositionTree(ctx, m.activeSnapshot, refine.DecompositionOptions{
				DocContext:  m.frontierOpts.DocContext,
				Guidelines:  m.frontierOpts.Guidelines,
				Model:       m.frontierOpts.Model,
				Temperature: m.frontierOpts.Temperature,
			})
			if err != nil {
				return refinementFinalizedMsg{err: err}
			}
		}

		if m.router != nil && m.activeSnapshot.Tree != nil {
			_ = m.router.Route(ctx, m.activeSnapshot)
		}

		m.activeSnapshot.Status = session.StatusFinalized
		if m.sessionStore != nil {
			_ = m.sessionStore.Save(m.activeSnapshot)
		}

		if m.planFile != "" && m.activeSnapshot.Tree != nil {
			var jiraCfg *config.JiraConfig
			if m.cfg != nil {
				jiraCfg = &m.cfg.Jira
			}
			planMd := refine.FormatPlanMarkdown(m.activeSnapshot, jiraCfg)
			_ = refine.WritePlanFile(m.planFile, planMd)
		}

		return refinementFinalizedMsg{tree: tree}
	}
}

// Update handles incoming messages and updates state
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case recentTicketsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Failed to load tickets: %v", msg.err)
			m.statusIsErr = true
		} else {
			m.tickets = msg.tickets
			m.populateTable()
			m.statusMsg = fmt.Sprintf("Loaded %d recent unresolved ticket(s)", len(msg.tickets))
			m.statusIsErr = false
		}

	case snapshotCheckResultMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error checking ticket: %v", msg.err)
			m.statusIsErr = true
			return m, nil
		}

		if msg.hasSnap {
			// Found existing local snapshot -> prompt user to resume or start fresh
			m.resumeModal = ResumeModalState{
				Active:   true,
				Key:      msg.key,
				Snapshot: msg.snapshot,
				Ticket:   msg.ticket,
			}
			m.statusMsg = fmt.Sprintf("Found existing refinement session for %s", msg.key)
			m.statusIsErr = false
			return m, nil
		}
		// No snapshot -> initialize fresh snapshot and transition to refinement interview
		cmd := m.startFreshSession(msg.key, msg.ticket)
		return m, cmd

	case frontierGeneratedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error generating frontier: %v", msg.err)
			m.statusIsErr = true
		} else {
			if m.sessionStore != nil && m.activeSnapshot != nil {
				_ = m.sessionStore.Save(m.activeSnapshot)
			}
			m.interviewIndex = 0
			if len(msg.questions) == 0 {
				m.statusMsg = "All frontier questions resolved. The refinement frontier is empty."
			} else {
				roundNum := 1
				if m.activeSnapshot != nil {
					roundNum = len(m.activeSnapshot.Rounds) + 1
				}
				m.statusMsg = fmt.Sprintf("Round %d generated (%d question(s))", roundNum, len(msg.questions))
			}
			m.statusIsErr = false
		}
		return m, nil

	case refinementFinalizedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error finalizing refinement: %v", msg.err)
			m.statusIsErr = true
		} else {
			epicCount := 0
			if msg.tree != nil {
				epicCount = len(msg.tree.Epics)
			}
			m.statusMsg = fmt.Sprintf("Refinement session finalized. Created %d epic(s).", epicCount)
			m.statusIsErr = false
		}
		return m, nil

	case tea.KeyMsg:
		k := msg.String()

		// Global quit
		if k == "ctrl+c" {
			return m, tea.Quit
		}

		// Prevent re-triggering while a network operation is in progress
		if m.loading && (k == "enter" || k == "r") {
			return m, nil
		}

		// Modal handling
		if m.resumeModal.Active {
			switch k {
			case "r", "R", "enter":
				// Resume existing session
				m.activeSnapshot = m.resumeModal.Snapshot
				m.resumeModal.Active = false
				m.screen = ScreenInterview
				if len(m.activeSnapshot.CurrentFrontier) == 0 && m.refineEngine != nil && m.activeSnapshot.Status != session.StatusFinalized {
					m.loading = true
					m.loadingMsg = fmt.Sprintf("Analyzing Strategic Ticket [%s] and generating frontier questions...", m.activeSnapshot.Key)
					return m, tea.Batch(m.spinner.Tick, m.generateFrontierCmd())
				}
				m.loading = false
				m.statusMsg = fmt.Sprintf("Resumed refinement session for %s", m.activeSnapshot.Key)
				m.statusIsErr = false
				return m, nil

			case "f", "F":
				// Start fresh: overwrite with fresh snapshot
				cmd := m.startFreshSession(m.resumeModal.Key, m.resumeModal.Ticket)
				return m, cmd

			case "esc", "q", "n", "N":
				// Cancel modal
				m.resumeModal.Active = false
				m.statusMsg = "Session selection cancelled"
				m.statusIsErr = false
				return m, nil
			}
			return m, nil
		}

		// Picker screen handling
		if m.screen == ScreenPicker {
			if m.pickerFocus == FocusInput {
				switch k {
				case "esc":
					m.pickerFocus = FocusTable
					m.input.Blur()
					m.table.Focus()
					return m, nil

				case "enter":
					val := strings.TrimSpace(m.input.Value())
					if val == "" {
						m.statusMsg = "Please enter a valid Jira issue key"
						m.statusIsErr = true
						return m, nil
					}
					m.loading = true
					m.loadingMsg = fmt.Sprintf("Checking session for %s...", val)
					return m, m.checkSnapshotCmd(val, nil)

				default:
					var cmd tea.Cmd
					m.input, cmd = m.input.Update(msg)
					return m, cmd
				}
			}

			// Table has focus
			switch k {
			case "q":
				return m, tea.Quit

			case "tab", "/":
				m.pickerFocus = FocusInput
				m.table.Blur()
				m.input.Focus()
				return m, textinput.Blink

			case "r":
				m.loading = true
				m.loadingMsg = "Refreshing recent tickets..."
				return m, m.loadRecentTicketsCmd()

			case "enter":
				selectedIdx := m.table.Cursor()
				if selectedIdx >= 0 && selectedIdx < len(m.tickets) {
					t := m.tickets[selectedIdx]
					m.loading = true
					m.loadingMsg = fmt.Sprintf("Checking session for %s...", t.Key)
					return m, m.checkSnapshotCmd(t.Key, &t)
				}

			default:
				var cmd tea.Cmd
				m.table, cmd = m.table.Update(msg)
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		// Interview screen handling
		if m.screen == ScreenInterview {
			if m.interviewEditing {
				switch k {
				case "esc":
					m.interviewEditing = false
					m.interviewInput.Blur()
					return m, nil

				case "enter":
					if m.activeSnapshot != nil && m.interviewIndex >= 0 && m.interviewIndex < len(m.activeSnapshot.CurrentFrontier) {
						val := strings.TrimSpace(m.interviewInput.Value())
						m.activeSnapshot.CurrentFrontier[m.interviewIndex].Answer = val
						if m.sessionStore != nil {
							_ = m.sessionStore.Save(m.activeSnapshot)
						}
						if m.interviewIndex < len(m.activeSnapshot.CurrentFrontier)-1 {
							m.interviewIndex++
						}
					}
					m.interviewEditing = false
					m.interviewInput.Blur()
					return m, nil

				default:
					var cmd tea.Cmd
					m.interviewInput, cmd = m.interviewInput.Update(msg)
					return m, cmd
				}
			}

			// Non-editing navigation mode
			switch k {
			case "esc", "b":
				// Return to picker
				m.screen = ScreenPicker
				m.statusMsg = "Returned to ticket picker"
				m.statusIsErr = false
				return m, nil

			case "q":
				return m, tea.Quit

			case "up", "k":
				if m.interviewIndex > 0 {
					m.interviewIndex--
				}
				return m, nil

			case "down", "j":
				if m.activeSnapshot != nil && m.interviewIndex < len(m.activeSnapshot.CurrentFrontier)-1 {
					m.interviewIndex++
				}
				return m, nil

			case "enter", "y":
				if m.activeSnapshot != nil && m.interviewIndex >= 0 && m.interviewIndex < len(m.activeSnapshot.CurrentFrontier) {
					q := &m.activeSnapshot.CurrentFrontier[m.interviewIndex]
					if q.Recommendation != "" {
						q.Answer = q.Recommendation
						if m.sessionStore != nil {
							_ = m.sessionStore.Save(m.activeSnapshot)
						}
						if m.interviewIndex < len(m.activeSnapshot.CurrentFrontier)-1 {
							m.interviewIndex++
						}
						m.statusMsg = fmt.Sprintf("Accepted recommendation for [%s]", q.ID)
						m.statusIsErr = false
					}
				}
				return m, nil

			case "1", "2", "3", "4", "5", "6", "7", "8", "9":
				if m.activeSnapshot != nil && m.interviewIndex >= 0 && m.interviewIndex < len(m.activeSnapshot.CurrentFrontier) {
					q := &m.activeSnapshot.CurrentFrontier[m.interviewIndex]
					optIdx := int(k[0] - '1')
					if optIdx >= 0 && optIdx < len(q.Options) {
						q.Answer = q.Options[optIdx]
						if m.sessionStore != nil {
							_ = m.sessionStore.Save(m.activeSnapshot)
						}
						if m.interviewIndex < len(m.activeSnapshot.CurrentFrontier)-1 {
							m.interviewIndex++
						}
						m.statusMsg = fmt.Sprintf("Selected option for [%s]", q.ID)
						m.statusIsErr = false
						return m, nil
					}
				}

			case "e":
				if m.activeSnapshot != nil && m.interviewIndex >= 0 && m.interviewIndex < len(m.activeSnapshot.CurrentFrontier) {
					q := &m.activeSnapshot.CurrentFrontier[m.interviewIndex]
					m.interviewEditing = true
					m.interviewInput.SetValue(q.Answer)
					if q.Recommendation != "" {
						m.interviewInput.Placeholder = fmt.Sprintf("Recommendation: %s", q.Recommendation)
					}
					m.interviewInput.Focus()
					return m, textinput.Blink
				}

			case "ctrl+s":
				if m.activeSnapshot != nil {
					m.activeSnapshot.AdvanceRound()
					if m.sessionStore != nil {
						_ = m.sessionStore.Save(m.activeSnapshot)
					}
					if m.refineEngine != nil {
						m.loading = true
						m.loadingMsg = fmt.Sprintf("Analyzing responses & generating next frontier round %d...", len(m.activeSnapshot.Rounds)+1)
						return m, tea.Batch(m.spinner.Tick, m.generateFrontierCmd())
					}
					m.statusMsg = fmt.Sprintf("Round %d submitted", len(m.activeSnapshot.Rounds))
					m.statusIsErr = false
					return m, nil
				}

			case "ctrl+f":
				if m.activeSnapshot != nil {
					m.loading = true
					m.loadingMsg = "Synthesizing Decomposition Tree from settled refinement rounds..."
					return m, tea.Batch(m.spinner.Tick, m.finalizeRefinementCmd())
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) populateTable() {
	rows := make([]table.Row, len(m.tickets))
	for i, t := range m.tickets {
		assignee := t.Assignee
		if assignee == "" {
			assignee = "Unassigned"
		}
		rows[i] = table.Row{
			t.Key,
			t.Summary,
			t.Status,
			assignee,
		}
	}
	m.table.SetRows(rows)
}

func (m *Model) startFreshSession(key string, ticket *jira.Ticket) tea.Cmd {
	snap := &session.Snapshot{
		Key:    key,
		Status: session.StatusNew,
	}
	if ticket != nil {
		snap.Ticket = *ticket
	}
	if m.sessionStore != nil {
		_ = m.sessionStore.Save(snap)
	}
	m.activeSnapshot = snap
	m.resumeModal.Active = false
	m.screen = ScreenInterview
	if m.refineEngine != nil {
		m.loading = true
		m.loadingMsg = fmt.Sprintf("Analyzing Strategic Ticket [%s] and generating frontier questions...", key)
		return tea.Batch(m.spinner.Tick, m.generateFrontierCmd())
	}
	m.loading = false
	m.statusMsg = fmt.Sprintf("Started refinement session for %s", key)
	m.statusIsErr = false
	return nil
}

func (m *Model) updateLayout() {
	tableHeight := m.height - 12
	if tableHeight < 5 {
		tableHeight = 5
	}
	m.table.SetHeight(tableHeight)

	contentWidth := m.width - 6
	if contentWidth < 40 {
		contentWidth = 40
	}
	m.input.Width = contentWidth - 4

	// Dynamically scale table columns
	keyWidth := 12
	statusWidth := 14
	assigneeWidth := 16
	summaryWidth := contentWidth - keyWidth - statusWidth - assigneeWidth - 6
	if summaryWidth < 18 {
		summaryWidth = 18
	}
	m.table.SetColumns([]table.Column{
		{Title: "Key", Width: keyWidth},
		{Title: "Summary", Width: summaryWidth},
		{Title: "Status", Width: statusWidth},
		{Title: "Assignee", Width: assigneeWidth},
	})

	vpHeight := m.height - 10
	if vpHeight < 5 {
		vpHeight = 5
	}
	m.viewport.Width = m.width - 4
	m.viewport.Height = vpHeight
}
