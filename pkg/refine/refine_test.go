package refine_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/arxcruz/pr-review/pkg/ai"
	"github.com/arxcruz/pr-review/pkg/jira"
	"github.com/arxcruz/pr-review/pkg/refine"
	"github.com/arxcruz/pr-review/pkg/session"
)

func mustDefaultPromptSet(t *testing.T) *refine.PromptSet {
	t.Helper()
	ps, err := refine.DefaultPromptSet()
	if err != nil {
		t.Fatalf("failed to load default prompt set: %v", err)
	}
	return ps
}

type mockAIEngine struct {
	name           string
	capturedPrompt ai.PromptRequest
	response       string
	err            error
}

func (m *mockAIEngine) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock-ai"
}

func (m *mockAIEngine) Review(ctx context.Context, req ai.ReviewRequest) (*ai.ReviewResult, error) {
	return nil, nil
}

func (m *mockAIEngine) Generate(ctx context.Context, req ai.PromptRequest) (*ai.GenerateResult, error) {
	m.capturedPrompt = req
	if m.err != nil {
		return nil, m.err
	}
	return &ai.GenerateResult{
		Provider:   m.Name(),
		Model:      "mock-model",
		Content:    m.response,
		TokensUsed: 120,
	}, nil
}

func TestBuildFrontierPrompt(t *testing.T) {
	ticket := jira.Ticket{
		Key:         "STRAT-100",
		Summary:     "Migrate Auth to Distributed OAuth2",
		Description: "Decouple legacy auth service into distributed OAuth2 tokens.",
		IssueType:   "Epic",
		Status:      "Open",
		IssueLinks: []jira.IssueLink{
			{Relationship: "blocks", Key: "DELIV-1", Summary: "Implement token service", Status: "To Do"},
		},
		Comments: []jira.Comment{
			{Author: "Alice", Body: "Ensure latency is under 50ms."},
		},
	}

	docContext := "## Multi-Repo Docs\n### [auth-repo] CONTEXT.md\nOAuth2 boundaries"
	guidelines := "Always favor OIDC compliant JWTs."

	now := time.Now().UTC()
	rounds := []session.Round{
		{
			Number: 1,
			Questions: []session.Question{
				{
					ID:             "Q1",
					Title:          "Token Format",
					Options:        []string{"JWT", "Paseto"},
					Recommendation: "JWT",
					Answer:         "JWT",
				},
			},
			AnsweredAt: &now,
		},
	}

	ps := mustDefaultPromptSet(t)
	ps.Guidelines = guidelines

	sysPrompt, userPrompt, err := refine.BuildFrontierPrompt(ps, ticket, rounds, docContext)
	if err != nil {
		t.Fatalf("unexpected error building frontier prompt: %v", err)
	}

	// System prompt should contain instructions and schema
	if !strings.Contains(sysPrompt, "frontier questions") {
		t.Errorf("expected sysPrompt to describe frontier questions, got: %s", sysPrompt)
	}
	if !strings.Contains(sysPrompt, "recommendation") {
		t.Errorf("expected sysPrompt to mention recommendations, got: %s", sysPrompt)
	}

	// User prompt should contain ticket details
	if !strings.Contains(userPrompt, "STRAT-100") {
		t.Errorf("expected userPrompt to contain ticket key, got: %s", userPrompt)
	}
	if !strings.Contains(userPrompt, "Migrate Auth to Distributed OAuth2") {
		t.Errorf("expected userPrompt to contain ticket summary, got: %s", userPrompt)
	}
	if !strings.Contains(userPrompt, "Decouple legacy auth service") {
		t.Errorf("expected userPrompt to contain description, got: %s", userPrompt)
	}
	if !strings.Contains(userPrompt, "DELIV-1") {
		t.Errorf("expected userPrompt to contain linked issue, got: %s", userPrompt)
	}
	if !strings.Contains(userPrompt, "Ensure latency is under 50ms") {
		t.Errorf("expected userPrompt to contain comment, got: %s", userPrompt)
	}

	// Should contain doc context
	if !strings.Contains(userPrompt, "OAuth2 boundaries") {
		t.Errorf("expected userPrompt to contain docContext, got: %s", userPrompt)
	}

	// Should contain guidelines
	if !strings.Contains(userPrompt, "Always favor OIDC compliant JWTs") {
		t.Errorf("expected userPrompt to contain guidelines, got: %s", userPrompt)
	}

	// Should contain previous rounds
	if !strings.Contains(userPrompt, "Round 1") || !strings.Contains(userPrompt, "Token Format") || !strings.Contains(userPrompt, "JWT") {
		t.Errorf("expected userPrompt to contain previous round 1 details, got: %s", userPrompt)
	}
}

