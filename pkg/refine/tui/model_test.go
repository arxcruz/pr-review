package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/arxcruz/pr-review/pkg/jira"
	"github.com/arxcruz/pr-review/pkg/session"
)

type mockJiraClient struct {
	recentTickets []jira.RecentTicket
	ticketMap     map[string]*jira.Ticket
	recentErr     error
	getErr        error
}

func (m *mockJiraClient) GetTicket(ctx context.Context, key string) (*jira.Ticket, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	t, ok := m.ticketMap[key]
	if !ok {
		return &jira.Ticket{Key: key, Summary: "Fetched " + key}, nil
	}
	return t, nil
}

func (m *mockJiraClient) GetRecentTickets(ctx context.Context, project string) ([]jira.RecentTicket, error) {
	if m.recentErr != nil {
		return nil, m.recentErr
	}
	return m.recentTickets, nil
}

func (m *mockJiraClient) CreateIssue(ctx context.Context, req jira.CreateIssueRequest) (*jira.CreatedIssue, error) {
	return nil, nil
}
func (m *mockJiraClient) CreateEpic(ctx context.Context, req jira.CreateEpicRequest) (*jira.CreatedIssue, error) {
	return nil, nil
}
func (m *mockJiraClient) CreateTask(ctx context.Context, req jira.CreateTaskRequest) (*jira.CreatedIssue, error) {
	return nil, nil
}
func (m *mockJiraClient) CreateIssueLink(ctx context.Context, req jira.CreateIssueLinkRequest) error {
	return nil
}
func (m *mockJiraClient) CreateDependencyLink(ctx context.Context, blockedKey, blockerKey string) error {
	return nil
}
func (m *mockJiraClient) SetParentLink(ctx context.Context, childKey, parentKey string) error {
	return nil
}
func (m *mockJiraClient) LinkStrategicTicket(ctx context.Context, req jira.StrategicLinkRequest) (*jira.StrategicLinkResult, error) {
	return nil, nil
}

func setupTestModel(t *testing.T) (Model, *mockJiraClient, session.Store) {
	cfg := &config.Config{
		Jira: config.JiraConfig{
			OriginProject: "STRAT",
		},
	}
	client := &mockJiraClient{
		recentTickets: []jira.RecentTicket{
			{Key: "STRAT-1", Summary: "Ticket 1", Status: "Open", Assignee: "Alice"},
			{Key: "STRAT-2", Summary: "Ticket 2", Status: "In Progress", Assignee: "Bob"},
		},
		ticketMap: map[string]*jira.Ticket{
			"STRAT-1": {Key: "STRAT-1", Summary: "Ticket 1", Status: "Open"},
			"STRAT-2": {Key: "STRAT-2", Summary: "Ticket 2", Status: "In Progress"},
			"PROJ-99": {Key: "PROJ-99", Summary: "Custom Ticket 99", Status: "To Do"},
		},
	}
	store := session.NewFileStore(t.TempDir())

	m := NewModel(cfg, client, store)
	return m, client, store
}

func TestNewPickerModel_InitialState(t *testing.T) {
	m, _, _ := setupTestModel(t)

	if m.Screen() != ScreenPicker {
		t.Fatalf("expected ScreenPicker, got %v", m.Screen())
	}
	if !m.Loading() {
		t.Fatalf("expected loading to be true initially")
	}
	if m.PickerFocus() != FocusTable {
		t.Fatalf("expected table focus initially")
	}

	initCmd := m.Init()
	if initCmd == nil {
		t.Fatalf("expected non-nil Init command")
	}
}

func TestPicker_TicketsLoaded(t *testing.T) {
	m, client, _ := setupTestModel(t)

	// Simulate tickets loaded msg
	updated, _ := m.Update(recentTicketsLoadedMsg{
		tickets: client.recentTickets,
	})
	m = updated.(Model)

	if m.Loading() {
		t.Fatalf("expected loading to be false after tickets loaded")
	}
	if len(m.Tickets()) != 2 {
		t.Fatalf("expected 2 tickets, got %d", len(m.Tickets()))
	}
}

func TestPicker_FocusToggle(t *testing.T) {
	m, client, _ := setupTestModel(t)
	updated, _ := m.Update(recentTicketsLoadedMsg{tickets: client.recentTickets})
	m = updated.(Model)

	// Press '/' to switch to input
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(Model)
	if m.PickerFocus() != FocusInput {
		t.Fatalf("expected FocusInput, got %v", m.PickerFocus())
	}

	// Press Esc to switch back to table
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.PickerFocus() != FocusTable {
		t.Fatalf("expected FocusTable, got %v", m.PickerFocus())
	}

	// Press Tab to switch to input
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.PickerFocus() != FocusInput {
		t.Fatalf("expected FocusInput after Tab, got %v", m.PickerFocus())
	}
}

