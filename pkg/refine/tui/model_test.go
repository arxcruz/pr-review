package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arxcruz/pr-review/pkg/ai"
	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/arxcruz/pr-review/pkg/jira"
	"github.com/arxcruz/pr-review/pkg/refine"
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

func TestPicker_SelectTicket_NoSnapshot_ShowsOverview(t *testing.T) {
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

	if m.Screen() != ScreenOverview {
		t.Fatalf("expected ScreenOverview, got %v", m.Screen())
	}
	if m.ResumeModalActive() {
		t.Fatalf("resume modal should not be active when no previous snapshot exists")
	}
	if m.PendingKey() != "STRAT-1" {
		t.Fatalf("expected pending key STRAT-1, got %s", m.PendingKey())
	}
	if m.PendingTicket() == nil {
		t.Fatalf("expected pending ticket to be set")
	}
	if m.ActiveSnapshot() != nil {
		t.Fatalf("expected no active snapshot before leaving the overview screen")
	}
}

func TestPicker_SelectTicket_NoSnapshot_TransitionsToInterview(t *testing.T) {
	m, client, _ := setupTestModel(t)
	updated, _ := m.Update(recentTicketsLoadedMsg{tickets: client.recentTickets})
	m = updated.(Model)

	// Enter on table row 0 (STRAT-1)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.Screen() != ScreenOverview {
		t.Fatalf("expected ScreenOverview, got %v", m.Screen())
	}

	// Skip the overview with Esc to proceed into the interview.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
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

func TestOverview_ContextNote_SubmittedViaCtrlS_SeedsFirstRound(t *testing.T) {
	m, client, _ := setupTestModel(t)
	updated, _ := m.Update(recentTicketsLoadedMsg{tickets: client.recentTickets})
	m = updated.(Model)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.Screen() != ScreenOverview {
		t.Fatalf("expected ScreenOverview, got %v", m.Screen())
	}

	// Type a Context Note.
	for _, r := range "Only mobile clients are in scope for this initiative." {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	if m.ContextNoteValue() != "Only mobile clients are in scope for this initiative." {
		t.Fatalf("expected context note to reflect typed text, got %q", m.ContextNoteValue())
	}

	// Submit with Ctrl+S.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)

	if m.Screen() != ScreenInterview {
		t.Fatalf("expected ScreenInterview, got %v", m.Screen())
	}
	snap := m.ActiveSnapshot()
	if snap == nil {
		t.Fatalf("expected active snapshot")
	}
	if len(snap.Rounds) != 1 {
		t.Fatalf("expected the Context Note to seed round 1, got %d rounds", len(snap.Rounds))
	}
	if len(snap.Rounds[0].Questions) != 1 || snap.Rounds[0].Questions[0].Answer != "Only mobile clients are in scope for this initiative." {
		t.Fatalf("expected round 1 to carry the context note as an answered question, got %+v", snap.Rounds[0].Questions)
	}
}

func TestOverview_SkipWithEsc_StartsFreshSessionWithoutExtraRound(t *testing.T) {
	m, client, _ := setupTestModel(t)
	updated, _ := m.Update(recentTicketsLoadedMsg{tickets: client.recentTickets})
	m = updated.(Model)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.Screen() != ScreenInterview {
		t.Fatalf("expected ScreenInterview, got %v", m.Screen())
	}
	snap := m.ActiveSnapshot()
	if snap == nil {
		t.Fatalf("expected active snapshot")
	}
	if len(snap.Rounds) != 0 {
		t.Fatalf("expected no rounds when the overview was skipped without a note, got %d", len(snap.Rounds))
	}
}

func TestOverview_CtrlG_ReturnsToPickerWithoutCreatingSnapshot(t *testing.T) {
	m, client, _ := setupTestModel(t)
	updated, _ := m.Update(recentTicketsLoadedMsg{tickets: client.recentTickets})
	m = updated.(Model)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.Screen() != ScreenOverview {
		t.Fatalf("expected ScreenOverview, got %v", m.Screen())
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	m = updated.(Model)

	if m.Screen() != ScreenPicker {
		t.Fatalf("expected ScreenPicker after Ctrl+G, got %v", m.Screen())
	}
	if m.ActiveSnapshot() != nil {
		t.Fatalf("expected no active snapshot after backing out of the overview")
	}
	if m.PendingKey() != "" || m.PendingTicket() != nil {
		t.Fatalf("expected pending ticket state cleared after backing out")
	}
}

func TestOverview_CtrlB_MovesCursorInsteadOfLeavingScreen(t *testing.T) {
	// Ctrl+B is bubbles/textarea's default "character backward" binding, so
	// the overview screen must leave it to the textarea (back-to-picker uses
	// Ctrl+G instead) rather than stealing it to exit the screen.
	m, client, _ := setupTestModel(t)
	updated, _ := m.Update(recentTicketsLoadedMsg{tickets: client.recentTickets})
	m = updated.(Model)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	m = updated.(Model)

	if m.Screen() != ScreenOverview {
		t.Fatalf("expected Ctrl+B to stay on ScreenOverview (handled by the textarea), got %v", m.Screen())
	}
}

func TestOverview_ThinDescription_ShowsWarning(t *testing.T) {
	m, client, _ := setupTestModel(t)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)
	updated, _ = m.Update(recentTicketsLoadedMsg{tickets: client.recentTickets})
	m = updated.(Model)

	// STRAT-1's mock ticket has no description at all.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	view := m.View()
	if !strings.Contains(view, "description is thin") {
		t.Fatalf("expected thin-description warning in overview view, got:\n%s", view)
	}
}

func TestOverview_AdequateDescription_NoWarning(t *testing.T) {
	cfg := &config.Config{Jira: config.JiraConfig{OriginProject: "STRAT"}}
	client := &mockJiraClient{
		recentTickets: []jira.RecentTicket{
			{Key: "STRAT-5", Summary: "Ticket 5", Status: "Open"},
		},
		ticketMap: map[string]*jira.Ticket{
			"STRAT-5": {
				Key:         "STRAT-5",
				Summary:     "Ticket 5",
				Status:      "Open",
				Description: strings.Repeat("This ticket has a properly detailed description. ", 3),
			},
		},
	}
	store := session.NewFileStore(t.TempDir())
	m := NewModel(cfg, client, store)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)
	updated, _ = m.Update(recentTicketsLoadedMsg{tickets: client.recentTickets})
	m = updated.(Model)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	view := m.View()
	if strings.Contains(view, "description is thin") {
		t.Fatalf("did not expect thin-description warning for a detailed description, got:\n%s", view)
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
	if m.Screen() != ScreenOverview {
		t.Fatalf("expected ScreenOverview before the fresh session begins, got %v", m.Screen())
	}
	if m.PendingKey() != "STRAT-1" {
		t.Fatalf("expected pending key STRAT-1, got %s", m.PendingKey())
	}

	// Skip the overview to actually start the fresh session.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

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

	if m.Screen() != ScreenOverview {
		t.Fatalf("expected ScreenOverview, got %v", m.Screen())
	}

	// Skip the overview to reach the interview.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
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

	// Skip the overview to reach the interview.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
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

	if m.Screen() != ScreenOverview {
		t.Fatalf("expected ScreenOverview, got %v", m.Screen())
	}

	// Skip the overview to reach the interview.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.Screen() != ScreenInterview {
		t.Fatalf("expected ScreenInterview, got %v", m.Screen())
	}
	if m.ActiveSnapshot() == nil || m.ActiveSnapshot().Key != "INIT-1" {
		t.Fatalf("expected active snapshot for INIT-1")
	}
}