func TestBuildDecompositionPrompt(t *testing.T) {
	ticket := jira.Ticket{
		Key:         "STRAT-100",
		Summary:     "Migrate Auth to Distributed OAuth2",
		Description: "Decouple legacy auth service into distributed OAuth2 tokens.",
	}

	now := time.Now().UTC()
	rounds := []session.Round{
		{
			Number: 1,
			Questions: []session.Question{
				{
					ID:     "Q1",
					Title:  "Token Format",
					Answer: "JWT",
				},
			},
			AnsweredAt: &now,
		},
	}

	docContext := "## Multi-Repo Docs\nOAuth2 boundaries"
	guidelines := "Always favor OIDC compliant JWTs."

	ps := mustDefaultPromptSet(t)
	ps.Guidelines = guidelines

	sysPrompt, userPrompt, err := refine.BuildDecompositionPrompt(ps, ticket, rounds, docContext)
	if err != nil {
		t.Fatalf("unexpected error building decomposition prompt: %v", err)
	}

	if !strings.Contains(sysPrompt, "Decomposition Tree") {
		t.Errorf("expected sysPrompt to mention Decomposition Tree, got: %s", sysPrompt)
	}
	if !strings.Contains(sysPrompt, "acceptance_criteria") {
		t.Errorf("expected sysPrompt to mention acceptance_criteria, got: %s", sysPrompt)
	}
	if !strings.Contains(sysPrompt, "depends_on") {
		t.Errorf("expected sysPrompt to mention depends_on, got: %s", sysPrompt)
	}

	if !strings.Contains(userPrompt, "STRAT-100") || !strings.Contains(userPrompt, "Migrate Auth to Distributed OAuth2") {
		t.Errorf("expected userPrompt to include ticket info, got: %s", userPrompt)
	}
	if !strings.Contains(userPrompt, "Token Format") || !strings.Contains(userPrompt, "JWT") {
		t.Errorf("expected userPrompt to include settled interview round answers, got: %s", userPrompt)
	}
	if !strings.Contains(userPrompt, "OAuth2 boundaries") {
		t.Errorf("expected userPrompt to include doc context, got: %s", userPrompt)
	}
	if !strings.Contains(userPrompt, "Always favor OIDC compliant JWTs") {
		t.Errorf("expected userPrompt to include guidelines, got: %s", userPrompt)
	}
}

func TestParseQuestions_ValidJSON(t *testing.T) {
	raw := `[
		{
			"id": "Q1",
			"title": "Database Storage",
			"explanation": "Need high throughput persistence.",
			"options": ["PostgreSQL", "DynamoDB"],
			"recommendation": "PostgreSQL"
		},
		{
			"title": "Caching Strategy",
			"explanation": "Cache session tokens.",
			"options": ["Redis", "Memcached"],
			"recommendation": "Redis"
		}
	]`

	questions, err := refine.ParseQuestions(raw)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(questions))
	}

	if questions[0].ID != "Q1" || questions[0].Title != "Database Storage" {
		t.Errorf("unexpected question 0: %+v", questions[0])
	}
	if questions[0].Recommendation != "PostgreSQL" {
		t.Errorf("expected recommendation PostgreSQL, got %s", questions[0].Recommendation)
	}

	// Auto-populated ID for second question
	if questions[1].ID != "Q2" {
		t.Errorf("expected auto-assigned ID Q2, got %s", questions[1].ID)
	}
}

func TestParseQuestions_MarkdownFenced(t *testing.T) {
	raw := "Here are the frontier questions for your ticket:\n\n```json\n" + `[
		{
			"id": "Q1",
			"title": "Async Messaging",
			"explanation": "Event pub/sub bus.",
			"options": ["Kafka", "RabbitMQ"],
			"recommendation": "Kafka"
		}
	]` + "\n```\n\nPlease let me know if you want changes."

	questions, err := refine.ParseQuestions(raw)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(questions))
	}
	if questions[0].Title != "Async Messaging" {
		t.Errorf("expected title 'Async Messaging', got %s", questions[0].Title)
	}
}

