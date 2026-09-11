package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
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

type ticketDetailsLoadedMsg struct {
	ticket   *jira.Ticket
	snapshot *session.Snapshot
	err      error
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
	activeSnapshot *session.Snapshot

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
		cfg:          cfg,
		jiraClient:   client,
		sessionStore: store,
		screen:       ScreenPicker,
		pickerFocus:  FocusTable,
		table:        tbl,
		input:        ti,
		spinner:      s,
		loading:      true,
		loadingMsg:   "Loading recent tickets from Origin Project...",
		width:        80,
		height:       24,
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
			if err != nil {
				return snapshotCheckResultMsg{key: key, err: fmt.Errorf("failed to fetch ticket %s: %w", key, err)}
			}
			ticket = fetched
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
		} else {
			// No snapshot -> initialize fresh snapshot and transition to refinement interview
			snap := &session.Snapshot{
				Key:    msg.key,
				Status: session.StatusNew,
			}
			if msg.ticket != nil {
				snap.Ticket = *msg.ticket
			}
			if m.sessionStore != nil {
				_ = m.sessionStore.Save(snap)
			}
			m.activeSnapshot = snap
			m.screen = ScreenInterview
			m.statusMsg = fmt.Sprintf("Started refinement session for %s", msg.key)
			m.statusIsErr = false
		}

	case tea.KeyMsg:
		k := msg.String()

		// Global quit
		if k == "ctrl+c" {
			return m, tea.Quit
		}

		// Modal handling
		if m.resumeModal.Active {
			switch k {
			case "r", "R", "enter":
				// Resume existing session
				m.activeSnapshot = m.resumeModal.Snapshot
				m.resumeModal.Active = false
				m.screen = ScreenInterview
				m.statusMsg = fmt.Sprintf("Resumed refinement session for %s", m.activeSnapshot.Key)
				m.statusIsErr = false
				return m, nil

			case "f", "F":
				// Start fresh: overwrite with fresh snapshot
				freshSnap := &session.Snapshot{
					Key:    m.resumeModal.Key,
					Status: session.StatusNew,
				}
				if m.resumeModal.Ticket != nil {
					freshSnap.Ticket = *m.resumeModal.Ticket
				}
				if m.sessionStore != nil {
					_ = m.sessionStore.Save(freshSnap)
				}
				m.activeSnapshot = freshSnap
				m.resumeModal.Active = false
				m.screen = ScreenInterview
				m.statusMsg = fmt.Sprintf("Started fresh refinement session for %s", m.resumeModal.Key)
				m.statusIsErr = false
				return m, nil

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

		// Interview screen handling (for transitions and back navigation)
		if m.screen == ScreenInterview {
			switch k {
			case "esc", "b":
				// Return to picker
				m.screen = ScreenPicker
				m.statusMsg = "Returned to ticket picker"
				m.statusIsErr = false
				return m, nil
			case "q":
				return m, tea.Quit
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
}