func TestInterview_FrontierQuestionsViewRendering(t *testing.T) {
	m, _, store := setupTestModel(t)
	snap := &session.Snapshot{
		Key:    "STRAT-1",
		Status: session.StatusInProgress,
		Ticket: jira.Ticket{
			Key:     "STRAT-1",
			Summary: "Strategic Architecture Redesign",
		},
		CurrentFrontier: []session.Question{
			{
				ID:             "Q1",
				Title:          "Authentication Strategy",
				Explanation:    "Need to decide on auth flow",
				Options:        []string{"Basic Auth", "OAuth2 with PKCE", "SAML SSO"},
				Recommendation: "OAuth2 with PKCE",
			},
			{
				ID:             "Q2",
				Title:          "Database Engine",
				Explanation:    "Need primary datastore",
				Options:        []string{"PostgreSQL", "DynamoDB"},
				Recommendation: "PostgreSQL",
			},
		},
	}
	_ = store.Save(snap)

	m.activeSnapshot = snap
	m.screen = ScreenInterview
	m.loading = false
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	if m.CurrentQuestionIndex() != 0 {
		t.Fatalf("expected initial question index 0, got %d", m.CurrentQuestionIndex())
	}

	view := m.View()
	if !strings.Contains(view, "Authentication Strategy") {
		t.Fatalf("expected question title 'Authentication Strategy' in view, got: %s", view)
	}
	if !strings.Contains(view, "OAuth2 with PKCE") {
		t.Fatalf("expected recommendation 'OAuth2 with PKCE' in view, got: %s", view)
	}
	if !strings.Contains(view, "Database Engine") {
		t.Fatalf("expected question 'Database Engine' in view, got: %s", view)
	}
	if !strings.Contains(view, "Round 1") {
		t.Fatalf("expected 'Round 1' in view, got: %s", view)
	}
}

func TestInterview_NavigationAndAcceptRecommendation(t *testing.T) {
	m, _, store := setupTestModel(t)
	snap := &session.Snapshot{
		Key:    "STRAT-1",
		Status: session.StatusInProgress,
		CurrentFrontier: []session.Question{
			{
				ID:             "Q1",
				Title:          "Auth Strategy",
				Options:        []string{"Basic", "OAuth2"},
				Recommendation: "OAuth2",
			},
			{
				ID:             "Q2",
				Title:          "Database Engine",
				Options:        []string{"PostgreSQL", "DynamoDB"},
				Recommendation: "PostgreSQL",
			},
		},
	}
	_ = store.Save(snap)
	m.activeSnapshot = snap
	m.screen = ScreenInterview
	m.loading = false

	// Navigate down with 'j'
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)
	if m.CurrentQuestionIndex() != 1 {
		t.Fatalf("expected index 1 after 'j', got %d", m.CurrentQuestionIndex())
	}

	// Navigate up with 'k'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(Model)
	if m.CurrentQuestionIndex() != 0 {
		t.Fatalf("expected index 0 after 'k', got %d", m.CurrentQuestionIndex())
	}

	// Press 'enter' or 'y' to accept recommendation on Q1
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(Model)

	if m.ActiveSnapshot().CurrentFrontier[0].Answer != "OAuth2" {
		t.Fatalf("expected Answer to be 'OAuth2', got %q", m.ActiveSnapshot().CurrentFrontier[0].Answer)
	}
	// Verify auto-advance to next question index
	if m.CurrentQuestionIndex() != 1 {
		t.Fatalf("expected index auto-advanced to 1, got %d", m.CurrentQuestionIndex())
	}
}

