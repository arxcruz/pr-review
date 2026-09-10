package refine_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/arxcruz/pr-review/pkg/ai"
	"github.com/arxcruz/pr-review/pkg/jira"
	"github.com/arxcruz/pr-review/pkg/refine"
	"github.com/arxcruz/pr-review/pkg/session"
)

type sequentialMockAI struct {
	responses []string
	callCount int
	prompts   []ai.PromptRequest
}

func (s *sequentialMockAI) Name() string {
	return "sequential-mock"
}

func (s *sequentialMockAI) Review(ctx context.Context, req ai.ReviewRequest) (*ai.ReviewResult, error) {
	return nil, nil
}

func (s *sequentialMockAI) Generate(ctx context.Context, req ai.PromptRequest) (*ai.GenerateResult, error) {
	s.prompts = append(s.prompts, req)
	resp := "[]"
	if s.callCount < len(s.responses) {
		resp = s.responses[s.callCount]
	}
	s.callCount++
	return &ai.GenerateResult{
		Provider: s.Name(),
		Model:    "mock-model",
		Content:  resp,
	}, nil
}

func TestSessionLoop_Run_SingleRoundAndFinalize(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)

	snap := &session.Snapshot{
		Key:    "STRAT-10",
		Status: "new",
		Ticket: jira.Ticket{
			Key:         "STRAT-10",
			Summary:     "Refactor Authentication Service",
			Description: "Modernize legacy tokens to OAuth2/OIDC",
		},
	}
	if err := store.Save(snap); err != nil {
		t.Fatalf("failed to save initial snapshot: %v", err)
	}

	round1JSON := `[
		{
			"id": "Q1",
			"title": "Storage Engine",
			"explanation": "Choose token persistence backend",
			"options": ["PostgreSQL", "Redis", "DynamoDB"],
			"recommendation": "PostgreSQL"
		},
		{
			"id": "Q2",
			"title": "Protocol",
			"explanation": "API transport",
			"options": ["REST", "gRPC"],
			"recommendation": "REST"
		},
		{
			"id": "Q3",
			"title": "Cache Layer",
			"explanation": "Session caching",
			"options": ["Redis", "Memcached"],
			"recommendation": "Redis"
		}
	]`
	emptyRoundJSON := `[]`
	treeJSON := `{
		"epics": [
			{
				"id": "EPIC-1",
				"title": "Authentication Modernization",
				"tasks": [
					{
						"id": "TASK-1",
						"title": "JWT Endpoint",
						"acceptance_criteria": ["Issues valid JWTs"]
					}
				]
			}
		]
	}`

	mockAI := &sequentialMockAI{
		responses: []string{round1JSON, emptyRoundJSON, treeJSON},
	}
	engine := refine.NewEngine(mockAI)

	// User input sequence:
	// Q1: Enter (accept recommendation: "PostgreSQL")
	// Q2: "2" (select 2nd option: "gRPC")
	// Q3: "Custom In-Memory" (custom answer)
	// Empty frontier prompt: Enter (finalize)
	input := strings.Join([]string{
		"",                  // Q1 accept default
		"2",                 // Q2 choose option 2
		"Custom In-Memory",  // Q3 custom answer
		"",                  // finalize prompt: Enter
	}, "\n") + "\n"

	inBuf := strings.NewReader(input)
	outBuf := new(bytes.Buffer)

	loop := refine.NewSessionLoop(refine.LoopConfig{
		Engine:   engine,
		Store:    store,
		Snapshot: snap,
		In:       inBuf,
		Out:      outBuf,
	})

	err := loop.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected loop run error: %v", err)
	}

	// Verify output displays question details and recommendations
	outStr := outBuf.String()
	if !strings.Contains(outStr, "Storage Engine") || !strings.Contains(outStr, "PostgreSQL") {
		t.Errorf("output missing Q1 details: %s", outStr)
	}
	if !strings.Contains(outStr, "Protocol") || !strings.Contains(outStr, "gRPC") {
		t.Errorf("output missing Q2 details: %s", outStr)
	}
	if !strings.Contains(outStr, "Cache Layer") {
		t.Errorf("output missing Q3 details: %s", outStr)
	}

	// Verify snapshot in memory
	if snap.Status != "finalized" {
		t.Errorf("expected snapshot status 'finalized', got '%s'", snap.Status)
	}
	if len(snap.Rounds) != 1 {
		t.Fatalf("expected 1 round in snapshot, got %d", len(snap.Rounds))
	}
	r1 := snap.Rounds[0]
	if len(r1.Questions) != 3 {
		t.Fatalf("expected 3 questions in round 1, got %d", len(r1.Questions))
	}
	if r1.Questions[0].Answer != "PostgreSQL" {
		t.Errorf("Q1 answer expected 'PostgreSQL', got '%s'", r1.Questions[0].Answer)
	}
	if r1.Questions[1].Answer != "gRPC" {
		t.Errorf("Q2 answer expected 'gRPC', got '%s'", r1.Questions[1].Answer)
	}
	if r1.Questions[2].Answer != "Custom In-Memory" {
		t.Errorf("Q3 answer expected 'Custom In-Memory', got '%s'", r1.Questions[2].Answer)
	}

	// Verify persistence in store
	loaded, err := store.Load(snap.Key)
	if err != nil {
		t.Fatalf("failed to load snapshot from store: %v", err)
	}
	if loaded.Status != "finalized" {
		t.Errorf("expected loaded snapshot status 'finalized', got '%s'", loaded.Status)
	}
	if len(loaded.Rounds) != 1 {
		t.Fatalf("expected 1 round in loaded snapshot, got %d", len(loaded.Rounds))
	}
	if loaded.Rounds[0].Questions[0].Answer != "PostgreSQL" {
		t.Errorf("loaded Q1 answer expected 'PostgreSQL', got '%s'", loaded.Rounds[0].Questions[0].Answer)
	}
	if snap.Tree == nil || len(snap.Tree.Epics) != 1 {
		t.Fatalf("expected snap.Tree to have 1 epic, got %+v", snap.Tree)
	}
	if loaded.Tree == nil || len(loaded.Tree.Epics) != 1 {
		t.Fatalf("expected loaded.Tree to have 1 epic, got %+v", loaded.Tree)
	}
}

