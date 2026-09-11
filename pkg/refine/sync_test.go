package refine

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/arxcruz/pr-review/pkg/jira"
	"github.com/arxcruz/pr-review/pkg/session"
)

type mockJiraSyncClient struct {
	createEpicCalls       []jira.CreateEpicRequest
	createTaskCalls       []jira.CreateTaskRequest
	linkStrategicCalls    []jira.StrategicLinkRequest
	createDependencyCalls [][2]string // [blockedKey, blockerKey]
	callOrder             []string

	epicKeySeq int
	taskKeySeq int
}

func newMockJiraSyncClient() *mockJiraSyncClient {
	return &mockJiraSyncClient{
		epicKeySeq: 10,
		taskKeySeq: 100,
	}
}

func (m *mockJiraSyncClient) GetTicket(ctx context.Context, key string) (*jira.Ticket, error) {
	return &jira.Ticket{Key: key}, nil
}

func (m *mockJiraSyncClient) GetRecentTickets(ctx context.Context, project string) ([]jira.RecentTicket, error) {
	return nil, nil
}

func (m *mockJiraSyncClient) CreateIssue(ctx context.Context, req jira.CreateIssueRequest) (*jira.CreatedIssue, error) {
	return nil, nil
}

func (m *mockJiraSyncClient) CreateEpic(ctx context.Context, req jira.CreateEpicRequest) (*jira.CreatedIssue, error) {
	m.createEpicCalls = append(m.createEpicCalls, req)
	m.epicKeySeq++
	key := req.Project + "-" + string(rune('0'+m.epicKeySeq))
	m.callOrder = append(m.callOrder, "CreateEpic:"+key)
	return &jira.CreatedIssue{
		ID:  key,
		Key: key,
		URL: "https://jira.example.com/browse/" + key,
	}, nil
}

func (m *mockJiraSyncClient) CreateTask(ctx context.Context, req jira.CreateTaskRequest) (*jira.CreatedIssue, error) {
	m.createTaskCalls = append(m.createTaskCalls, req)
	m.taskKeySeq++
	key := req.Project + "-" + string(rune('0'+m.taskKeySeq))
	m.callOrder = append(m.callOrder, "CreateTask:"+key)
	return &jira.CreatedIssue{
		ID:  key,
		Key: key,
		URL: "https://jira.example.com/browse/" + key,
	}, nil
}

func (m *mockJiraSyncClient) CreateIssueLink(ctx context.Context, req jira.CreateIssueLinkRequest) error {
	return nil
}

func (m *mockJiraSyncClient) CreateDependencyLink(ctx context.Context, blockedKey, blockerKey string) error {
	m.createDependencyCalls = append(m.createDependencyCalls, [2]string{blockedKey, blockerKey})
	m.callOrder = append(m.callOrder, "CreateDependencyLink:"+blockedKey+"->"+blockerKey)
	return nil
}

func (m *mockJiraSyncClient) SetParentLink(ctx context.Context, childKey, parentKey string) error {
	return nil
}

func (m *mockJiraSyncClient) LinkStrategicTicket(ctx context.Context, req jira.StrategicLinkRequest) (*jira.StrategicLinkResult, error) {
	m.linkStrategicCalls = append(m.linkStrategicCalls, req)
	m.callOrder = append(m.callOrder, "LinkStrategicTicket:"+req.ChildKey+"->"+req.OriginKey)
	return &jira.StrategicLinkResult{
		ChildKey:  req.ChildKey,
		OriginKey: req.OriginKey,
		Method:    "parent",
	}, nil
}

type memorySessionStore struct {
	saved *session.Snapshot
}

func (m *memorySessionStore) Save(snap *session.Snapshot) error {
	m.saved = snap
	return nil
}

func (m *memorySessionStore) Load(key string) (*session.Snapshot, error) {
	if m.saved != nil && m.saved.Key == key {
		return m.saved, nil
	}
	return nil, session.ErrNotFound
}

func (m *memorySessionStore) Exists(key string) (bool, error) {
	return m.saved != nil && m.saved.Key == key, nil
}