func TestInterview_OptionNumberSelection(t *testing.T) {
	m, _, store := setupTestModel(t)
	snap := &session.Snapshot{
		Key:    "STRAT-1",
		Status: session.StatusInProgress,
		CurrentFrontier: []session.Question{
			{
				ID:             "Q1",
				Title:          "Auth Strategy",
				Options:        []string{"Basic", "OAuth2", "SAML"},
				Recommendation: "OAuth2",
			},
		},
	}
	_ = store.Save(snap)
	m.activeSnapshot = snap
	m.screen = ScreenInterview
	m.loading = false

	// Press '1' to select option 1 "Basic"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = updated.(Model)

	if m.ActiveSnapshot().CurrentFrontier[0].Answer != "Basic" {
		t.Fatalf("expected answer 'Basic', got %q", m.ActiveSnapshot().CurrentFrontier[0].Answer)
	}
}

func TestInterview_CustomAnswerEditing(t *testing.T) {
	m, _, store := setupTestModel(t)
	snap := &session.Snapshot{
		Key:    "STRAT-1",
		Status: session.StatusInProgress,
		CurrentFrontier: []session.Question{
			{
				ID:             "Q1",
				Title:          "Custom Field",
				Recommendation: "DefaultVal",
			},
		},
	}
	_ = store.Save(snap)
	m.activeSnapshot = snap
	m.screen = ScreenInterview
	m.loading = false

	// Press 'e' to start editing
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(Model)

	if !m.InterviewEditing() {
		t.Fatalf("expected InterviewEditing to be true")
	}

	// Type custom answer
	for _, r := range "My Custom Architecture" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}

	// Press Enter to submit answer
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.InterviewEditing() {
		t.Fatalf("expected InterviewEditing to be false after Enter")
	}
	if m.ActiveSnapshot().CurrentFrontier[0].Answer != "My Custom Architecture" {
		t.Fatalf("expected custom answer, got %q", m.ActiveSnapshot().CurrentFrontier[0].Answer)
	}
}

type mockAIEngine struct {
	responses []string
	callCount int
	err       error
}

func (m *mockAIEngine) Name() string { return "mock-ai" }
func (m *mockAIEngine) Review(ctx context.Context, req ai.ReviewRequest) (*ai.ReviewResult, error) {
	return nil, nil
}
func (m *mockAIEngine) Generate(ctx context.Context, req ai.PromptRequest) (*ai.GenerateResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	resp := "[]"
	if m.callCount < len(m.responses) {
		resp = m.responses[m.callCount]
	}
	m.callCount++
	return &ai.GenerateResult{
		Provider: "mock-ai",
		Content:  resp,
	}, nil
}

func TestInterview_SubmitRound_AdvancesAndRecomputesFrontier(t *testing.T) {
	cfg := &config.Config{
		Jira: config.JiraConfig{
			OriginProject: "STRAT",
		},
	}
	client := &mockJiraClient{}
	store := session.NewFileStore(t.TempDir())

	aiMock := &mockAIEngine{
		responses: []string{
			`[
				{
					"id": "Q2",
					"title": "Token Storage",
					"explanation": "Where to store tokens",
					"options": ["Redis", "Postgres"],
					"recommendation": "Redis"
				}
			]`,
		},
	}

	m := NewModel(cfg, client, store, WithAIEngine(aiMock))
	snap := &session.Snapshot{
		Key:    "STRAT-1",
		Status: session.StatusInProgress,
		CurrentFrontier: []session.Question{
			{
				ID:             "Q1",
				Title:          "Auth Strategy",
				Recommendation: "OAuth2",
				Answer:         "OAuth2",
			},
		},
	}
	_ = store.Save(snap)
	m.activeSnapshot = snap
	m.screen = ScreenInterview
	m.loading = false

	// Press Ctrl+S to submit round
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)

	if !m.Loading() {
		t.Fatalf("expected loading to be true while generating next frontier")
	}
	if cmd == nil {
		t.Fatalf("expected non-nil cmd for frontier generation")
	}

	// Verify loading spinner view
	view := m.View()
	if !strings.Contains(view, "generating next frontier") && !strings.Contains(view, "Generating") && !strings.Contains(view, "Analyzing") {
		t.Fatalf("expected loading message in view, got: %s", view)
	}

	// Execute command
	msg := cmd()
	if batchMsg, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batchMsg {
			if c != nil {
				mMsg := c()
				updated, _ = m.Update(mMsg)
				m = updated.(Model)
			}
		}
	} else {
		updated, _ = m.Update(msg)
		m = updated.(Model)
	}

	if m.Loading() {
		t.Fatalf("expected loading to be false after completion")
	}
	if len(m.ActiveSnapshot().Rounds) != 1 {
		t.Fatalf("expected 1 completed round in snapshot, got %d", len(m.ActiveSnapshot().Rounds))
	}
	if len(m.ActiveSnapshot().CurrentFrontier) != 1 {
		t.Fatalf("expected 1 new question in CurrentFrontier, got %d", len(m.ActiveSnapshot().CurrentFrontier))
	}
	if m.ActiveSnapshot().CurrentFrontier[0].ID != "Q2" {
		t.Fatalf("expected question Q2, got %s", m.ActiveSnapshot().CurrentFrontier[0].ID)
	}
	if m.CurrentQuestionIndex() != 0 {
		t.Fatalf("expected question index reset to 0, got %d", m.CurrentQuestionIndex())
	}
}