func TestSessionLoop_Run_MultiRoundWithAddRequirement(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)

	snap := &session.Snapshot{
		Key:    "STRAT-20",
		Status: "new",
		Ticket: jira.Ticket{
			Key:     "STRAT-20",
			Summary: "Event Bus Architecture",
		},
	}

	round1JSON := `[
		{
			"id": "Q1",
			"title": "Broker Choice",
			"explanation": "Event streaming broker",
			"options": ["Kafka", "RabbitMQ"],
			"recommendation": "Kafka"
		}
	]`
	round2JSON := `[
		{
			"id": "Q2",
			"title": "Ordering Guarantees",
			"explanation": "Partition ordering",
			"options": ["Strict", "Eventual"],
			"recommendation": "Strict"
		}
	]`
	emptyRoundJSON := `[]`
	roundAfterReqJSON := `[
		{
			"id": "Q3",
			"title": "Zero-downtime Partition Rebalance",
			"explanation": "Strategy during rebalance",
			"options": ["CooperativeSticky", "Range"],
			"recommendation": "CooperativeSticky"
		}
	]`

	mockAI := &sequentialMockAI{
		responses: []string{
			round1JSON,
			round2JSON,
			emptyRoundJSON,
			roundAfterReqJSON,
			emptyRoundJSON,
			`{"epics":[{"id":"EPIC-1","title":"Event Bus","tasks":[{"id":"TASK-1","title":"Kafka Topic Config"}]}]}`,
		},
	}
	engine := refine.NewEngine(mockAI)

	// Flow:
	// Round 1 Q1: Enter (accept "Kafka")
	// Round 2 Q2: Enter (accept "Strict")
	// Empty frontier prompt: "a" (add requirement)
	// Add requirement prompt: "Zero-downtime rolling upgrades required"
	// Round 3 Q3: "1" (select "CooperativeSticky")
	// Empty frontier prompt: "f" (finalize)
	input := strings.Join([]string{
		"",                                          // Q1 accept
		"",                                          // Q2 accept
		"a",                                         // add requirement
		"Zero-downtime rolling upgrades required",   // requirement text
		"1",                                         // Q3 choose option 1
		"f",                                         // finalize
	}, "\n") + "\n"

	inBuf := strings.NewReader(input)
	outBuf := new(bytes.Buffer)

	loop := refine.NewSessionLoop(refine.LoopConfig{
		Engine:   engine,
		Store:    store,
		Snapshot: snap,
		In:       inBuf,
		Out:      outBuf,
	})

	err := loop.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected loop error: %v", err)
	}

	if snap.Status != "finalized" {
		t.Errorf("expected status 'finalized', got '%s'", snap.Status)
	}
	// Rounds: Round 1 (Q1), Round 2 (Q2), Round 3 (USER-REQ-3), Round 4 (Q3)
	if len(snap.Rounds) != 4 {
		t.Fatalf("expected 4 rounds, got %d", len(snap.Rounds))
	}

	loaded, err := store.Load(snap.Key)
	if err != nil {
		t.Fatalf("failed to load snapshot from store: %v", err)
	}
	if len(loaded.Rounds) != 4 {
		t.Fatalf("expected 4 loaded rounds, got %d", len(loaded.Rounds))
	}
	if loaded.Rounds[1].Questions[0].Answer != "Strict" {
		t.Errorf("round 2 answer expected 'Strict', got '%s'", loaded.Rounds[1].Questions[0].Answer)
	}
	if loaded.Rounds[2].Questions[0].Answer != "Zero-downtime rolling upgrades required" {
		t.Errorf("round 3 requirement text mismatch, got '%s'", loaded.Rounds[2].Questions[0].Answer)
	}
	if loaded.Rounds[3].Questions[0].Answer != "CooperativeSticky" {
		t.Errorf("round 4 answer expected 'CooperativeSticky', got '%s'", loaded.Rounds[3].Questions[0].Answer)
	}
	if snap.Tree == nil || len(snap.Tree.Epics) != 1 {
		t.Fatalf("expected snap.Tree to be populated, got %+v", snap.Tree)
	}
	if loaded.Tree == nil || len(loaded.Tree.Epics) != 1 {
		t.Fatalf("expected loaded.Tree to be populated, got %+v", loaded.Tree)
	}
}