func (m *memorySessionStore) Delete(key string) error {
	if m.saved != nil && m.saved.Key == key {
		m.saved = nil
	}
	return nil
}

func (m *memorySessionStore) List() ([]string, error) {
	if m.saved != nil {
		return []string{m.saved.Key}, nil
	}
	return nil, nil
}

func TestSyncer_Verify(t *testing.T) {
	cfg := &config.JiraConfig{OriginProject: "STRAT"}
	syncer := NewSyncer(nil, nil, cfg)

	t.Run("nil snapshot", func(t *testing.T) {
		err := syncer.Verify(nil)
		if err == nil || !strings.Contains(err.Error(), "session snapshot is required") {
			t.Errorf("expected error for nil snapshot, got %v", err)
		}
	})

	t.Run("missing tree", func(t *testing.T) {
		snap := &session.Snapshot{Key: "STRAT-1"}
		err := syncer.Verify(snap)
		if err == nil || !strings.Contains(err.Error(), "no decomposition tree found") {
			t.Errorf("expected error for missing tree, got %v", err)
		}
	})

	t.Run("empty epics", func(t *testing.T) {
		snap := &session.Snapshot{
			Key:  "STRAT-1",
			Tree: &session.DecompositionTree{},
		}
		err := syncer.Verify(snap)
		if err == nil || !strings.Contains(err.Error(), "no decomposition tree found") {
			t.Errorf("expected error for empty epics, got %v", err)
		}
	})

	t.Run("unrouted epic", func(t *testing.T) {
		snap := &session.Snapshot{
			Key: "STRAT-1",
			Tree: &session.DecompositionTree{
				Epics: []session.DecompositionEpic{
					{
						ID:    "EPIC-1",
						Title: "Unrouted Epic",
					},
				},
			},
		}
		err := syncer.Verify(snap)
		if err == nil || !strings.Contains(err.Error(), "not routed to a delivery project") {
			t.Errorf("expected unrouted epic error, got %v", err)
		}
	})

	t.Run("unrouted task", func(t *testing.T) {
		snap := &session.Snapshot{
			Key: "STRAT-1",
			Tree: &session.DecompositionTree{
				Epics: []session.DecompositionEpic{
					{
						ID:              "EPIC-1",
						Title:           "Routed Epic",
						DeliveryProject: "DELIV",
						Tasks: []session.DecompositionTask{
							{
								ID:    "TASK-1",
								Title: "Unrouted Task",
							},
						},
					},
				},
			},
		}
		err := syncer.Verify(snap)
		if err == nil || !strings.Contains(err.Error(), "not routed to a delivery project") {
			t.Errorf("expected unrouted task error, got %v", err)
		}
	})

	t.Run("valid complete and routed tree", func(t *testing.T) {
		snap := &session.Snapshot{
			Key: "STRAT-1",
			Tree: &session.DecompositionTree{
				Epics: []session.DecompositionEpic{
					{
						ID:              "EPIC-1",
						Title:           "Routed Epic",
						DeliveryProject: "DELIV",
						Type:            "Epic",
						Tasks: []session.DecompositionTask{
							{
								ID:              "TASK-1",
								Title:           "Routed Task",
								DeliveryProject: "DELIV",
								Type:            "Task",
							},
						},
					},
				},
			},
		}
		err := syncer.Verify(snap)
		if err != nil {
			t.Fatalf("expected valid tree to pass verification, got: %v", err)
		}
	})
}

func TestSortTasksByDependency(t *testing.T) {
	epics := []session.DecompositionEpic{
		{
			ID:              "EPIC-1",
			Title:           "Epic One",
			DeliveryProject: "DELIV",
			Tasks: []session.DecompositionTask{
				{
					ID:              "TASK-3",
					Title:           "Task 3 (Depends on Task 2)",
					DeliveryProject: "DELIV",
					DependsOn:       []string{"TASK-2"},
				},
				{
					ID:              "TASK-1",
					Title:           "Task 1 (Base)",
					DeliveryProject: "DELIV",
				},
				{
					ID:              "TASK-2",
					Title:           "Task 2 (Depends on Task 1 and EPIC-1)",
					DeliveryProject: "DELIV",
					DependsOn:       []string{"TASK-1", "EPIC-1"},
				},
			},
		},
	}

	sorted, err := sortTasksByDependency(epics)
	if err != nil {
		t.Fatalf("unexpected sort error: %v", err)
	}

	if len(sorted) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(sorted))
	}

	expectedOrder := []string{"TASK-1", "TASK-2", "TASK-3"}
	for i, ref := range sorted {
		if ref.Task.ID != expectedOrder[i] {
			t.Errorf("at index %d: expected %s, got %s", i, expectedOrder[i], ref.Task.ID)
		}
	}
}

