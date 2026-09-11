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
	ScreenTree
	ScreenSync
)

// SyncState indicates the current progress state of synchronization.
type SyncState int

const (
	SyncStateIdle SyncState = iota
	SyncStateRunning
	SyncStateFailed
	SyncStateSuccess
)

// TreeItemKind indicates whether a tree item is an Epic or Task.
type TreeItemKind int

const (
	TreeItemEpic TreeItemKind = iota
	TreeItemTask
)

// TreeEditField represents the active field being edited on a Decomposition Tree item.
type TreeEditField string

const (
	TreeEditFieldNone            TreeEditField = ""
	TreeEditFieldTitle           TreeEditField = "title"
	TreeEditFieldDescription     TreeEditField = "description"
	TreeEditFieldDeliveryProject TreeEditField = "project"
)

// TreeItem represents a flattened node in the Decomposition Tree for navigation and editing.
type TreeItem struct {
	Kind            TreeItemKind
	EpicIndex       int
	TaskIndex       int // -1 for Epic
	ID              string
	Key             string
	Title           string
	Description     string
	DeliveryProject string
	Type            string
	Excluded        bool
	DependsOn       []string
}

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

type syncStepResultMsg struct {
	stepIndex int
	step      refine.SyncStep
	err       error
}

// minTermWidth and minTermHeight are the smallest terminal dimensions this
// TUI's layout supports. Below this, sections are shown a "too small"
// message instead of a layout squeezed into a size it wasn't budgeted for.
const (
	minTermWidth  = 80
	minTermHeight = 24
)

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

	// Tree editor state
	treeIndex            int
	treeInput            textinput.Model
	treeEditing          bool
	treeEditField        TreeEditField
	syncProceedRequested bool

	// Sync execution state
	syncState       SyncState
	syncSteps       []refine.SyncStep
	syncCurrentStep int
	syncError       error
	syncIDMap       map[string]string
	syncViewport    viewport.Model

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

	treeTi := textinput.New()
	treeTi.Placeholder = "Enter value..."
	treeTi.CharLimit = 256
	treeTi.Width = 60

	vp := viewport.New(80, 20)

	tbl := table.New(table.WithFocused(true))

	// tStyle.Cell/Header keep bubbles/table's default Padding(0, 1) per
	// column; if that default ever changes here, update tableColumnChrome
	// in updateLayout to match, or column widths will drift out of sync
	// with contentWidth again.
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
		Bold(true).
		// Inline keeps the selected row on exactly one line even if its
		// rendered width momentarily exceeds the box's content width;
		// every unselected row already gets this from bubbles/table's
		// per-cell styling, but the row-level Selected style did not,
		// which let a wide row soft-wrap only when selected.
		Inline(true)
	tbl.SetStyles(tStyle)

	syncVp := viewport.New(80, 20)

	m := Model{
		cfg:            cfg,
		jiraClient:     client,
		sessionStore:   store,
		screen:         ScreenPicker,
		pickerFocus:    FocusTable,
		table:          tbl,
		input:          ti,
		interviewInput: interviewTi,
		treeInput:      treeTi,
		viewport:       vp,
		syncViewport:   syncVp,
		syncIDMap:      make(map[string]string),
		spinner:        s,
		loading:        true,
		loadingMsg:     "Loading recent tickets from Origin Project...",
		width:          80,
		height:         24,
	}

	for _, opt := range opts {
		opt(&m)
	}

	m.updateLayout()

	return m
}