func TestPicker_SelectTicket_NoSnapshot_TransitionsToInterview(t *testing.T) {
	m, client, _ := setupTestModel(t)
	updated, _ := m.Update(recentTicketsLoadedMsg{tickets: client.recentTickets})
	m = updated.(Model)

	// Enter on table row 0 (STRAT-1)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil {
		t.Fatalf("expected checkSnapshotCmd")
	}

	// Execute cmd
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.Screen() != ScreenInterview {
		t.Fatalf("expected ScreenInterview, got %v", m.Screen())
	}
	if m.ActiveSnapshot() == nil {
		t.Fatalf("expected active snapshot to be created")
	}
	if m.ActiveSnapshot().Key != "STRAT-1" {
		t.Fatalf("expected key STRAT-1, got %s", m.ActiveSnapshot().Key)
	}
	if m.ResumeModalActive() {
		t.Fatalf("resume modal should not be active when no previous snapshot exists")
	}
}

func TestPicker_SelectTicket_ExistingSnapshot_ShowsResumeModal(t *testing.T) {
	m, client, store := setupTestModel(t)

	// Pre-create existing snapshot for STRAT-1
	existingSnap := &session.Snapshot{
		Key:    "STRAT-1",
		Status: session.StatusInProgress,
		Ticket: jira.Ticket{
			Key:     "STRAT-1",
			Summary: "Ticket 1 Existing",
		},
		Rounds: []session.Round{
			{Number: 1},
		},
	}
	if err := store.Save(existingSnap); err != nil {
		t.Fatalf("failed to save existing snapshot: %v", err)
	}

	updated, _ := m.Update(recentTicketsLoadedMsg{tickets: client.recentTickets})
	m = updated.(Model)

	// Enter on table row 0 (STRAT-1)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil {
		t.Fatalf("expected checkSnapshotCmd")
	}

	// Execute cmd
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if !m.ResumeModalActive() {
		t.Fatalf("expected resume modal to be active for existing snapshot")
	}
	if m.Screen() != ScreenPicker {
		t.Fatalf("expected to remain on ScreenPicker while modal is open, got %v", m.Screen())
	}
}

func TestPicker_ResumeModal_ResumeChoice(t *testing.T) {
	m, client, store := setupTestModel(t)

	existingSnap := &session.Snapshot{
		Key:    "STRAT-1",
		Status: session.StatusInProgress,
		Ticket: jira.Ticket{
			Key:     "STRAT-1",
			Summary: "Ticket 1 Existing",
		},
		Rounds: []session.Round{
			{Number: 1},
		},
	}
	_ = store.Save(existingSnap)

	updated, _ := m.Update(recentTicketsLoadedMsg{tickets: client.recentTickets})
	m = updated.(Model)

	// Trigger selection
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if !m.ResumeModalActive() {
		t.Fatalf("expected resume modal active")
	}

	// Press 'r' to resume
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = updated.(Model)

	if m.ResumeModalActive() {
		t.Fatalf("expected resume modal to be closed")
	}
	if m.Screen() != ScreenInterview {
		t.Fatalf("expected ScreenInterview, got %v", m.Screen())
	}
	if m.ActiveSnapshot() == nil {
		t.Fatalf("expected active snapshot")
	}
	if len(m.ActiveSnapshot().Rounds) != 1 {
		t.Fatalf("expected preserved round count 1, got %d", len(m.ActiveSnapshot().Rounds))
	}
	if m.ActiveSnapshot().Status != session.StatusInProgress {
		t.Fatalf("expected status in-progress, got %s", m.ActiveSnapshot().Status)
	}
}

func TestPicker_ResumeModal_StartFreshChoice(t *testing.T) {
	m, client, store := setupTestModel(t)

	existingSnap := &session.Snapshot{
		Key:    "STRAT-1",
		Status: session.StatusInProgress,
		Ticket: jira.Ticket{
			Key:     "STRAT-1",
			Summary: "Ticket 1 Existing",
		},
		Rounds: []session.Round{
			{Number: 1},
		},
	}
	_ = store.Save(existingSnap)

	updated, _ := m.Update(recentTicketsLoadedMsg{tickets: client.recentTickets})
	m = updated.(Model)

	// Trigger selection
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	// Press 'f' to start fresh
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = updated.(Model)

	if m.ResumeModalActive() {
		t.Fatalf("expected resume modal closed")
	}
	if m.Screen() != ScreenInterview {
		t.Fatalf("expected ScreenInterview, got %v", m.Screen())
	}
	if m.ActiveSnapshot() == nil {
		t.Fatalf("expected active snapshot")
	}
	if len(m.ActiveSnapshot().Rounds) != 0 {
		t.Fatalf("expected fresh snapshot with 0 rounds, got %d", len(m.ActiveSnapshot().Rounds))
	}
	if m.ActiveSnapshot().Status != session.StatusNew {
		t.Fatalf("expected status new, got %s", m.ActiveSnapshot().Status)
	}
}