func TestParseQuestions_EmptyOrResolved(t *testing.T) {
	raw := "[]"
	questions, err := refine.ParseQuestions(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(questions) != 0 {
		t.Errorf("expected 0 questions, got %d", len(questions))
	}
}

func TestParseDecompositionTree_ValidJSON(t *testing.T) {
	raw := `{
		"epics": [
			{
				"id": "EPIC-1",
				"title": "Authentication Core",
				"description": "OAuth2 / OIDC backend",
				"delivery_project": "AUTH",
				"tasks": [
					{
						"id": "TASK-1",
						"title": "JWT Token Verification",
						"description": "Verify tokens via JWKS",
						"type": "Story",
						"acceptance_criteria": ["Caches JWKS keys", "Rejects expired"],
						"delivery_project": "AUTH",
						"depends_on": []
					}
				]
			}
		]
	}`

	tree, err := refine.ParseDecompositionTree(raw)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(tree.Epics) != 1 {
		t.Fatalf("expected 1 epic, got %d", len(tree.Epics))
	}
	epic := tree.Epics[0]
	if epic.ID != "EPIC-1" || epic.Title != "Authentication Core" {
		t.Errorf("unexpected epic: %+v", epic)
	}
	if len(epic.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(epic.Tasks))
	}
	task := epic.Tasks[0]
	if task.ID != "TASK-1" || task.Type != "Story" || len(task.AcceptanceCriteria) != 2 {
		t.Errorf("unexpected task: %+v", task)
	}
}

func TestParseDecompositionTree_MarkdownFencedAndArrayRoot(t *testing.T) {
	raw := "Here is the resulting decomposition tree:\n\n```json\n" + `[
		{
			"id": "EPIC-1",
			"title": "Event Bus",
			"tasks": [
				{
					"id": "TASK-1",
					"title": "Configure Kafka Topics",
					"acceptance_criteria": ["Partition count configured"]
				}
			]
		}
	]` + "\n```\nHope this helps!"

	tree, err := refine.ParseDecompositionTree(raw)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(tree.Epics) != 1 {
		t.Fatalf("expected 1 epic, got %d", len(tree.Epics))
	}
	if tree.Epics[0].Tasks[0].Title != "Configure Kafka Topics" {
		t.Errorf("unexpected task title: %s", tree.Epics[0].Tasks[0].Title)
	}
}

func TestParseDecompositionTree_MalformedJSON(t *testing.T) {
	raw := "Internal server error: LLM crashed"
	_, err := refine.ParseDecompositionTree(raw)
	if err == nil {
		t.Fatal("expected error on malformed json, got nil")
	}
}

func TestParseDecompositionTree_ValidationError(t *testing.T) {
	// Task depending on non-existent task
	raw := `{
		"epics": [
			{
				"id": "EPIC-1",
				"title": "Epic",
				"tasks": [
					{
						"id": "TASK-1",
						"title": "Task 1",
						"depends_on": ["NONEXISTENT-99"]
					}
				]
			}
		]
	}`

	_, err := refine.ParseDecompositionTree(raw)
	if err == nil {
		t.Fatal("expected validation error on non-existent dependency, got nil")
	}
	if !strings.Contains(err.Error(), "unknown dependency") {
		t.Errorf("expected error to mention unknown dependency, got: %v", err)
	}
}