// Accessor methods for inspectability and testing
func (m Model) Screen() Screen                        { return m.screen }
func (m Model) Loading() bool                         { return m.loading }
func (m Model) ResumeModalActive() bool               { return m.resumeModal.Active }
func (m Model) ActiveSnapshot() *session.Snapshot     { return m.activeSnapshot }
func (m Model) CurrentQuestionIndex() int             { return m.interviewIndex }
func (m Model) InterviewEditing() bool                { return m.interviewEditing }
func (m Model) TreeCursor() int                       { return m.treeIndex }
func (m Model) TreeEditing() bool                     { return m.treeEditing }
func (m Model) TreeEditField() TreeEditField          { return m.treeEditField }
func (m Model) SyncProceedRequested() bool            { return m.syncProceedRequested }
func (m Model) SyncState() SyncState                  { return m.syncState }
func (m Model) SyncSteps() []refine.SyncStep          { return m.syncSteps }
func (m Model) SyncCurrentStep() int                  { return m.syncCurrentStep }
func (m Model) SyncError() error                      { return m.syncError }
func (m Model) Tickets() []jira.RecentTicket          { return m.tickets }
func (m Model) PickerFocus() PickerFocus              { return m.pickerFocus }
func (m Model) InputValue() string                    { return m.input.Value() }
func (m Model) StatusMsg() (string, bool)             { return m.statusMsg, m.statusIsErr }

// TreeItems returns the flattened list of epics and tasks in the decomposition tree.
func (m Model) TreeItems() []TreeItem {
	if m.activeSnapshot == nil || m.activeSnapshot.Tree == nil {
		return nil
	}
	var items []TreeItem
	for eIdx, epic := range m.activeSnapshot.Tree.Epics {
		items = append(items, TreeItem{
			Kind:            TreeItemEpic,
			EpicIndex:       eIdx,
			TaskIndex:       -1,
			ID:              epic.ID,
			Key:             epic.Key,
			Title:           epic.Title,
			Description:     epic.Description,
			DeliveryProject: epic.DeliveryProject,
			Type:            epic.Type,
			Excluded:        epic.Excluded,
		})
		for tIdx, task := range epic.Tasks {
			taskExcluded := task.Excluded || epic.Excluded
			items = append(items, TreeItem{
				Kind:            TreeItemTask,
				EpicIndex:       eIdx,
				TaskIndex:       tIdx,
				ID:              task.ID,
				Key:             task.Key,
				Title:           task.Title,
				Description:     task.Description,
				DeliveryProject: task.DeliveryProject,
				Type:            task.Type,
				Excluded:        taskExcluded,
				DependsOn:       task.DependsOn,
			})
		}
	}
	return items
}

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
		if len(m.activeSnapshot.CurrentFrontier) > 0 {
			m.activeSnapshot.AdvanceRound()
		}

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

