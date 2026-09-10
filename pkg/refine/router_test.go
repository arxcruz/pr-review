package refine_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/arxcruz/pr-review/pkg/jira"
	"github.com/arxcruz/pr-review/pkg/refine"
	"github.com/arxcruz/pr-review/pkg/session"
)

func TestRouter_Route_RuleBased(t *testing.T) {
	cfg := &config.JiraConfig{
		OriginProject: "ORIGIN",
		Teams: map[string]config.JiraTeamConfig{
			"backend": {
				DeliveryProject: "CORE",
				Keywords:        []string{"database", "auth", "token", "service"},
				IssueTypes: config.JiraIssueTypesConfig{
					Epic:  "Epic",
					Task:  "Task",
					Story: "Story",
				},
			},
			"frontend": {
				DeliveryProject: "WEB",
				Keywords:        []string{"ui", "react", "component", "frontend"},
				IssueTypes: config.JiraIssueTypesConfig{
					Epic:  "Feature",
					Task:  "Dev Task",
					Story: "User Story",
				},
			},
		},
	}

	snap := &session.Snapshot{
		Key:    "ORIGIN-1",
		Status: session.StatusInProgress,
		Ticket: jira.Ticket{
			Key:     "ORIGIN-1",
			Summary: "User authentication platform",
		},
		Tree: &session.DecompositionTree{
			Epics: []session.DecompositionEpic{
				{
					ID:          "EPIC-1",
					Title:       "Backend Authentication Service",
					Description: "Core auth service implementation",
					Tasks: []session.DecompositionTask{
						{
							ID:          "TASK-1",
							Title:       "Create token database schema",
							Description: "SQL migrations for tokens",
							Type:        "Task",
						},
						{
							ID:          "TASK-2",
							Title:       "React login UI component",
							Description: "Frontend form in React",
							Type:        "Story",
							DependsOn:   []string{"TASK-1"},
						},
						{
							ID:          "TASK-3",
							Title:       "Configure rate limiter",
							Description: "Limit requests per IP",
							Type:        "Task",
						},
					},
				},
				{
					ID:          "EPIC-2",
					Title:       "Frontend Client Portal",
					Description: "Web portal UI features",
					Tasks: []session.DecompositionTask{
						{
							ID:          "TASK-4",
							Title:       "Build dashboard view",
							Description: "Component tree for user portal",
							Type:        "Story",
						},
					},
				},
			},
		},
	}

	router := refine.NewRouter(cfg)
	ctx := context.Background()

	err := router.Route(ctx, snap)
	if err != nil {
		t.Fatalf("unexpected error routing snapshot: %v", err)
	}

	tree := snap.Tree
	if len(tree.Epics) != 2 {
		t.Fatalf("expected 2 epics, got %d", len(tree.Epics))
	}

	// EPIC-1 should be mapped to CORE with issue type "Epic"
	epic1 := tree.Epics[0]
	if epic1.DeliveryProject != "CORE" {
		t.Errorf("EPIC-1 delivery project: expected CORE, got %q", epic1.DeliveryProject)
	}
	if epic1.Type != "Epic" {
		t.Errorf("EPIC-1 type: expected Epic, got %q", epic1.Type)
	}

	// TASK-1 should be mapped to CORE (keyword: database)
	task1 := epic1.Tasks[0]
	if task1.DeliveryProject != "CORE" {
		t.Errorf("TASK-1 delivery project: expected CORE, got %q", task1.DeliveryProject)
	}
	if task1.Type != "Task" {
		t.Errorf("TASK-1 type: expected Task, got %q", task1.Type)
	}

	// TASK-2 should be mapped to WEB (keyword: react, ui) and Story -> "User Story"
	task2 := epic1.Tasks[1]
	if task2.DeliveryProject != "WEB" {
		t.Errorf("TASK-2 delivery project: expected WEB, got %q", task2.DeliveryProject)
	}
	if task2.Type != "User Story" {
		t.Errorf("TASK-2 type: expected User Story, got %q", task2.Type)
	}

	// TASK-3 should inherit parent epic CORE, type "Task"
	task3 := epic1.Tasks[2]
	if task3.DeliveryProject != "CORE" {
		t.Errorf("TASK-3 delivery project: expected CORE (inherited), got %q", task3.DeliveryProject)
	}
	if task3.Type != "Task" {
		t.Errorf("TASK-3 type: expected Task, got %q", task3.Type)
	}

	// EPIC-2 should be mapped to WEB (keyword: frontend, ui), type "Feature"
	epic2 := tree.Epics[1]
	if epic2.DeliveryProject != "WEB" {
		t.Errorf("EPIC-2 delivery project: expected WEB, got %q", epic2.DeliveryProject)
	}
	if epic2.Type != "Feature" {
		t.Errorf("EPIC-2 type: expected Feature, got %q", epic2.Type)
	}

	// TASK-4 should be mapped to WEB, type "User Story"
	task4 := epic2.Tasks[0]
	if task4.DeliveryProject != "WEB" {
		t.Errorf("TASK-4 delivery project: expected WEB, got %q", task4.DeliveryProject)
	}
	if task4.Type != "User Story" {
		t.Errorf("TASK-4 type: expected User Story, got %q", task4.Type)
	}
}