func TestPicker_ResumeModal_Cancel(t *testing.T) {
	m, client, store := setupTestModel(t)

	existingSnap := &session.Snapshot{
		Key:    "STRAT-1",
		Status: session.StatusInProgress,
	}
	_ = store.Save(existingSnap)

	updated, _ := m.Update(recentTicketsLoadedMsg{tickets: client.recentTickets})
	m = updated.(Model)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	// Press Esc to cancel
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.ResumeModalActive() {
		t.Fatalf("expected resume modal closed after cancel")
	}
	if m.Screen() != ScreenPicker {
		t.Fatalf("expected ScreenPicker after cancel, got %v", m.Screen())
	}
	if m.ActiveSnapshot() != nil {
		t.Fatalf("expected nil active snapshot after cancel")
	}
}

func TestPicker_ManualKeyInput_Submit(t *testing.T) {
	m, client, _ := setupTestModel(t)
	updated, _ := m.Update(recentTicketsLoadedMsg{tickets: client.recentTickets})
	m = updated.(Model)

	// Focus input
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)

	// Type PROJ-99
	for _, r := range "PROJ-99" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}

	if m.InputValue() != "PROJ-99" {
		t.Fatalf("expected input value PROJ-99, got %s", m.InputValue())
	}

	// Press Enter to submit
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil {
		t.Fatalf("expected checkSnapshotCmd")
	}

	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.Screen() != ScreenInterview {
		t.Fatalf("expected ScreenInterview, got %v", m.Screen())
	}
	if m.ActiveSnapshot() == nil || m.ActiveSnapshot().Key != "PROJ-99" {
		t.Fatalf("expected active snapshot for PROJ-99")
	}
}

func TestPicker_ViewRendering(t *testing.T) {
	m, client, _ := setupTestModel(t)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)
	updated, _ = m.Update(recentTicketsLoadedMsg{tickets: client.recentTickets})
	m = updated.(Model)

	viewStr := m.View()
	if !strings.Contains(viewStr, "Jira Refine") {
		t.Fatalf("expected header 'Jira Refine' in view")
	}
	if !strings.Contains(viewStr, "STRAT-1") || !strings.Contains(viewStr, "STRAT-2") {
		t.Fatalf("expected ticket keys in view")
	}
	if !strings.Contains(viewStr, "Manual Ticket Key Input") {
		t.Fatalf("expected manual ticket input section in view")
	}
	if !strings.Contains(viewStr, "Navigate") {
		t.Fatalf("expected help navigation in view")
	}
}

func TestPicker_InterviewView_BackNavigation(t *testing.T) {
	m, client, _ := setupTestModel(t)
	updated, _ := m.Update(recentTicketsLoadedMsg{tickets: client.recentTickets})
	m = updated.(Model)

	// Enter on table row 0
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.Screen() != ScreenInterview {
		t.Fatalf("expected ScreenInterview")
	}

	// Press 'b' to navigate back to picker
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(Model)

	if m.Screen() != ScreenPicker {
		t.Fatalf("expected ScreenPicker after pressing b, got %v", m.Screen())
	}
}

func TestPicker_WithInitialKey(t *testing.T) {
	cfg := &config.Config{
		Jira: config.JiraConfig{
			OriginProject: "STRAT",
		},
	}
	client := &mockJiraClient{
		ticketMap: map[string]*jira.Ticket{
			"INIT-1": {Key: "INIT-1", Summary: "Initial Ticket"},
		},
	}
	store := session.NewFileStore(t.TempDir())

	m := NewModel(cfg, client, store, WithInitialKey("INIT-1"))
	initCmd := m.Init()
	if initCmd == nil {
		t.Fatalf("expected non-nil initCmd")
	}

	// Executing checkSnapshotCmd directly
	cmd := m.checkSnapshotCmd("INIT-1", nil)
	msg := cmd()
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.Screen() != ScreenInterview {
		t.Fatalf("expected ScreenInterview, got %v", m.Screen())
	}
	if m.ActiveSnapshot() == nil || m.ActiveSnapshot().Key != "INIT-1" {
		t.Fatalf("expected active snapshot for INIT-1")
	}
}