func TestGenerateFrontier_Success(t *testing.T) {
	llmResponse := `[
		{
			"id": "Q1",
			"title": "Protocol Selection",
			"explanation": "RPC vs HTTP",
			"options": ["gRPC", "REST"],
			"recommendation": "gRPC"
		}
	]`

	mockAI := &mockAIEngine{response: llmResponse}
	engine := refine.NewEngine(mockAI, mustDefaultPromptSet(t))

	snap := &session.Snapshot{
		Key:     "STRAT-42",
		Status:  "new",
		Ticket: jira.Ticket{
			Key:     "STRAT-42",
			Summary: "Build Core Microservices",
		},
	}

	opts := refine.FrontierOptions{
		DocContext: "## Architecture Docs",
	}

	ctx := context.Background()
	questions, err := engine.GenerateFrontier(ctx, snap, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(questions))
	}
	if questions[0].ID != "Q1" {
		t.Errorf("expected question ID Q1, got %s", questions[0].ID)
	}

	// Verify integration into snapshot
	if len(snap.CurrentFrontier) != 1 {
		t.Fatalf("expected snapshot CurrentFrontier to have 1 question, got %d", len(snap.CurrentFrontier))
	}
	if snap.CurrentFrontier[0].Title != "Protocol Selection" {
		t.Errorf("expected snapshot CurrentFrontier title 'Protocol Selection', got %s", snap.CurrentFrontier[0].Title)
	}
	if snap.Status != "in-progress" {
		t.Errorf("expected snapshot status 'in-progress', got %s", snap.Status)
	}
	if snap.UpdatedAt.IsZero() {
		t.Errorf("expected UpdatedAt to be set")
	}

	// Verify prompt was passed to AI
	if !strings.Contains(mockAI.capturedPrompt.UserPrompt, "STRAT-42") {
		t.Errorf("expected AI prompt to contain ticket key")
	}
}

func TestAdvanceRound(t *testing.T) {
	snap := &session.Snapshot{
		Key: "STRAT-42",
		CurrentFrontier: []session.Question{
			{
				ID:             "Q1",
				Title:          "Protocol",
				Recommendation: "gRPC",
				Answer:         "gRPC",
			},
		},
	}

	refine.AdvanceRound(snap)

	if len(snap.CurrentFrontier) != 0 {
		t.Errorf("expected CurrentFrontier to be cleared, got %d items", len(snap.CurrentFrontier))
	}
	if len(snap.Rounds) != 1 {
		t.Fatalf("expected 1 round in snapshot, got %d", len(snap.Rounds))
	}
	if snap.Rounds[0].Number != 1 {
		t.Errorf("expected Round Number 1, got %d", snap.Rounds[0].Number)
	}
	if snap.Rounds[0].AnsweredAt == nil {
		t.Errorf("expected AnsweredAt to be set")
	}
	if snap.Rounds[0].Questions[0].Answer != "gRPC" {
		t.Errorf("expected Question answer 'gRPC', got %s", snap.Rounds[0].Questions[0].Answer)
	}
}

func TestParseQuestions_ObjectWrapper(t *testing.T) {
	raw := `{
		"questions": [
			{
				"id": "Q1",
				"title": "Auth Provider",
				"explanation": "Choose provider.",
				"options": ["Auth0", "Keycloak"],
				"recommendation": "Keycloak"
			}
		]
	}`

	questions, err := refine.ParseQuestions(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(questions))
	}
	if questions[0].Title != "Auth Provider" {
		t.Errorf("expected title 'Auth Provider', got %s", questions[0].Title)
	}
}

func TestParseQuestions_InvalidJSON(t *testing.T) {
	raw := `Internal server error: rate limit exceeded`
	_, err := refine.ParseQuestions(raw)
	if err == nil {
		t.Fatal("expected error on invalid JSON, got nil")
	}
}

func TestGenerateFrontier_NilSnapshot(t *testing.T) {
	engine := refine.NewEngine(&mockAIEngine{}, mustDefaultPromptSet(t))
	_, err := engine.GenerateFrontier(context.Background(), nil, refine.FrontierOptions{})
	if err == nil {
		t.Fatal("expected error on nil snapshot, got nil")
	}
}

func TestGenerateFrontier_AIError(t *testing.T) {
	mockAI := &mockAIEngine{err: context.DeadlineExceeded}
	engine := refine.NewEngine(mockAI, mustDefaultPromptSet(t))
	snap := &session.Snapshot{Key: "STRAT-1"}
	_, err := engine.GenerateFrontier(context.Background(), snap, refine.FrontierOptions{})
	if err == nil {
		t.Fatal("expected error on AI failure, got nil")
	}
	if !strings.Contains(err.Error(), "failed to generate frontier questions") {
		t.Errorf("expected actionable error message, got: %v", err)
	}
}

type sequenceMockAI struct {
	responses []string
	callCount int
}