func TestSortTasksByDependency_Cycle(t *testing.T) {
	epics := []session.DecompositionEpic{
		{
			ID:              "EPIC-1",
			DeliveryProject: "DELIV",
			Tasks: []session.DecompositionTask{
				{
					ID:        "TASK-1",
					DependsOn: []string{"TASK-2"},
				},
				{
					ID:        "TASK-2",
					DependsOn: []string{"TASK-1"},
				},
			},
		},
	}

	_, err := sortTasksByDependency(epics)
	if err == nil || !strings.Contains(err.Error(), "dependency cycle") {
		t.Fatalf("expected cycle error, got: %v", err)
	}
}

func TestSyncer_Sync_UserConfirmationPrompt(t *testing.T) {
	cfg := &config.JiraConfig{OriginProject: "STRAT"}
	client := newMockJiraSyncClient()
	store := &memorySessionStore{}
	syncer := NewSyncer(client, store, cfg)

	makeSnap := func() *session.Snapshot {
		return &session.Snapshot{
			Key: "STRAT-42",
			Tree: &session.DecompositionTree{
				Epics: []session.DecompositionEpic{
					{
						ID:              "EPIC-1",
						Title:           "Architecture Epic",
						DeliveryProject: "DELIV",
						Type:            "Epic",
						Tasks: []session.DecompositionTask{
							{
								ID:              "TASK-1",
								Title:           "Setup Core",
								DeliveryProject: "DELIV",
								Type:            "Task",
							},
						},
					},
				},
			},
		}
	}

	t.Run("aborted when user enters n", func(t *testing.T) {
		snap := makeSnap()
		in := strings.NewReader("n\n")
		out := new(bytes.Buffer)

		res, err := syncer.Sync(context.Background(), snap, SyncOptions{
			In:  in,
			Out: out,
		})
		if err != nil {
			t.Fatalf("expected no error on cancel, got: %v", err)
		}
		if res == nil || !res.Aborted {
			t.Errorf("expected result to be marked Aborted")
		}
		if len(client.createEpicCalls) > 0 || len(client.createTaskCalls) > 0 {
			t.Errorf("no Jira calls should be made when aborted")
		}
		if !strings.Contains(out.String(), "Sync aborted") {
			t.Errorf("expected 'Sync aborted' in output, got: %s", out.String())
		}
	})

	t.Run("aborted on EOF", func(t *testing.T) {
		snap := makeSnap()
		in := strings.NewReader("")
		out := new(bytes.Buffer)

		res, err := syncer.Sync(context.Background(), snap, SyncOptions{
			In:  in,
			Out: out,
		})
		if err != nil {
			t.Fatalf("expected no error on EOF cancel, got: %v", err)
		}
		if res == nil || !res.Aborted {
			t.Errorf("expected result to be marked Aborted")
		}
		if len(client.createEpicCalls) > 0 {
			t.Errorf("no Jira calls should be made on EOF")
		}
	})

	t.Run("proceeds when user enters y", func(t *testing.T) {
		snap := makeSnap()
		in := strings.NewReader("y\n")
		out := new(bytes.Buffer)

		res, err := syncer.Sync(context.Background(), snap, SyncOptions{
			In:  in,
			Out: out,
		})
		if err != nil {
			t.Fatalf("expected success on confirm, got: %v", err)
		}
		if res == nil || res.Aborted {
			t.Errorf("expected result not to be aborted")
		}
		if len(client.createEpicCalls) != 1 {
			t.Errorf("expected 1 create epic call, got %d", len(client.createEpicCalls))
		}
	})
}

