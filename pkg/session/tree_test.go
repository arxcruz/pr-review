package session_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/arxcruz/pr-review/pkg/session"
)

func TestDecompositionTree_Validate_Valid(t *testing.T) {
	tree := &session.DecompositionTree{
		Epics: []session.DecompositionEpic{
			{
				ID:          "EPIC-1",
				Title:       "Auth Modernization",
				Description: "OAuth2 migration",
				Tasks: []session.DecompositionTask{
					{
						ID:                 "TASK-1",
						Title:              "Token Issuer",
						Type:               "Story",
						AcceptanceCriteria: []string{"RS256 signing", "Token rotation"},
						DependsOn:          []string{},
					},
					{
						ID:                 "TASK-2",
						Title:              "Token Validator",
						Type:               "Task",
						AcceptanceCriteria: []string{"Public key caching"},
						DependsOn:          []string{"TASK-1"},
					},
				},
			},
		},
	}

	if err := tree.Validate(); err != nil {
		t.Fatalf("expected valid tree, got error: %v", err)
	}
}

func TestDecompositionTree_Validate_NilOrEmpty(t *testing.T) {
	var nilTree *session.DecompositionTree
	if err := nilTree.Validate(); err == nil || !strings.Contains(err.Error(), "nil") {
		t.Errorf("expected error mentioning nil, got: %v", err)
	}

	emptyTree := &session.DecompositionTree{}
	if err := emptyTree.Validate(); err == nil || !strings.Contains(err.Error(), "at least one epic") {
		t.Errorf("expected error mentioning at least one epic, got: %v", err)
	}
}

func TestDecompositionTree_Validate_MissingFields(t *testing.T) {
	// Missing epic ID
	tree1 := &session.DecompositionTree{
		Epics: []session.DecompositionEpic{
			{
				ID:    "",
				Title: "Epic without ID",
			},
		},
	}
	if err := tree1.Validate(); err == nil || !strings.Contains(err.Error(), "epic id is required") {
		t.Errorf("expected error for missing epic ID, got: %v", err)
	}

	// Missing epic Title
	tree2 := &session.DecompositionTree{
		Epics: []session.DecompositionEpic{
			{
				ID:    "EPIC-1",
				Title: "",
			},
		},
	}
	if err := tree2.Validate(); err == nil || !strings.Contains(err.Error(), "epic title is required") {
		t.Errorf("expected error for missing epic title, got: %v", err)
	}

	// Missing task ID
	tree3 := &session.DecompositionTree{
		Epics: []session.DecompositionEpic{
			{
				ID:    "EPIC-1",
				Title: "Valid Epic",
				Tasks: []session.DecompositionTask{
					{
						ID:    "",
						Title: "Task without ID",
					},
				},
			},
		},
	}
	if err := tree3.Validate(); err == nil || !strings.Contains(err.Error(), "task id is required") {
		t.Errorf("expected error for missing task ID, got: %v", err)
	}

	// Missing task Title
	tree4 := &session.DecompositionTree{
		Epics: []session.DecompositionEpic{
			{
				ID:    "EPIC-1",
				Title: "Valid Epic",
				Tasks: []session.DecompositionTask{
					{
						ID:    "TASK-1",
						Title: "",
					},
				},
			},
		},
	}
	if err := tree4.Validate(); err == nil || !strings.Contains(err.Error(), "task title is required") {
		t.Errorf("expected error for missing task title, got: %v", err)
	}
}

func TestDecompositionTree_Validate_DuplicateIDs(t *testing.T) {
	tree := &session.DecompositionTree{
		Epics: []session.DecompositionEpic{
			{
				ID:    "ID-1",
				Title: "Epic 1",
				Tasks: []session.DecompositionTask{
					{
						ID:    "ID-1", // duplicate with epic
						Title: "Task 1",
					},
				},
			},
		},
	}
	if err := tree.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate id") {
		t.Errorf("expected duplicate ID error, got: %v", err)
	}
}

func TestDecompositionTree_Validate_Dependencies(t *testing.T) {
	// Self dependency
	treeSelf := &session.DecompositionTree{
		Epics: []session.DecompositionEpic{
			{
				ID:    "EPIC-1",
				Title: "Epic 1",
				Tasks: []session.DecompositionTask{
					{
						ID:        "TASK-1",
						Title:     "Task 1",
						DependsOn: []string{"TASK-1"},
					},
				},
			},
		},
	}
	if err := treeSelf.Validate(); err == nil || !strings.Contains(err.Error(), "cannot depend on itself") {
		t.Errorf("expected self-dependency error, got: %v", err)
	}

	// Unknown dependency
	treeUnknown := &session.DecompositionTree{
		Epics: []session.DecompositionEpic{
			{
				ID:    "EPIC-1",
				Title: "Epic 1",
				Tasks: []session.DecompositionTask{
					{
						ID:        "TASK-1",
						Title:     "Task 1",
						DependsOn: []string{"NONEXISTENT-99"},
					},
				},
			},
		},
	}
	if err := treeUnknown.Validate(); err == nil || !strings.Contains(err.Error(), "unknown dependency") {
		t.Errorf("expected unknown dependency error, got: %v", err)
	}

	// Circular dependency: TASK-1 -> TASK-2 -> TASK-1
	treeCycle := &session.DecompositionTree{
		Epics: []session.DecompositionEpic{
			{
				ID:    "EPIC-1",
				Title: "Epic 1",
				Tasks: []session.DecompositionTask{
					{
						ID:        "TASK-1",
						Title:     "Task 1",
						DependsOn: []string{"TASK-2"},
					},
					{
						ID:        "TASK-2",
						Title:     "Task 2",
						DependsOn: []string{"TASK-1"},
					},
				},
			},
		},
	}
	if err := treeCycle.Validate(); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected circular dependency cycle error, got: %v", err)
	}
}

func TestDecompositionTree_JSONSchema_ValidJSON(t *testing.T) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(session.DecompositionTreeJSONSchema), &parsed); err != nil {
		t.Fatalf("DecompositionTreeJSONSchema is not valid JSON: %v", err)
	}
	if parsed["title"] != "DecompositionTree" {
		t.Errorf("expected schema title 'DecompositionTree', got %v", parsed["title"])
	}
}