func (s *sequenceMockAI) Name() string { return "sequence-mock" }
func (s *sequenceMockAI) Review(ctx context.Context, req ai.ReviewRequest) (*ai.ReviewResult, error) {
	return nil, nil
}
func (s *sequenceMockAI) Generate(ctx context.Context, req ai.PromptRequest) (*ai.GenerateResult, error) {
	if s.callCount >= len(s.responses) {
		return &ai.GenerateResult{Content: "{}"}, nil
	}
	resp := s.responses[s.callCount]
	s.callCount++
	return &ai.GenerateResult{Provider: s.Name(), Content: resp}, nil
}

func TestGenerateDecompositionTree_Success(t *testing.T) {
	validTreeJSON := `{
		"epics": [
			{
				"id": "EPIC-1",
				"title": "Auth Platform",
				"tasks": [
					{
						"id": "TASK-1",
						"title": "JWT Endpoint",
						"acceptance_criteria": ["Generates RS256 JWT"]
					}
				]
			}
		]
	}`

	mockAI := &sequenceMockAI{responses: []string{validTreeJSON}}
	engine := refine.NewEngine(mockAI, mustDefaultPromptSet(t))

	snap := &session.Snapshot{
		Key:    "STRAT-1",
		Status: "in-progress",
		Ticket: jira.Ticket{Key: "STRAT-1", Summary: "Auth Modernization"},
	}

	tree, err := engine.GenerateDecompositionTree(context.Background(), snap, refine.DecompositionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tree == nil || len(tree.Epics) != 1 {
		t.Fatalf("expected 1 epic, got %+v", tree)
	}
	if snap.Tree == nil || snap.Tree != tree {
		t.Errorf("expected snapshot.Tree to be populated with generated tree")
	}
	if snap.UpdatedAt.IsZero() {
		t.Errorf("expected snapshot.UpdatedAt to be updated")
	}
	if mockAI.callCount != 1 {
		t.Errorf("expected exactly 1 AI call, got %d", mockAI.callCount)
	}
}

func TestGenerateDecompositionTree_RetrySuccess(t *testing.T) {
	malformedJSON := "Error: Could not output valid JSON"
	validTreeJSON := `{
		"epics": [
			{
				"id": "EPIC-1",
				"title": "Data Pipeline",
				"tasks": [
					{
						"id": "TASK-1",
						"title": "Ingestion Worker",
						"acceptance_criteria": ["Ingests 1000 events/sec"]
					}
				]
			}
		]
	}`

	mockAI := &sequenceMockAI{responses: []string{malformedJSON, validTreeJSON}}
	engine := refine.NewEngine(mockAI, mustDefaultPromptSet(t))

	snap := &session.Snapshot{
		Key:    "STRAT-2",
		Status: "in-progress",
		Ticket: jira.Ticket{Key: "STRAT-2", Summary: "Data Ingestion Pipeline"},
	}

	tree, err := engine.GenerateDecompositionTree(context.Background(), snap, refine.DecompositionOptions{
		MaxRetries: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}

	if tree == nil || len(tree.Epics) != 1 {
		t.Fatalf("expected valid tree after retry, got %+v", tree)
	}
	if mockAI.callCount != 2 {
		t.Errorf("expected 2 AI calls (1 failure + 1 retry success), got %d", mockAI.callCount)
	}
}

func TestGenerateDecompositionTree_RetryExhausted(t *testing.T) {
	mockAI := &sequenceMockAI{responses: []string{"bad-json-1", "bad-json-2", "bad-json-3"}}
	engine := refine.NewEngine(mockAI, mustDefaultPromptSet(t))

	snap := &session.Snapshot{
		Key: "STRAT-3",
	}

	_, err := engine.GenerateDecompositionTree(context.Background(), snap, refine.DecompositionOptions{
		MaxRetries: 2,
	})
	if err == nil {
		t.Fatal("expected error when retries exhausted, got nil")
	}
	if !strings.Contains(err.Error(), "failed to generate valid decomposition tree") {
		t.Errorf("expected retry failure message, got: %v", err)
	}
	if mockAI.callCount != 3 { // 1 initial + 2 retries
		t.Errorf("expected 3 AI calls, got %d", mockAI.callCount)
	}
}

func TestGenerateDecompositionTree_NilSnapshot(t *testing.T) {
	engine := refine.NewEngine(&mockAIEngine{}, mustDefaultPromptSet(t))
	_, err := engine.GenerateDecompositionTree(context.Background(), nil, refine.DecompositionOptions{})
	if err == nil {
		t.Fatal("expected error on nil snapshot, got nil")
	}
}