func TestInterview_FinalizeRefinementShortcut(t *testing.T) {
	cfg := &config.Config{
		Jira: config.JiraConfig{
			OriginProject: "STRAT",
		},
	}
	client := &mockJiraClient{}
	store := session.NewFileStore(t.TempDir())

	aiMock := &mockAIEngine{
		responses: []string{
			`{
				"epics": [
					{
						"id": "EPIC-1",
						"key": "EPIC-1",
						"title": "Core Auth Epic",
						"description": "Authentication decomposition",
						"tasks": [
							{
								"id": "TASK-1",
								"title": "Implement OAuth2 handler",
								"description": "Handler task"
							}
						]
					}
				]
			}`,
		},
	}

	m := NewModel(cfg, client, store, WithAIEngine(aiMock))
	snap := &session.Snapshot{
		Key:    "STRAT-1",
		Status: session.StatusInProgress,
		CurrentFrontier: []session.Question{
			{
				ID:             "Q1",
				Title:          "Auth Strategy",
				Recommendation: "OAuth2",
				Answer:         "OAuth2",
			},
		},
	}
	_ = store.Save(snap)
	m.activeSnapshot = snap
	m.screen = ScreenInterview
	m.loading = false

	// Press Ctrl+F to finalize refinement anytime
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	m = updated.(Model)

	if !m.Loading() {
		t.Fatalf("expected loading while synthesizing tree")
	}
	if cmd == nil {
		t.Fatalf("expected non-nil cmd for finalize")
	}

	// Execute command
	msg := cmd()
	if batchMsg, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batchMsg {
			if c != nil {
				mMsg := c()
				updated, _ = m.Update(mMsg)
				m = updated.(Model)
			}
		}
	} else {
		updated, _ = m.Update(msg)
		m = updated.(Model)
	}

	if m.Loading() {
		t.Fatalf("expected loading to be false after finalization")
	}
	if m.ActiveSnapshot().Status != session.StatusFinalized {
		t.Fatalf("expected status finalized, got %s", m.ActiveSnapshot().Status)
	}
	if m.ActiveSnapshot().Tree == nil || len(m.ActiveSnapshot().Tree.Epics) != 1 {
		t.Fatalf("expected 1 epic in tree")
	}
	if m.ActiveSnapshot().Tree.Epics[0].Title != "Core Auth Epic" {
		t.Fatalf("expected title 'Core Auth Epic', got %s", m.ActiveSnapshot().Tree.Epics[0].Title)
	}

	if m.Screen() != ScreenTree {
		t.Fatalf("expected ScreenTree, got %v", m.Screen())
	}
	view := m.View()
	if !strings.Contains(view, "Core Auth Epic") {
		t.Fatalf("expected epic title in view, got: %s", view)
	}
}

func TestInterview_ActionButtonsAndScrolling(t *testing.T) {
	m, _, store := setupTestModel(t)
	snap := &session.Snapshot{
		Key:    "STRAT-1",
		Status: session.StatusInProgress,
		CurrentFrontier: []session.Question{
			{
				ID:             "Q1",
				Title:          "Architecture Direction",
				Recommendation: "Modular monolith",
			},
		},
	}
	_ = store.Save(snap)
	m.activeSnapshot = snap
	m.screen = ScreenInterview
	m.loading = false

	// Verify action buttons rendered
	view := m.View()
	if !strings.Contains(view, "Finalize Refinement (Ctrl+F)") {
		t.Fatalf("expected finalize action button in view, got: %s", view)
	}
	if !strings.Contains(view, "Submit Round (Ctrl+S)") {
		t.Fatalf("expected submit round action button in view, got: %s", view)
	}

	// Verify scroll key updates viewport without error
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m = updated.(Model)
	if m.Screen() != ScreenInterview {
		t.Fatalf("expected to remain on ScreenInterview after pgdown")
	}
}

func TestTreeEditor_NavigationAndHierarchy(t *testing.T) {
	m, _, store := setupTestModel(t)
	snap := &session.Snapshot{
		Key:    "STRAT-1",
		Status: session.StatusFinalized,
		Tree: &session.DecompositionTree{
			Epics: []session.DecompositionEpic{
				{
					ID:              "EPIC-1",
					Title:           "Authentication Epic",
					Description:     "Epic description",
					DeliveryProject: "AUTH",
					Tasks: []session.DecompositionTask{
						{
							ID:              "TASK-1",
							Title:           "Login handler",
							Description:     "Handler desc",
							DeliveryProject: "AUTH",
						},
						{
							ID:              "TASK-2",
							Title:           "Token validation",
							Description:     "Validator desc",
							DeliveryProject: "CORE",
						},
					},
				},
			},
		},
	}
	_ = store.Save(snap)
	m.activeSnapshot = snap
	m.screen = ScreenTree
	m.loading = false
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	if m.TreeCursor() != 0 {
		t.Fatalf("expected initial tree cursor 0, got %d", m.TreeCursor())
	}

	// Move down to TASK-1
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)
	if m.TreeCursor() != 1 {
		t.Fatalf("expected tree cursor 1, got %d", m.TreeCursor())
	}

	// Move down to TASK-2
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if m.TreeCursor() != 2 {
		t.Fatalf("expected tree cursor 2, got %d", m.TreeCursor())
	}

	// Move up back to TASK-1
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if m.TreeCursor() != 1 {
		t.Fatalf("expected tree cursor 1, got %d", m.TreeCursor())
	}

	// Verify View contains hierarchy and checkboxes
	view := m.View()
	if !strings.Contains(view, "Authentication Epic") {
		t.Fatalf("expected epic title in view, got: %s", view)
	}
	if !strings.Contains(view, "Login handler") {
		t.Fatalf("expected task 1 title in view, got: %s", view)
	}
	if !strings.Contains(view, "Token validation") {
		t.Fatalf("expected task 2 title in view, got: %s", view)
	}
	if !strings.Contains(view, "[x]") {
		t.Fatalf("expected [x] inclusion indicator in view, got: %s", view)
	}
}