func TestRouter_Route_FallbackToOrigin(t *testing.T) {
	cfg := &config.JiraConfig{
		OriginProject: "ORIGIN-PROJECT",
		Teams: map[string]config.JiraTeamConfig{
			"backend": {
				DeliveryProject: "CORE",
				Keywords:        []string{"golang", "grpc"},
			},
		},
	}

	snap := &session.Snapshot{
		Key: "ORIGIN-1",
		Tree: &session.DecompositionTree{
			Epics: []session.DecompositionEpic{
				{
					ID:          "EPIC-1",
					Title:       "Legal Compliance Audit",
					Description: "Review terms of service agreements",
					Tasks: []session.DecompositionTask{
						{
							ID:          "TASK-1",
							Title:       "Draft policy document",
							Description: "Review document with legal counsel",
						},
					},
				},
			},
		},
	}

	router := refine.NewRouter(cfg)
	err := router.Route(context.Background(), snap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	epic := snap.Tree.Epics[0]
	if epic.DeliveryProject != "ORIGIN-PROJECT" {
		t.Errorf("expected fallback to ORIGIN-PROJECT, got %q", epic.DeliveryProject)
	}
	if epic.Type != "Epic" {
		t.Errorf("expected default Epic type, got %q", epic.Type)
	}

	task := epic.Tasks[0]
	if task.DeliveryProject != "ORIGIN-PROJECT" {
		t.Errorf("expected fallback to ORIGIN-PROJECT, got %q", task.DeliveryProject)
	}
	if task.Type != "Task" {
		t.Errorf("expected default Task type, got %q", task.Type)
	}
}

func TestRouter_Route_InteractiveUserPrompt(t *testing.T) {
	cfg := &config.JiraConfig{
		OriginProject: "ORIGIN",
		Teams: map[string]config.JiraTeamConfig{
			"infra": {
				DeliveryProject: "DEVOPS",
				Keywords:        []string{"terraform", "aws"},
			},
			"qa": {
				DeliveryProject: "TESTING",
				Keywords:        []string{"cypress", "selenium"},
			},
		},
	}

	snap := &session.Snapshot{
		Key: "ORIGIN-1",
		Tree: &session.DecompositionTree{
			Epics: []session.DecompositionEpic{
				{
					ID:          "EPIC-1",
					Title:       "Uncategorized Epic",
					Description: "Misc chores",
				},
			},
		},
	}

	// User inputs "1" to pick the first offered team (infra / DEVOPS)
	input := strings.NewReader("1\n")
	var output bytes.Buffer

	router := refine.NewRouter(cfg, refine.WithInteractive(input, &output))
	err := router.Route(context.Background(), snap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	epic := snap.Tree.Epics[0]
	if epic.DeliveryProject != "DEVOPS" && epic.DeliveryProject != "TESTING" {
		t.Errorf("expected interactive selection to assign team project, got %q", epic.DeliveryProject)
	}
	if !strings.Contains(output.String(), "Uncategorized Epic") {
		t.Errorf("expected output prompt to mention epic title, got: %s", output.String())
	}
}

func TestRouter_Route_AIEngineRecommendation(t *testing.T) {
	cfg := &config.JiraConfig{
		OriginProject: "ORIGIN",
		Teams: map[string]config.JiraTeamConfig{
			"analytics": {
				DeliveryProject: "DATA",
				Keywords:        []string{"spark", "hadoop"},
			},
		},
	}

	snap := &session.Snapshot{
		Key: "ORIGIN-1",
		Tree: &session.DecompositionTree{
			Epics: []session.DecompositionEpic{
				{
					ID:          "EPIC-1",
					Title:       "Clickstream ingestion telemetry",
					Description: "Track events and pipe into warehouse",
				},
			},
		},
	}

	mockAI := &mockAIEngine{
		response: `{"EPIC-1": "DATA"}`,
	}

	router := refine.NewRouter(cfg, refine.WithAIEngine(mockAI))
	err := router.Route(context.Background(), snap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if snap.Tree.Epics[0].DeliveryProject != "DATA" {
		t.Errorf("expected AI recommendation DATA, got %q", snap.Tree.Epics[0].DeliveryProject)
	}
}

func TestFormatPlanMarkdown_OutputsSummary(t *testing.T) {
	cfg := &config.JiraConfig{
		OriginProject: "ORIGIN",
		Teams: map[string]config.JiraTeamConfig{
			"backend": {
				DeliveryProject: "CORE",
			},
			"frontend": {
				DeliveryProject: "WEB",
			},
		},
	}

	snap := &session.Snapshot{
		Key:       "ORIGIN-42",
		Status:    session.StatusFinalized,
		CreatedAt: time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC),
		Ticket: jira.Ticket{
			Key:     "ORIGIN-42",
			Summary: "Implement Single Sign-On (SSO)",
		},
		Tree: &session.DecompositionTree{
			Epics: []session.DecompositionEpic{
				{
					ID:              "EPIC-1",
					Title:           "SSO Backend Service",
					Description:     "SAML/OIDC authentication endpoints",
					DeliveryProject: "CORE",
					Type:            "Epic",
					Tasks: []session.DecompositionTask{
						{
							ID:                 "TASK-1",
							Title:              "Implement SAML validator",
							Description:        "Parse assertion XML",
							DeliveryProject:    "CORE",
							Type:               "Task",
							AcceptanceCriteria: []string{"Validates XML signature", "Returns user identity"},
						},
						{
							ID:                 "TASK-2",
							Title:              "SSO Redirect UI",
							Description:        "Frontend login button",
							DeliveryProject:    "WEB",
							Type:               "Story",
							DependsOn:          []string{"TASK-1"},
							AcceptanceCriteria: []string{"Redirects to IdP", "Handles callback"},
						},
					},
				},
			},
		},
	}

	md := refine.FormatPlanMarkdown(snap, cfg)

	if !strings.Contains(md, "# Refinement Delivery Plan: ORIGIN-42 - Implement Single Sign-On (SSO)") {
		t.Errorf("expected plan title in markdown, got:\n%s", md)
	}
	if !strings.Contains(md, "CORE") || !strings.Contains(md, "WEB") {
		t.Errorf("expected delivery project keys in markdown summary, got:\n%s", md)
	}
	if !strings.Contains(md, "EPIC-1") || !strings.Contains(md, "TASK-1") || !strings.Contains(md, "TASK-2") {
		t.Errorf("expected item IDs in markdown, got:\n%s", md)
	}
	if !strings.Contains(md, "Validates XML signature") {
		t.Errorf("expected acceptance criteria in markdown, got:\n%s", md)
	}
	if !strings.Contains(md, "TASK-1") {
		t.Errorf("expected dependency references in markdown, got:\n%s", md)
	}
}

func TestWritePlanFile(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plans", "ORIGIN-42-plan.md")

	content := "# Test Plan\n\nPlan details here."
	err := refine.WritePlanFile(planPath, content)
	if err != nil {
		t.Fatalf("failed to write plan file: %v", err)
	}

	data, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("failed to read written plan file: %v", err)
	}

	if string(data) != content {
		t.Errorf("expected file content %q, got %q", content, string(data))
	}
}