func TestSyncer_Sync_FullExecution(t *testing.T) {
	cfg := &config.JiraConfig{
		OriginProject: "STRAT",
		LinkType:      "Relates",
	}
	client := newMockJiraSyncClient()
	store := &memorySessionStore{}
	syncer := NewSyncer(client, store, cfg)

	snap := &session.Snapshot{
		Key:    "STRAT-100",
		Status: session.StatusFinalized,
		Tree: &session.DecompositionTree{
			Epics: []session.DecompositionEpic{
				{
					ID:              "EPIC-1",
					Title:           "Security Subsystem",
					Description:     "Authentication and authorization",
					DeliveryProject: "AUTH",
					Type:            "Epic",
					Tasks: []session.DecompositionTask{
						{
							ID:                 "TASK-2",
							Title:              "Token Middleware",
							DeliveryProject:    "AUTH",
							Type:               "Story",
							AcceptanceCriteria: []string{"Validates JWT", "Handles expiry"},
							DependsOn:          []string{"TASK-1"},
						},
						{
							ID:                 "TASK-1",
							Title:              "Key Management",
							DeliveryProject:    "AUTH",
							Type:               "Task",
							AcceptanceCriteria: []string{"Rotates keys"},
						},
					},
				},
				{
					ID:              "EPIC-2",
					Title:           "Audit Logger",
					DeliveryProject: "CORE",
					Type:            "Epic",
					Tasks: []session.DecompositionTask{
						{
							ID:              "TASK-3",
							Title:           "Audit Hook",
							DeliveryProject: "CORE",
							Type:            "Task",
							DependsOn:       []string{"TASK-2"},
						},
					},
				},
			},
		},
	}

	out := new(bytes.Buffer)
	res, err := syncer.Sync(context.Background(), snap, SyncOptions{
		Out:         out,
		AutoConfirm: true,
	})
	if err != nil {
		t.Fatalf("unexpected sync error: %v", err)
	}

	if res.Aborted {
		t.Fatalf("expected sync not to be aborted")
	}

	// 1. Verify Epics created first and linked to STRAT-100
	if len(client.createEpicCalls) != 2 {
		t.Fatalf("expected 2 epics created, got %d", len(client.createEpicCalls))
	}
	if len(client.linkStrategicCalls) != 2 {
		t.Fatalf("expected 2 strategic links created, got %d", len(client.linkStrategicCalls))
	}
	for _, link := range client.linkStrategicCalls {
		if link.OriginKey != "STRAT-100" {
			t.Errorf("expected OriginKey STRAT-100, got %s", link.OriginKey)
		}
	}

	// 2. Verify Tasks created in topological dependency order (TASK-1, then TASK-2, then TASK-3)
	if len(client.createTaskCalls) != 3 {
		t.Fatalf("expected 3 tasks created, got %d", len(client.createTaskCalls))
	}
	if client.createTaskCalls[0].Summary != "Key Management" {
		t.Errorf("expected TASK-1 (Key Management) first, got %s", client.createTaskCalls[0].Summary)
	}
	if client.createTaskCalls[1].Summary != "Token Middleware" {
		t.Errorf("expected TASK-2 (Token Middleware) second, got %s", client.createTaskCalls[1].Summary)
	}
	if client.createTaskCalls[2].Summary != "Audit Hook" {
		t.Errorf("expected TASK-3 (Audit Hook) third, got %s", client.createTaskCalls[2].Summary)
	}

	// 3. Verify parent links set on tasks
	epic1Key := snap.Tree.Epics[0].Key
	epic2Key := snap.Tree.Epics[1].Key
	if client.createTaskCalls[0].ParentKey != epic1Key {
		t.Errorf("expected task 1 parent %s, got %s", epic1Key, client.createTaskCalls[0].ParentKey)
	}
	if client.createTaskCalls[1].ParentKey != epic1Key {
		t.Errorf("expected task 2 parent %s, got %s", epic1Key, client.createTaskCalls[1].ParentKey)
	}
	if client.createTaskCalls[2].ParentKey != epic2Key {
		t.Errorf("expected task 3 parent %s, got %s", epic2Key, client.createTaskCalls[2].ParentKey)
	}

	// 4. Verify inter-task dependency links (TASK-2 depends on TASK-1, TASK-3 depends on TASK-2)
	if len(client.createDependencyCalls) != 2 {
		t.Fatalf("expected 2 dependency links, got %d", len(client.createDependencyCalls))
	}

	// 5. Verify snapshot keys recorded and saved
	if snap.Tree.Epics[0].Key == "" || snap.Tree.Epics[1].Key == "" {
		t.Errorf("expected epic keys recorded in snapshot")
	}
	if snap.Tree.Epics[0].Tasks[0].Key == "" || snap.Tree.Epics[0].Tasks[1].Key == "" || snap.Tree.Epics[1].Tasks[0].Key == "" {
		t.Errorf("expected task keys recorded in snapshot")
	}
	if snap.Status != session.StatusSynced {
		t.Errorf("expected snapshot status %s, got %s", session.StatusSynced, snap.Status)
	}
	if store.saved != snap {
		t.Errorf("expected snapshot to be saved to store")
	}

	// 6. Summary table output contains created issues
	summaryTable := FormatSyncSummaryTable(res)
	if !strings.Contains(summaryTable, "AUTH") || !strings.Contains(summaryTable, "CORE") {
		t.Errorf("expected projects in summary table, got:\n%s", summaryTable)
	}
	if !strings.Contains(summaryTable, "Security Subsystem") || !strings.Contains(summaryTable, "Token Middleware") {
		t.Errorf("expected titles in summary table, got:\n%s", summaryTable)
	}
}