func TestTreeEditor_ToggleInclusion(t *testing.T) {
	m, _, store := setupTestModel(t)
	snap := &session.Snapshot{
		Key:    "STRAT-1",
		Status: session.StatusFinalized,
		Tree: &session.DecompositionTree{
			Epics: []session.DecompositionEpic{
				{
					ID:              "EPIC-1",
					Title:           "Authentication Epic",
					DeliveryProject: "AUTH",
					Tasks: []session.DecompositionTask{
						{
							ID:              "TASK-1",
							Title:           "Login handler",
							DeliveryProject: "AUTH",
						},
						{
							ID:              "TASK-2",
							Title:           "Token validation",
							DeliveryProject: "CORE",
						},
					},
				},
			},
		},
	}
	_ = store.Save(snap)
	m.activeSnapshot = snap
	m.screen = ScreenTree
	m.loading = false
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	// Move cursor to TASK-1 (index 1)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)
	if m.TreeCursor() != 1 {
		t.Fatalf("expected tree cursor 1, got %d", m.TreeCursor())
	}

	// Press space to toggle TASK-1 off
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)

	if !m.ActiveSnapshot().Tree.Epics[0].Tasks[0].Excluded {
		t.Fatalf("expected TASK-1 to be excluded")
	}
	if m.ActiveSnapshot().Tree.Epics[0].Tasks[1].Excluded {
		t.Fatalf("expected TASK-2 to remain included")
	}
	items := m.TreeItems()
	if !items[1].Excluded {
		t.Fatalf("expected tree item 1 to be marked excluded")
	}

	// Press 'x' on TASK-1 to toggle it back on
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = updated.(Model)
	if m.ActiveSnapshot().Tree.Epics[0].Tasks[0].Excluded {
		t.Fatalf("expected TASK-1 to be included again")
	}

	// Move to EPIC-1 (index 0)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(Model)

	// Press space on EPIC-1 to toggle epic off
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)
	if !m.ActiveSnapshot().Tree.Epics[0].Excluded {
		t.Fatalf("expected EPIC-1 to be excluded")
	}
	// Verify children also report excluded when parent epic is excluded
	items = m.TreeItems()
	if !items[0].Excluded {
		t.Fatalf("expected epic item to be excluded")
	}
	if !items[1].Excluded || !items[2].Excluded {
		t.Fatalf("expected child tasks to be excluded when parent epic is excluded")
	}

	// Check disk persistence
	loaded, err := store.Load("STRAT-1")
	if err != nil {
		t.Fatalf("failed to load snapshot from store: %v", err)
	}
	if !loaded.Tree.Epics[0].Excluded {
		t.Fatalf("expected saved snapshot to have EPIC-1 excluded")
	}
}

func TestTreeEditor_InlineEditing(t *testing.T) {
	m, _, store := setupTestModel(t)
	snap := &session.Snapshot{
		Key:    "STRAT-1",
		Status: session.StatusFinalized,
		Tree: &session.DecompositionTree{
			Epics: []session.DecompositionEpic{
				{
					ID:              "EPIC-1",
					Title:           "Old Epic Title",
					Description:     "Old Epic Desc",
					DeliveryProject: "OLD-PROJ",
					Tasks: []session.DecompositionTask{
						{
							ID:              "TASK-1",
							Title:           "Old Task Title",
							Description:     "Old Task Desc",
							DeliveryProject: "OLD-TASK-PROJ",
						},
					},
				},
			},
		},
	}
	_ = store.Save(snap)
	m.activeSnapshot = snap
	m.screen = ScreenTree
	m.loading = false
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	// 1. Edit Epic Title
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(Model)
	if !m.TreeEditing() || m.TreeEditField() != "title" {
		t.Fatalf("expected treeEditing=true with field=title")
	}

	// Clear and set value
	m.treeInput.SetValue("New Epic Title")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.TreeEditing() {
		t.Fatalf("expected treeEditing=false after Enter")
	}
	if m.ActiveSnapshot().Tree.Epics[0].Title != "New Epic Title" {
		t.Fatalf("expected epic title updated, got %s", m.ActiveSnapshot().Tree.Epics[0].Title)
	}

	// 2. Edit Epic Description
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(Model)
	if !m.TreeEditing() || m.TreeEditField() != "description" {
		t.Fatalf("expected treeEditing=true with field=description")
	}
	m.treeInput.SetValue("New Epic Desc")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.ActiveSnapshot().Tree.Epics[0].Description != "New Epic Desc" {
		t.Fatalf("expected epic description updated, got %s", m.ActiveSnapshot().Tree.Epics[0].Description)
	}

	// 3. Edit Epic Delivery Project
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = updated.(Model)
	if !m.TreeEditing() || m.TreeEditField() != "project" {
		t.Fatalf("expected treeEditing=true with field=project")
	}
	m.treeInput.SetValue("NEW-EPIC-PROJ")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.ActiveSnapshot().Tree.Epics[0].DeliveryProject != "NEW-EPIC-PROJ" {
		t.Fatalf("expected epic project updated, got %s", m.ActiveSnapshot().Tree.Epics[0].DeliveryProject)
	}

	// 4. Navigate down to TASK-1
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)
	if m.TreeCursor() != 1 {
		t.Fatalf("expected cursor on task 1")
	}

	// Edit task title and cancel with Esc
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(Model)
	m.treeInput.SetValue("Cancelled Title")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.TreeEditing() {
		t.Fatalf("expected treeEditing=false after Esc")
	}
	if m.ActiveSnapshot().Tree.Epics[0].Tasks[0].Title != "Old Task Title" {
		t.Fatalf("expected old task title preserved after Esc, got %s", m.ActiveSnapshot().Tree.Epics[0].Tasks[0].Title)
	}

	// Edit task Delivery Project
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = updated.(Model)
	m.treeInput.SetValue("NEW-TASK-PROJ")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.ActiveSnapshot().Tree.Epics[0].Tasks[0].DeliveryProject != "NEW-TASK-PROJ" {
		t.Fatalf("expected task project updated, got %s", m.ActiveSnapshot().Tree.Epics[0].Tasks[0].DeliveryProject)
	}

	// Check persisted snapshot
	loaded, err := store.Load("STRAT-1")
	if err != nil {
		t.Fatalf("failed to load snapshot: %v", err)
	}
	if loaded.Tree.Epics[0].Title != "New Epic Title" {
		t.Fatalf("expected saved snapshot to have updated title")
	}
	if loaded.Tree.Epics[0].Tasks[0].DeliveryProject != "NEW-TASK-PROJ" {
		t.Fatalf("expected saved snapshot to have updated task project")
	}
}