func TestSessionLoop_Run_ResumeExistingFrontier(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)

	snap := &session.Snapshot{
		Key:    "STRAT-30",
		Status: "in-progress",
		Ticket: jira.Ticket{
			Key:     "STRAT-30",
			Summary: "Audit Logging Service",
		},
		CurrentFrontier: []session.Question{
			{
				ID:             "Q1",
				Title:          "Log Retention Period",
				Options:        []string{"30 days", "90 days", "365 days"},
				Recommendation: "90 days",
			},
		},
	}
	if err := store.Save(snap); err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	mockAI := &sequentialMockAI{
		responses: []string{
			`[]`,
			`{"epics":[{"id":"EPIC-1","title":"Audit Logging","tasks":[{"id":"TASK-1","title":"Retention Worker"}]}]}`,
		},
	}
	engine := refine.NewEngine(mockAI)

	// Input: "3" (select 365 days), then Enter to finalize
	input := "3\n\n"
	inBuf := strings.NewReader(input)
	outBuf := new(bytes.Buffer)

	loop := refine.NewSessionLoop(refine.LoopConfig{
		Engine:   engine,
		Store:    store,
		Snapshot: snap,
		In:       inBuf,
		Out:      outBuf,
	})

	err := loop.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected loop error: %v", err)
	}

	// Engine should have been called for the next round (which was empty), and once for decomposition tree
	if mockAI.callCount != 2 {
		t.Errorf("expected 2 AI calls, got %d", mockAI.callCount)
	}

	loaded, err := store.Load(snap.Key)
	if err != nil {
		t.Fatalf("failed to load snapshot: %v", err)
	}
	if len(loaded.Rounds) != 1 {
		t.Fatalf("expected 1 round, got %d", len(loaded.Rounds))
	}
	if loaded.Rounds[0].Questions[0].Answer != "365 days" {
		t.Errorf("expected answer '365 days', got '%s'", loaded.Rounds[0].Questions[0].Answer)
	}
	if loaded.Status != "finalized" {
		t.Errorf("expected status 'finalized', got '%s'", loaded.Status)
	}
	if snap.Tree == nil || len(snap.Tree.Epics) != 1 {
		t.Fatalf("expected snap.Tree to be populated, got %+v", snap.Tree)
	}
	if loaded.Tree == nil || len(loaded.Tree.Epics) != 1 {
		t.Fatalf("expected loaded.Tree to be populated, got %+v", loaded.Tree)
	}
}