func (m Model) executeSyncStepCmd(stepIndex int) tea.Cmd {
	return func() tea.Msg {
		if m.activeSnapshot == nil || stepIndex < 0 || stepIndex >= len(m.syncSteps) {
			return syncStepResultMsg{stepIndex: stepIndex, err: errors.New("invalid sync step")}
		}

		step := m.syncSteps[stepIndex]
		var jiraCfg *config.JiraConfig
		if m.cfg != nil {
			jiraCfg = &m.cfg.Jira
		}
		syncer := refine.NewSyncer(m.jiraClient, m.sessionStore, jiraCfg)
		ctx := context.Background()
		err := syncer.ExecuteStep(ctx, m.activeSnapshot, &step, m.syncIDMap)
		return syncStepResultMsg{
			stepIndex: stepIndex,
			step:      step,
			err:       err,
		}
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

	case syncStepResultMsg:
		if msg.err != nil {
			m.syncState = SyncStateFailed
			m.syncError = msg.err
			if msg.stepIndex >= 0 && msg.stepIndex < len(m.syncSteps) {
				m.syncSteps[msg.stepIndex].Status = refine.StepFailed
				m.syncSteps[msg.stepIndex].Error = msg.err.Error()
			}
			m.loading = false
			m.statusMsg = fmt.Sprintf("Sync failed at step %d: %v", msg.stepIndex+1, msg.err)
			m.statusIsErr = true
			return m, nil
		}

		if msg.stepIndex >= 0 && msg.stepIndex < len(m.syncSteps) {
			m.syncSteps[msg.stepIndex] = msg.step
			if msg.step.Key != "" {
				m.syncIDMap[msg.step.ID] = msg.step.Key
			}
		}

		m.syncCurrentStep = msg.stepIndex + 1
		if m.syncCurrentStep >= len(m.syncSteps) {
			m.syncState = SyncStateSuccess
			m.loading = false
			if m.activeSnapshot != nil {
				m.activeSnapshot.Status = session.StatusSynced
				if m.sessionStore != nil {
					_ = m.sessionStore.Save(m.activeSnapshot)
				}
			}
			m.statusMsg = "All Jira issues and links synchronized successfully!"
			m.statusIsErr = false
			return m, nil
		}

		if m.syncCurrentStep < len(m.syncSteps) {
			m.syncSteps[m.syncCurrentStep].Status = refine.StepRunning
			m.loadingMsg = fmt.Sprintf("Executing step %d/%d: %s...", m.syncCurrentStep+1, len(m.syncSteps), m.syncSteps[m.syncCurrentStep].Title)
		}

		return m, m.executeSyncStepCmd(m.syncCurrentStep)

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
			m.screen = ScreenTree
			m.treeIndex = 0
			m.statusMsg = fmt.Sprintf("Decomposition Tree synthesized (%d epic(s)). Review and edit plan.", epicCount)
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
						if val == "" {
							val = m.activeSnapshot.CurrentFrontier[m.interviewIndex].Recommendation
						}
						m.recordCurrentQuestionAnswer(val)
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

			case "pgup", "pgdown", "u", "d":
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd

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
						m.recordCurrentQuestionAnswer(q.Recommendation)
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
						m.recordCurrentQuestionAnswer(q.Options[optIdx])
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

			case "v", "t":
				if m.activeSnapshot != nil && m.activeSnapshot.Tree != nil && len(m.activeSnapshot.Tree.Epics) > 0 {
					m.screen = ScreenTree
					m.treeIndex = 0
					m.statusMsg = "Navigated to Decomposition Tree plan review"
					m.statusIsErr = false
					return m, nil
				}
			}
		}

		// Tree editor screen handling
		if m.screen == ScreenTree {
			items := m.TreeItems()
			if len(items) == 0 {
				if k == "esc" || k == "b" {
					m.screen = ScreenInterview
					m.statusMsg = "Returned to interview"
					m.statusIsErr = false
					return m, nil
				}
				if k == "q" {
					return m, tea.Quit
				}
				return m, nil
			}

			if m.treeIndex >= len(items) {
				m.treeIndex = len(items) - 1
			}
			if m.treeIndex < 0 {
				m.treeIndex = 0
			}

			if m.treeEditing {
				switch k {
				case "esc":
					m.treeEditing = false
					m.treeInput.Blur()
					return m, nil

				case "enter":
					m.saveTreeItemEdit(m.treeInput.Value())
					return m, nil

				default:
					var cmd tea.Cmd
					m.treeInput, cmd = m.treeInput.Update(msg)
					return m, cmd
				}
			}

			switch k {
			case "esc", "b":
				m.screen = ScreenInterview
				m.statusMsg = "Returned to interview"
				m.statusIsErr = false
				return m, nil

			case "q":
				return m, tea.Quit

			case "up", "k":
				if m.treeIndex > 0 {
					m.treeIndex--
				}
				return m, nil

			case "down", "j":
				if m.treeIndex < len(items)-1 {
					m.treeIndex++
				}
				return m, nil

			case "pgup", "ctrl+u":
				m.treeIndex -= 5
				if m.treeIndex < 0 {
					m.treeIndex = 0
				}
				return m, nil

			case "pgdown", "ctrl+d":
				m.treeIndex += 5
				if m.treeIndex >= len(items) {
					m.treeIndex = len(items) - 1
				}
				return m, nil

			case " ", "x":
				m.toggleTreeItemInclusion()
				return m, nil

			case "e":
				if m.treeIndex >= 0 && m.treeIndex < len(items) {
					cmd := m.startTreeEdit(TreeEditFieldTitle, "Enter new title / summary...", items[m.treeIndex].Title)
					return m, cmd
				}

			case "d":
				if m.treeIndex >= 0 && m.treeIndex < len(items) {
					cmd := m.startTreeEdit(TreeEditFieldDescription, "Enter description...", items[m.treeIndex].Description)
					return m, cmd
				}

			case "p":
				if m.treeIndex >= 0 && m.treeIndex < len(items) {
					cmd := m.startTreeEdit(TreeEditFieldDeliveryProject, "Enter Delivery Project key (e.g. CORE)...", items[m.treeIndex].DeliveryProject)
					return m, cmd
				}

			case "s", "S":
				if m.activeSnapshot == nil || m.activeSnapshot.Tree == nil {
					m.statusMsg = "No decomposition tree available to sync"
					m.statusIsErr = true
					return m, nil
				}
				var jiraCfg *config.JiraConfig
				if m.cfg != nil {
					jiraCfg = &m.cfg.Jira
				}
				syncer := refine.NewSyncer(m.jiraClient, m.sessionStore, jiraCfg)
				if err := syncer.Verify(m.activeSnapshot); err != nil {
					m.statusMsg = fmt.Sprintf("Cannot sync: %v", err)
					m.statusIsErr = true
					return m, nil
				}
				steps, err := syncer.PlanSteps(m.activeSnapshot)
				if err != nil {
					m.statusMsg = fmt.Sprintf("Failed planning sync: %v", err)
					m.statusIsErr = true
					return m, nil
				}

				m.syncProceedRequested = true
				includedEpics := 0
				includedTasks := 0
				for _, e := range m.activeSnapshot.Tree.Epics {
					if !e.Excluded {
						includedEpics++
						for _, t := range e.Tasks {
							if !t.Excluded {
								includedTasks++
							}
						}
					}
				}
				m.statusMsg = fmt.Sprintf("Ready to synchronize %d epic(s) and %d task(s) with Jira.", includedEpics, includedTasks)
				m.statusIsErr = false

				m.screen = ScreenSync
				m.syncSteps = steps
				m.syncCurrentStep = 0
				m.syncState = SyncStateRunning
				m.syncError = nil
				m.syncIDMap = make(map[string]string)
				for _, e := range m.activeSnapshot.Tree.Epics {
					if e.Key != "" {
						m.syncIDMap[e.ID] = e.Key
					}
					for _, t := range e.Tasks {
						if t.Key != "" {
							m.syncIDMap[t.ID] = t.Key
						}
					}
				}
				m.loading = true
				if len(steps) > 0 {
					m.syncSteps[0].Status = refine.StepRunning
					m.loadingMsg = fmt.Sprintf("Executing step 1/%d: %s...", len(steps), steps[0].Title)
					return m, tea.Batch(m.spinner.Tick, m.executeSyncStepCmd(0))
				}
				m.syncState = SyncStateSuccess
				m.loading = false
				if m.activeSnapshot != nil {
					m.activeSnapshot.Status = session.StatusSynced
					if m.sessionStore != nil {
						_ = m.sessionStore.Save(m.activeSnapshot)
					}
				}
				return m, nil
			}
		}

		// Sync screen handling
		if m.screen == ScreenSync {
			switch k {
			case "q":
				if m.syncState == SyncStateSuccess || m.syncState == SyncStateFailed {
					return m, tea.Quit
				}
			case "r", "R":
				if m.syncState == SyncStateFailed {
					m.syncState = SyncStateRunning
					m.syncError = nil
					if m.syncCurrentStep < len(m.syncSteps) {
						m.syncSteps[m.syncCurrentStep].Status = refine.StepRunning
						m.syncSteps[m.syncCurrentStep].Error = ""
					}
					m.loading = true
					m.loadingMsg = fmt.Sprintf("Retrying step %d/%d: %s...", m.syncCurrentStep+1, len(m.syncSteps), m.syncSteps[m.syncCurrentStep].Title)
					m.statusMsg = fmt.Sprintf("Retrying step %d...", m.syncCurrentStep+1)
					m.statusIsErr = false
					return m, tea.Batch(m.spinner.Tick, m.executeSyncStepCmd(m.syncCurrentStep))
				}
			case "esc", "b":
				if m.syncState == SyncStateFailed || m.syncState == SyncStateSuccess {
					m.screen = ScreenTree
					m.statusMsg = "Returned to Decomposition Tree review"
					m.statusIsErr = false
					return m, nil
				}
			case "p", "P":
				if m.syncState == SyncStateSuccess {
					m.screen = ScreenPicker
					m.statusMsg = "Returned to Ticket Picker"
					m.statusIsErr = false
					return m, nil
				}
			case "up", "k", "down", "j", "pgup", "pgdown":
				var cmd tea.Cmd
				m.syncViewport, cmd = m.syncViewport.Update(msg)
				return m, cmd
			}
			return m, nil
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

func (m *Model) recordCurrentQuestionAnswer(ans string) {
	if m.activeSnapshot == nil || m.interviewIndex < 0 || m.interviewIndex >= len(m.activeSnapshot.CurrentFrontier) {
		return
	}
	m.activeSnapshot.CurrentFrontier[m.interviewIndex].Answer = ans
	if m.sessionStore != nil {
		_ = m.sessionStore.Save(m.activeSnapshot)
	}
	if m.interviewIndex < len(m.activeSnapshot.CurrentFrontier)-1 {
		m.interviewIndex++
	}
}

func (m *Model) toggleTreeItemInclusion() {
	if m.activeSnapshot == nil || m.activeSnapshot.Tree == nil {
		return
	}
	items := m.TreeItems()
	if m.treeIndex < 0 || m.treeIndex >= len(items) {
		return
	}
	item := items[m.treeIndex]
	if item.Kind == TreeItemEpic {
		if item.EpicIndex >= 0 && item.EpicIndex < len(m.activeSnapshot.Tree.Epics) {
			m.activeSnapshot.Tree.Epics[item.EpicIndex].Excluded = !m.activeSnapshot.Tree.Epics[item.EpicIndex].Excluded
		}
	} else if item.Kind == TreeItemTask {
		if item.EpicIndex >= 0 && item.EpicIndex < len(m.activeSnapshot.Tree.Epics) {
			epic := &m.activeSnapshot.Tree.Epics[item.EpicIndex]
			if epic.Excluded {
				epic.Excluded = false
			}
			if item.TaskIndex >= 0 && item.TaskIndex < len(epic.Tasks) {
				epic.Tasks[item.TaskIndex].Excluded = !epic.Tasks[item.TaskIndex].Excluded
			}
		}
	}
	if m.sessionStore != nil {
		_ = m.sessionStore.Save(m.activeSnapshot)
	}
	m.statusMsg = fmt.Sprintf("Toggled inclusion for %s", item.ID)
	m.statusIsErr = false
}

func (m *Model) startTreeEdit(field TreeEditField, placeholder string, val string) tea.Cmd {
	m.treeEditing = true
	m.treeEditField = field
	m.treeInput.Placeholder = placeholder
	m.treeInput.CharLimit = 4096
	m.treeInput.SetValue(val)
	m.treeInput.Focus()
	return textinput.Blink
}

func (m *Model) saveTreeItemEdit(val string) {
	if m.activeSnapshot == nil || m.activeSnapshot.Tree == nil {
		m.treeEditing = false
		m.treeInput.Blur()
		return
	}
	items := m.TreeItems()
	if m.treeIndex < 0 || m.treeIndex >= len(items) {
		m.treeEditing = false
		m.treeInput.Blur()
		return
	}
	item := items[m.treeIndex]
	val = strings.TrimSpace(val)

	switch m.treeEditField {
	case "title":
		if val != "" {
			if item.Kind == TreeItemEpic && item.EpicIndex < len(m.activeSnapshot.Tree.Epics) {
				m.activeSnapshot.Tree.Epics[item.EpicIndex].Title = val
			} else if item.Kind == TreeItemTask && item.EpicIndex < len(m.activeSnapshot.Tree.Epics) {
				epic := &m.activeSnapshot.Tree.Epics[item.EpicIndex]
				if item.TaskIndex < len(epic.Tasks) {
					epic.Tasks[item.TaskIndex].Title = val
				}
			}
		}
	case "description":
		if item.Kind == TreeItemEpic && item.EpicIndex < len(m.activeSnapshot.Tree.Epics) {
			m.activeSnapshot.Tree.Epics[item.EpicIndex].Description = val
		} else if item.Kind == TreeItemTask && item.EpicIndex < len(m.activeSnapshot.Tree.Epics) {
			epic := &m.activeSnapshot.Tree.Epics[item.EpicIndex]
			if item.TaskIndex < len(epic.Tasks) {
				epic.Tasks[item.TaskIndex].Description = val
			}
		}
	case "project":
		val = strings.ToUpper(val)
		if item.Kind == TreeItemEpic && item.EpicIndex < len(m.activeSnapshot.Tree.Epics) {
			m.activeSnapshot.Tree.Epics[item.EpicIndex].DeliveryProject = val
		} else if item.Kind == TreeItemTask && item.EpicIndex < len(m.activeSnapshot.Tree.Epics) {
			epic := &m.activeSnapshot.Tree.Epics[item.EpicIndex]
			if item.TaskIndex < len(epic.Tasks) {
				epic.Tasks[item.TaskIndex].DeliveryProject = val
			}
		}
	}

	if m.sessionStore != nil {
		_ = m.sessionStore.Save(m.activeSnapshot)
	}
	m.treeEditing = false
	m.treeInput.Blur()
	m.statusMsg = fmt.Sprintf("Updated %s on %s", m.treeEditField, item.ID)
	m.statusIsErr = false
}

// tableColumnChrome is how much horizontal space bubbles/table's default
// Cell/Header style (Padding(0, 1)) adds around each column beyond its
// declared Width: 1 column of padding on each side.
const tableColumnChrome = 2

// maxSummaryWidth caps the Summary column so it doesn't stretch to fill an
// arbitrarily wide terminal when actual ticket summaries are much shorter
// than the available space, leaving the column mostly blank.
const maxSummaryWidth = 100

func clampMin(v, min int) int {
	if v < min {
		return min
	}
	return v
}

func (m *Model) updateLayout() {
	// Table row height for the picker screen is computed at render time in
	// renderPickerView, from the actual rendered size of the surrounding
	// sections, rather than here — see the comment there for why.

	// contentWidth is the space available inside a box's border+padding
	// chrome: boxStyle declares Width(m.width-4) and adds Padding(0, 1),
	// which consumes 2 more columns.
	contentWidth := clampMin(m.width-6, 40)
	m.input.Width = contentWidth - 4

	// Dynamically scale table columns. keyWidth/statusWidth/assigneeWidth
	// are fixed; Summary absorbs the remainder, minus the per-column
	// Cell/Header padding chrome (tableColumnChrome per column, 4 columns)
	// so the row's true rendered width matches contentWidth exactly
	// instead of overflowing it.
	keyWidth := 12
	statusWidth := 14
	assigneeWidth := 16
	summaryWidth := contentWidth - keyWidth - statusWidth - assigneeWidth - 4*tableColumnChrome
	summaryWidth = clampMin(summaryWidth, 18)
	if summaryWidth > maxSummaryWidth {
		summaryWidth = maxSummaryWidth
	}
	m.table.SetColumns([]table.Column{
		{Title: "Key", Width: keyWidth},
		{Title: "Summary", Width: summaryWidth},
		{Title: "Status", Width: statusWidth},
		{Title: "Assignee", Width: assigneeWidth},
	})

	vpHeight := clampMin(m.height-10, 5)
	m.viewport.Width = clampMin(m.width-4, 40)
	m.viewport.Height = vpHeight
	m.syncViewport.Width = clampMin(m.width-4, 40)
	m.syncViewport.Height = vpHeight
}