func TestTreeEditor_ActionButtons_ReturnToInterview_And_ProceedSync(t *testing.T) {
	m, _, store := setupTestModel(t)
	snap := &session.Snapshot{
		Key:    "STRAT-1",
		Status: session.StatusFinalized,
		Tree: &session.DecompositionTree{
			Epics: []session.DecompositionEpic{
				{
					ID:              "EPIC-1",
					Title:           "Auth Epic",
					DeliveryProject: "", // unassigned
					Tasks: []session.DecompositionTask{
						{
							ID:              "TASK-1",
							Title:           "Auth Task",
							DeliveryProject: "",
						},
					},
				},
			},
		},
	}
	_ = store.Save(snap)
	m.activeSnapshot = snap
	m.screen = ScreenTree
	m.loading = false
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	// 1. Action: Return to Interview via 'b'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(Model)
	if m.Screen() != ScreenInterview {
		t.Fatalf("expected ScreenInterview after 'b', got %v", m.Screen())
	}

	// 2. Return to Tree via 'v'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = updated.(Model)
	if m.Screen() != ScreenTree {
		t.Fatalf("expected ScreenTree after 'v', got %v", m.Screen())
	}

	// 3. Action: Attempt to Proceed to Sync with unassigned project -> should fail validation
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(Model)
	if m.SyncProceedRequested() {
		t.Fatalf("expected sync not to proceed when items are unassigned")
	}
	statusMsg, isErr := m.StatusMsg()
	if !isErr || !strings.Contains(statusMsg, "delivery project") {
		t.Fatalf("expected delivery project error message, got %q (isErr=%v)", statusMsg, isErr)
	}

	// 4. Assign projects to Epic and Task
	m.activeSnapshot.Tree.Epics[0].DeliveryProject = "AUTH"
	m.activeSnapshot.Tree.Epics[0].Tasks[0].DeliveryProject = "AUTH"

	// 5. Action: Proceed to Sync with valid tree
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(Model)
	if !m.SyncProceedRequested() {
		t.Fatalf("expected SyncProceedRequested to be true after 's'")
	}
	statusMsg, isErr = m.StatusMsg()
	if isErr || !strings.Contains(statusMsg, "Ready to synchronize") {
		t.Fatalf("expected ready to synchronize status, got %q (isErr=%v)", statusMsg, isErr)
	}
}

func setupTestModelForSync(t *testing.T) (Model, *mockJiraClient, session.Store) {
	cfg := &config.Config{
		Jira: config.JiraConfig{
			URL:           "https://jira.example.com",
			OriginProject: "STRAT",
		},
	}
	client := &mockJiraClient{
		ticketMap: map[string]*jira.Ticket{
			"STRAT-1": {Key: "STRAT-1", Summary: "Strategic Ticket 1"},
		},
	}
	store := session.NewFileStore(t.TempDir())
	m := NewModel(cfg, client, store)

	m.activeSnapshot = &session.Snapshot{
		Key:    "STRAT-1",
		Status: session.StatusFinalized,
		Tree: &session.DecompositionTree{
			Epics: []session.DecompositionEpic{
				{
					ID:              "EPIC-1",
					Title:           "Auth Service",
					DeliveryProject: "AUTH",
					Tasks: []session.DecompositionTask{
						{
							ID:              "TASK-1",
							Title:           "User Model",
							DeliveryProject: "AUTH",
						},
						{
							ID:              "TASK-2",
							Title:           "Login Endpoint",
							DeliveryProject: "AUTH",
							DependsOn:       []string{"TASK-1"},
						},
					},
				},
			},
		},
	}
	m.screen = ScreenTree
	return m, client, store
}

func TestSyncScreen_TriggerAndInitialSteps(t *testing.T) {
	m, _, _ := setupTestModelForSync(t)

	// Press 's' to start sync
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(Model)

	if m.Screen() != ScreenSync {
		t.Fatalf("expected ScreenSync, got %v", m.Screen())
	}
	if m.SyncState() != SyncStateRunning {
		t.Fatalf("expected SyncStateRunning, got %v", m.SyncState())
	}
	if len(m.SyncSteps()) != 4 { // 1 epic + 2 tasks + 1 dependency link
		t.Fatalf("expected 4 steps, got %d", len(m.SyncSteps()))
	}
	if m.SyncCurrentStep() != 0 {
		t.Fatalf("expected current step 0, got %d", m.SyncCurrentStep())
	}
	if cmd == nil {
		t.Fatalf("expected non-nil command returned to execute first step")
	}

	view := m.View()
	if !strings.Contains(view, "Jira Synchronization") {
		t.Errorf("expected view to contain 'Jira Synchronization', got:\n%s", view)
	}
	if !strings.Contains(view, "Auth Service") {
		t.Errorf("expected view to contain step title 'Auth Service', got:\n%s", view)
	}
}