func TestSyncer_Sync_PartialResume(t *testing.T) {
	cfg := &config.JiraConfig{OriginProject: "STRAT"}
	client := newMockJiraSyncClient()
	store := &memorySessionStore{}
	syncer := NewSyncer(client, store, cfg)

	snap := &session.Snapshot{
		Key: "STRAT-200",
		Tree: &session.DecompositionTree{
			Epics: []session.DecompositionEpic{
				{
					ID:              "EPIC-1",
					Key:             "AUTH-999", // already created!
					Title:           "Security Subsystem",
					DeliveryProject: "AUTH",
					Type:            "Epic",
					Tasks: []session.DecompositionTask{
						{
							ID:              "TASK-1",
							Key:             "AUTH-1001", // already created!
							Title:           "Key Management",
							DeliveryProject: "AUTH",
							Type:            "Task",
						},
						{
							ID:              "TASK-2",
							Title:           "Token Middleware",
							DeliveryProject: "AUTH",
							Type:            "Story",
							DependsOn:       []string{"TASK-1"},
						},
					},
				},
			},
		},
	}

	res, err := syncer.Sync(context.Background(), snap, SyncOptions{AutoConfirm: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// EPIC-1 should NOT be recreated
	if len(client.createEpicCalls) != 0 {
		t.Errorf("expected 0 create epic calls for already created epic, got %d", len(client.createEpicCalls))
	}
	// TASK-1 should NOT be recreated, only TASK-2
	if len(client.createTaskCalls) != 1 {
		t.Fatalf("expected 1 create task call, got %d", len(client.createTaskCalls))
	}
	if client.createTaskCalls[0].ParentKey != "AUTH-999" {
		t.Errorf("expected ParentKey AUTH-999, got %s", client.createTaskCalls[0].ParentKey)
	}
	// Dependency link between TASK-2 and existing AUTH-1001
	if len(client.createDependencyCalls) != 1 {
		t.Fatalf("expected 1 dependency link, got %d", len(client.createDependencyCalls))
	}
	if client.createDependencyCalls[0][1] != "AUTH-1001" {
		t.Errorf("expected blocker key AUTH-1001, got %s", client.createDependencyCalls[0][1])
	}
	if res.EpicsCreated[0].Status != "Existing" {
		t.Errorf("expected existing status for EPIC-1, got %s", res.EpicsCreated[0].Status)
	}
}