func TestSyncScreen_StepProgressAndSuccessSummary(t *testing.T) {
	m, _, store := setupTestModelForSync(t)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(Model)

	// Step 0 completion (Epic)
	step0 := m.SyncSteps()[0]
	step0.Status = refine.StepCompleted
	step0.Key = "AUTH-100"
	step0.URL = "https://jira.example.com/browse/AUTH-100"
	m.activeSnapshot.Tree.Epics[0].Key = "AUTH-100"

	updated, cmd := m.Update(syncStepResultMsg{
		stepIndex: 0,
		step:      step0,
	})
	m = updated.(Model)

	if m.SyncCurrentStep() != 1 {
		t.Fatalf("expected current step 1, got %d", m.SyncCurrentStep())
	}
	if cmd == nil {
		t.Fatalf("expected command for next step")
	}

	// Step 1 completion (Task 1)
	step1 := m.SyncSteps()[1]
	step1.Status = refine.StepCompleted
	step1.Key = "AUTH-101"
	step1.URL = "https://jira.example.com/browse/AUTH-101"
	m.activeSnapshot.Tree.Epics[0].Tasks[0].Key = "AUTH-101"

	updated, _ = m.Update(syncStepResultMsg{
		stepIndex: 1,
		step:      step1,
	})
	m = updated.(Model)

	// Step 2 completion (Task 2)
	step2 := m.SyncSteps()[2]
	step2.Status = refine.StepCompleted
	step2.Key = "AUTH-102"
	step2.URL = "https://jira.example.com/browse/AUTH-102"
	m.activeSnapshot.Tree.Epics[0].Tasks[1].Key = "AUTH-102"

	updated, _ = m.Update(syncStepResultMsg{
		stepIndex: 2,
		step:      step2,
	})
	m = updated.(Model)

	// Step 3 completion (Dependency link)
	step3 := m.SyncSteps()[3]
	step3.Status = refine.StepCompleted

	updated, _ = m.Update(syncStepResultMsg{
		stepIndex: 3,
		step:      step3,
	})
	m = updated.(Model)

	if m.SyncState() != SyncStateSuccess {
		t.Fatalf("expected SyncStateSuccess, got %v", m.SyncState())
	}
	if m.ActiveSnapshot().Status != session.StatusSynced {
		t.Fatalf("expected snapshot status %q, got %q", session.StatusSynced, m.ActiveSnapshot().Status)
	}

	// Verify persistence
	saved, err := store.Load("STRAT-1")
	if err != nil {
		t.Fatalf("failed loading saved snapshot: %v", err)
	}
	if saved.Status != session.StatusSynced {
		t.Fatalf("expected persisted snapshot status %q, got %q", session.StatusSynced, saved.Status)
	}

	// Check final success view
	view := m.View()
	if !strings.Contains(view, "Synchronization Complete") {
		t.Errorf("expected view to contain 'Synchronization Complete', got:\n%s", view)
	}
	if !strings.Contains(view, "AUTH-100") || !strings.Contains(view, "https://jira.example.com/browse/AUTH-100") {
		t.Errorf("expected view to show created key AUTH-100 and URL, got:\n%s", view)
	}
	if !strings.Contains(view, "AUTH-101") || !strings.Contains(view, "https://jira.example.com/browse/AUTH-101") {
		t.Errorf("expected view to show created key AUTH-101 and URL, got:\n%s", view)
	}
}

func TestSyncScreen_ErrorAndRetry(t *testing.T) {
	m, _, _ := setupTestModelForSync(t)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(Model)

	// Step 0 fails
	failErr := errors.New("403 Forbidden: insufficient Jira permissions")
	updated, _ = m.Update(syncStepResultMsg{
		stepIndex: 0,
		err:       failErr,
	})
	m = updated.(Model)

	if m.SyncState() != SyncStateFailed {
		t.Fatalf("expected SyncStateFailed, got %v", m.SyncState())
	}
	if m.SyncError() == nil || !strings.Contains(m.SyncError().Error(), "403 Forbidden") {
		t.Fatalf("expected 403 Forbidden error recorded, got %v", m.SyncError())
	}

	view := m.View()
	if !strings.Contains(view, "403 Forbidden") {
		t.Errorf("expected view to display error message, got:\n%s", view)
	}
	if !strings.Contains(view, "Retry") {
		t.Errorf("expected view to show retry option, got:\n%s", view)
	}

	// Press 'r' to retry
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = updated.(Model)

	if m.SyncState() != SyncStateRunning {
		t.Fatalf("expected SyncStateRunning after retry, got %v", m.SyncState())
	}
	if m.SyncError() != nil {
		t.Fatalf("expected sync error to be cleared on retry, got %v", m.SyncError())
	}
	if cmd == nil {
		t.Fatalf("expected non-nil command returned for retry")
	}
}

func TestSyncScreen_Navigation(t *testing.T) {
	m, _, _ := setupTestModelForSync(t)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(Model)

	// In failed state, pressing 'esc' or 'b' returns to ScreenTree
	updated, _ = m.Update(syncStepResultMsg{
		stepIndex: 0,
		err:       errors.New("network timeout"),
	})
	m = updated.(Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(Model)
	if m.Screen() != ScreenTree {
		t.Fatalf("expected ScreenTree after pressing 'b' in failed state, got %v", m.Screen())
	}

	// Back to sync, finish, then 'p' returns to ScreenPicker
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(Model)

	// Complete all steps
	for i := 0; i < len(m.SyncSteps()); i++ {
		st := m.SyncSteps()[i]
		st.Status = refine.StepCompleted
		st.Key = fmt.Sprintf("AUTH-%d", 200+i)
		st.URL = fmt.Sprintf("https://jira.example.com/browse/AUTH-%d", 200+i)
		updated, _ = m.Update(syncStepResultMsg{stepIndex: i, step: st})
		m = updated.(Model)
	}

	if m.SyncState() != SyncStateSuccess {
		t.Fatalf("expected SyncStateSuccess, got %v", m.SyncState())
	}

	// Press 'p' to return to picker
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = updated.(Model)
	if m.Screen() != ScreenPicker {
		t.Fatalf("expected ScreenPicker after pressing 'p' in success state, got %v", m.Screen())
	}
}

// lineContainingBoth returns the first rendered line containing both
// substrings, or "" if no single line contains both. Bordered boxes rendered
// with plain string concatenation instead of lipgloss.JoinHorizontal end up
// staggered across separate lines instead of side by side, so this is what
// distinguishes a correctly joined button row from a broken one.
func lineContainingBoth(view, a, b string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, a) && strings.Contains(line, b) {
			return line
		}
	}
	return ""
}

func TestTreeView_ActionButtons_RenderSideBySide(t *testing.T) {
	m, _, _ := setupTestModelForSync(t)
	m.loading = false
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	view := m.View()
	if lineContainingBoth(view, "Return to Interview", "Proceed to Jira Sync") == "" {
		t.Fatalf("expected 'Return to Interview' and 'Proceed to Jira Sync' buttons on the same rendered line, got:\n%s", view)
	}
}

func TestSyncView_SuccessActionButtons_RenderSideBySide(t *testing.T) {
	m, _, _ := setupTestModelForSync(t)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(Model)

	for i := 0; i < len(m.SyncSteps()); i++ {
		st := m.SyncSteps()[i]
		st.Status = refine.StepCompleted
		st.Key = fmt.Sprintf("AUTH-%d", 300+i)
		updated, _ = m.Update(syncStepResultMsg{stepIndex: i, step: st})
		m = updated.(Model)
	}
	if m.SyncState() != SyncStateSuccess {
		t.Fatalf("expected SyncStateSuccess, got %v", m.SyncState())
	}

	view := m.View()
	if lineContainingBoth(view, "Return to Ticket Picker", "Return to Tree Review") == "" {
		t.Fatalf("expected 'Return to Ticket Picker' and 'Return to Tree Review' buttons on the same rendered line, got:\n%s", view)
	}
}

// firstLineWith returns the first rendered line containing needle, or "" if
// no line contains it.
func firstLineWith(view, needle string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

// contentColumn returns how many spaces of padding sit between a box's left
// border ('│') and the first non-space character of that row's content.
// Every row inside the same box shares the identical border+padding prefix,
// so this isolates a row's own leading indentation from the box chrome that
// leadingSpaces(line) (counting from index 0) would otherwise always see as
// zero, since every line starts with '│' regardless of content indentation.
func contentColumn(line string) int {
	idx := strings.IndexRune(line, '│')
	if idx == -1 {
		return 0
	}
	rest := line[idx+len("│"):]
	col := 0
	for _, r := range rest {
		if r != ' ' {
			break
		}
		col++
	}
	return col
}

func TestTreeView_FirstRow_NotIndentedPastHeader(t *testing.T) {
	// Regression test: the hierarchy header and "Item Details" header used
	// to embed their trailing "\n"/"\n\n" inside Render(), which lipgloss
	// pads to the header's width without a real trailing newline — gluing
	// the very next row onto the end of that padding and shifting it right
	// by the header's width. Same bug class as the interview screen's round
	// header (da327d4 / 4b721c7).
	m, _, _ := setupTestModelForSync(t)
	m.loading = false
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	view := m.View()

	headerLine := firstLineWith(view, "Decomposition Tree Hierarchy")
	if headerLine == "" {
		t.Fatalf("expected hierarchy header in view, got:\n%s", view)
	}
	epicLine := firstLineWith(view, "EPIC-1")
	if epicLine == "" {
		t.Fatalf("expected epic row in view, got:\n%s", view)
	}
	// Both rows sit inside the same bordered box, sharing an identical
	// border+padding prefix, so a correctly laid out epic row's own content
	// starts at the same column as the header's.
	if got, want := contentColumn(epicLine), contentColumn(headerLine); got != want {
		t.Fatalf("epic row starts at column %d, expected to match the header's column %d, view:\n%s", got, want, view)
	}

	detailsHeaderLine := firstLineWith(view, "Item Details")
	if detailsHeaderLine == "" {
		t.Fatalf("expected Item Details header in view, got:\n%s", view)
	}
	typeLine := firstLineWith(view, "Type:")
	if typeLine == "" {
		t.Fatalf("expected 'Type:' row in view, got:\n%s", view)
	}
	if got, want := contentColumn(typeLine), contentColumn(detailsHeaderLine); got != want {
		t.Fatalf("'Type:' row starts at column %d, expected to match the Item Details header's column %d, view:\n%s", got, want, view)
	}
}
