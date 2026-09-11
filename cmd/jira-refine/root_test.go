package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arxcruz/pr-review/pkg/jira"
	"github.com/arxcruz/pr-review/pkg/session"
	tea "github.com/charmbracelet/bubbletea"
)

func TestRootCmd_Dump_Success(t *testing.T) {
	mockTicket := map[string]interface{}{
		"key": "STRAT-42",
		"fields": map[string]interface{}{
			"summary":     "Strategic Architecture Migration",
			"description": "Break down services into modular components",
			"status": map[string]interface{}{
				"name": "In Progress",
			},
			"issuetype": map[string]interface{}{
				"name": "Epic",
			},
			"comment": map[string]interface{}{
				"comments": []map[string]interface{}{
					{
						"id":      "1",
						"author":  map[string]interface{}{"displayName": "Lead Architect"},
						"body":    "Discovery completed.",
						"created": "2026-03-01T12:00:00.000+0000",
					},
				},
			},
			"issuelinks": []map[string]interface{}{
				{
					"id": "10",
					"type": map[string]interface{}{
						"name":    "Blocks",
						"outward": "blocks",
					},
					"outwardIssue": map[string]interface{}{
						"key": "DELIV-1",
						"fields": map[string]interface{}{
							"summary": "Implement schema loader",
							"status":  map[string]interface{}{"name": "Done"},
						},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockTicket)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `
jira:
  url: "` + server.URL + `"
  pat: "test-pat"
  origin_project: "STRAT"
  teams:
    backend:
      delivery_project: "DELIV"
  doc_paths:
    - "` + tmpDir + `"
`
	if err := os.WriteFile(cfgFile, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	buf := new(bytes.Buffer)
	cmd := newRootCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"STRAT-42", "--dump", "--config", cfgFile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected execute success, got: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "STRAT-42") {
		t.Errorf("expected output to contain ticket key, got: %s", out)
	}
	if !strings.Contains(out, "Strategic Architecture Migration") {
		t.Errorf("expected output to contain ticket summary, got: %s", out)
	}
	if !strings.Contains(out, "Discovery completed.") {
		t.Errorf("expected output to contain comment, got: %s", out)
	}
	if !strings.Contains(out, "DELIV-1") {
		t.Errorf("expected output to contain linked issue, got: %s", out)
	}
}

func TestRootCmd_Dump_MissingKey(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newRootCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--dump"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when key is missing with --dump, got nil")
	}
	if !strings.Contains(err.Error(), "ticket key is required") {
		t.Errorf("expected error mentioning ticket key is required, got: %v", err)
	}
}

func TestRootCmd_Dump_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errorMessages":["Unauthorized access"]}`))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `
jira:
  url: "` + server.URL + `"
  token: "bad-token"
  origin_project: "STRAT"
  teams:
    team:
      delivery_project: "DELIV"
  doc_paths:
    - "` + tmpDir + `"
`
	if err := os.WriteFile(cfgFile, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	buf := new(bytes.Buffer)
	cmd := newRootCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"STRAT-42", "--dump", "--config", cfgFile})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error on auth failure, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") && !strings.Contains(err.Error(), "401") {
		t.Errorf("expected actionable auth error, got: %v", err)
	}
}

func TestRootCmd_NoArgs_ListRecentTickets_Success(t *testing.T) {
	mockResponse := map[string]interface{}{
		"startAt":    0,
		"maxResults": 50,
		"total":      2,
		"issues": []map[string]interface{}{
			{
				"key": "STRAT-100",
				"fields": map[string]interface{}{
					"summary": "Implement Auth Subsystem",
					"status": map[string]interface{}{
						"name": "In Progress",
					},
					"assignee": map[string]interface{}{
						"displayName": "Alice Smith",
						"name":        "asmith",
					},
				},
			},
			{
				"key": "STRAT-101",
				"fields": map[string]interface{}{
					"summary": "Database Sharding Plan",
					"status": map[string]interface{}{
						"name": "Open",
					},
					"assignee": nil,
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/rest/api/2/search") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `
jira:
  url: "` + server.URL + `"
  pat: "test-pat"
  origin_project: "STRAT"
  teams:
    backend:
      delivery_project: "DELIV"
  doc_paths:
    - "` + tmpDir + `"
`
	if err := os.WriteFile(cfgFile, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	buf := new(bytes.Buffer)
	cmd := newRootCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--config", cfgFile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected execute success, got: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "KEY") || !strings.Contains(out, "STATUS") || !strings.Contains(out, "ASSIGNEE") || !strings.Contains(out, "SUMMARY") {
		t.Errorf("expected table headers in output, got: %s", out)
	}
	if !strings.Contains(out, "STRAT-100") || !strings.Contains(out, "Alice Smith") || !strings.Contains(out, "Implement Auth Subsystem") {
		t.Errorf("expected ticket STRAT-100 in output, got: %s", out)
	}
	if !strings.Contains(out, "STRAT-101") || !strings.Contains(out, "Unassigned") || !strings.Contains(out, "Database Sharding Plan") {
		t.Errorf("expected ticket STRAT-101 with Unassigned in output, got: %s", out)
	}
}

func TestRootCmd_NoArgs_EmptyTickets(t *testing.T) {
	mockResponse := map[string]interface{}{
		"startAt":    0,
		"maxResults": 50,
		"total":      0,
		"issues":     []map[string]interface{}{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `
jira:
  url: "` + server.URL + `"
  pat: "test-pat"
  origin_project: "STRAT"
  teams:
    backend:
      delivery_project: "DELIV"
  doc_paths:
    - "` + tmpDir + `"
`
	if err := os.WriteFile(cfgFile, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	buf := new(bytes.Buffer)
	cmd := newRootCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--config", cfgFile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected execute success, got: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "No unresolved tickets found") {
		t.Errorf("expected empty tickets message, got: %s", out)
	}
}

func TestRootCmd_InteractiveRefine_Success(t *testing.T) {
	mockTicket := map[string]interface{}{
		"key": "STRAT-42",
		"fields": map[string]interface{}{
			"summary":     "Strategic Architecture Migration",
			"description": "Break down services into modular components",
			"status": map[string]interface{}{
				"name": "Open",
			},
			"issuetype": map[string]interface{}{
				"name": "Epic",
			},
		},
	}

	jiraServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockTicket)
	}))
	defer jiraServer.Close()

	callCount := 0
	aiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var content string
		if callCount == 0 {
			content = `[{"id":"Q1","title":"Storage Choice","options":["Postgres","Mongo"],"recommendation":"Postgres"}]`
		} else if callCount == 1 {
			content = `[]`
		} else {
			content = `{"epics":[{"id":"EPIC-1","title":"Storage Epic","tasks":[{"id":"TASK-1","title":"Setup Postgres"}]}]}`
		}
		callCount++
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": content,
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer aiServer.Close()

	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")
	docDir := filepath.Join(tmpDir, "docs")
	_ = os.MkdirAll(docDir, 0755)
	_ = os.WriteFile(filepath.Join(docDir, "CONTEXT.md"), []byte("# Architecture Context"), 0644)

	cfgFile := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `
default_ai_provider: "test-ai"
ai:
  endpoints:
    - id: "test-ai"
      provider: "openai"
      base_url: "` + aiServer.URL + `/v1"
      model: "gpt-4o"
jira:
  url: "` + jiraServer.URL + `"
  pat: "test-pat"
  origin_project: "STRAT"
  teams:
    backend:
      delivery_project: "DELIV"
  doc_paths:
    - "` + docDir + `"
`
	if err := os.WriteFile(cfgFile, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Stdin input:
	// Q1: Enter (accept recommendation: "Postgres")
	// Finalize prompt: Enter (finalize)
	inBuf := strings.NewReader("\n\n")
	outBuf := new(bytes.Buffer)

	cmd := newRootCmd()
	cmd.SetIn(inBuf)
	cmd.SetOut(outBuf)
	cmd.SetErr(outBuf)
	cmd.SetArgs([]string{"STRAT-42", "--config", cfgFile, "--session-dir", sessionDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected execute success, got: %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "Storage Choice") {
		t.Errorf("expected question title in output, got: %s", out)
	}
	if !strings.Contains(out, "finalized") {
		t.Errorf("expected finalization message in output, got: %s", out)
	}

	// Check snapshot on disk
	snapFile := filepath.Join(sessionDir, "STRAT-42.json")
	data, err := os.ReadFile(snapFile)
	if err != nil {
		t.Fatalf("failed to read snapshot file: %v", err)
	}
	if !strings.Contains(string(data), `"status": "finalized"`) {
		t.Errorf("expected snapshot to be finalized, got: %s", string(data))
	}
	if !strings.Contains(string(data), `"answer": "Postgres"`) {
		t.Errorf("expected answer Postgres in snapshot, got: %s", string(data))
	}
}

func TestRootCmd_InteractiveRefine_ResumeExisting(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")
	_ = os.MkdirAll(sessionDir, 0755)

	existingSnap := session.Snapshot{
		Version: 1,
		Key:     "STRAT-50",
		Status:  "in-progress",
		Ticket: jira.Ticket{
			Key:     "STRAT-50",
			Summary: "Resumed Strategic Ticket",
		},
		CurrentFrontier: []session.Question{
			{
				ID:             "Q1",
				Title:          "Resumed Frontier Question",
				Options:        []string{"OptA", "OptB"},
				Recommendation: "OptA",
			},
		},
	}
	snapData, _ := json.MarshalIndent(existingSnap, "", "  ")
	_ = os.WriteFile(filepath.Join(sessionDir, "STRAT-50.json"), snapData, 0644)

	resumeCallCount := 0
	aiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var content string
		if resumeCallCount == 0 {
			content = "[]"
		} else {
			content = `{"epics":[{"id":"EPIC-1","title":"Metrics Epic","tasks":[{"id":"TASK-1","title":"Setup Prometheus"}]}]}`
		}
		resumeCallCount++
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": content,
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer aiServer.Close()

	// Jira server should NOT be contacted for ticket fetch if snapshot already exists
	jiraServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected call to Jira server when snapshot exists: %s", r.URL.Path)
	}))
	defer jiraServer.Close()

	cfgFile := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `
default_ai_provider: "test-ai"
ai:
  endpoints:
    - id: "test-ai"
      provider: "openai"
      base_url: "` + aiServer.URL + `/v1"
      model: "gpt-4o"
jira:
  url: "` + jiraServer.URL + `"
  pat: "test-pat"
  origin_project: "STRAT"
  teams:
    backend:
      delivery_project: "DELIV"
  doc_paths:
    - "` + tmpDir + `"
`
	if err := os.WriteFile(cfgFile, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	inBuf := strings.NewReader("\n\n")
	outBuf := new(bytes.Buffer)

	cmd := newRootCmd()
	cmd.SetIn(inBuf)
	cmd.SetOut(outBuf)
	cmd.SetErr(outBuf)
	cmd.SetArgs([]string{"STRAT-50", "--config", cfgFile, "--session-dir", sessionDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected execute success, got: %v", err)
	}

	// Verify snapshot updated
	store := session.NewFileStore(sessionDir)
	loaded, err := store.Load("STRAT-50")
	if err != nil {
		t.Fatalf("failed to load snapshot: %v", err)
	}
	if loaded.Status != "finalized" {
		t.Errorf("expected status finalized, got: %s", loaded.Status)
	}
	if len(loaded.Rounds) != 1 {
		t.Fatalf("expected 1 round, got %d", len(loaded.Rounds))
	}
	if loaded.Rounds[0].Questions[0].Answer != "OptA" {
		t.Errorf("expected OptA answer, got: %s", loaded.Rounds[0].Questions[0].Answer)
	}
}

func TestRootCmd_Plan_Success(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")
	_ = os.MkdirAll(sessionDir, 0755)

	snap := session.Snapshot{
		Version: 1,
		Key:     "STRAT-99",
		Status:  "finalized",
		Ticket: jira.Ticket{
			Key:     "STRAT-99",
			Summary: "Enterprise SSO Rollout",
		},
		Tree: &session.DecompositionTree{
			Epics: []session.DecompositionEpic{
				{
					ID:          "EPIC-1",
					Title:       "Backend SSO Core",
					Description: "Backend service integration",
					Tasks: []session.DecompositionTask{
						{
							ID:                 "TASK-1",
							Title:              "Token validation logic",
							Description:        "Validate signatures",
							AcceptanceCriteria: []string{"Returns 200 OK"},
						},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(snap, "", "  ")
	_ = os.WriteFile(filepath.Join(sessionDir, "STRAT-99.json"), data, 0644)

	cfgFile := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `
jira:
  url: "https://jira.example.com"
  pat: "test-pat"
  origin_project: "STRAT"
  teams:
    backend:
      delivery_project: "DELIV"
      keywords: ["backend", "token"]
      issue_types:
        epic: "Epic"
        task: "Task"
        story: "Story"
  doc_paths:
    - "` + tmpDir + `"
`
	if err := os.WriteFile(cfgFile, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	planFilePath := filepath.Join(tmpDir, "plans", "strat-99-plan.md")
	outBuf := new(bytes.Buffer)

	cmd := newRootCmd()
	cmd.SetOut(outBuf)
	cmd.SetErr(outBuf)
	cmd.SetArgs([]string{"STRAT-99", "--plan", "--config", cfgFile, "--session-dir", sessionDir, "--plan-file", planFilePath})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected execute success, got: %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "# Refinement Delivery Plan: STRAT-99 - Enterprise SSO Rollout") {
		t.Errorf("expected plan header in stdout, got:\n%s", out)
	}
	if !strings.Contains(out, "DELIV") {
		t.Errorf("expected DELIV delivery project in stdout, got:\n%s", out)
	}

	// Verify plan file
	planBytes, err := os.ReadFile(planFilePath)
	if err != nil {
		t.Fatalf("failed to read plan file: %v", err)
	}
	if !strings.Contains(string(planBytes), "Refinement Delivery Plan: STRAT-99") {
		t.Errorf("plan file missing title, got:\n%s", string(planBytes))
	}

	// Verify snapshot updated on disk
	store := session.NewFileStore(sessionDir)
	loaded, err := store.Load("STRAT-99")
	if err != nil {
		t.Fatalf("failed to load snapshot: %v", err)
	}
	if loaded.Tree.Epics[0].DeliveryProject != "DELIV" {
		t.Errorf("expected routed delivery project DELIV, got %q", loaded.Tree.Epics[0].DeliveryProject)
	}
}

func TestRootCmd_Plan_NoTreeError(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")
	_ = os.MkdirAll(sessionDir, 0755)

	snap := session.Snapshot{
		Version: 1,
		Key:     "STRAT-10",
		Status:  "in-progress",
		Ticket: jira.Ticket{
			Key:     "STRAT-10",
			Summary: "Incomplete ticket",
		},
	}
	data, _ := json.MarshalIndent(snap, "", "  ")
	_ = os.WriteFile(filepath.Join(sessionDir, "STRAT-10.json"), data, 0644)

	cfgFile := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `
jira:
  url: "https://jira.example.com"
  pat: "test-pat"
  origin_project: "STRAT"
  teams:
    backend:
      delivery_project: "DELIV"
  doc_paths:
    - "` + tmpDir + `"
`
	_ = os.WriteFile(cfgFile, []byte(cfgContent), 0644)

	cmd := newRootCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"STRAT-10", "--plan", "--config", cfgFile, "--session-dir", sessionDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no tree is present, got nil")
	}
	if !strings.Contains(err.Error(), "no decomposition tree found") {
		t.Errorf("expected error mentioning no decomposition tree, got: %v", err)
	}
}

func TestRootCmd_Sync_MissingKey(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newRootCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--sync"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when key is missing with --sync, got nil")
	}
	if !strings.Contains(err.Error(), "ticket key is required") {
		t.Errorf("expected error mentioning ticket key is required, got: %v", err)
	}
}

func TestRootCmd_Sync_SnapshotNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `
jira:
  url: "https://jira.example.com"
  pat: "test-pat"
  origin_project: "STRAT"
  teams:
    backend:
      delivery_project: "DELIV"
  doc_paths:
    - "` + tmpDir + `"
`
	_ = os.WriteFile(cfgFile, []byte(cfgContent), 0644)

	cmd := newRootCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"STRAT-404", "--sync", "--config", cfgFile, "--session-dir", filepath.Join(tmpDir, "sessions")})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing snapshot, got nil")
	}
	if !strings.Contains(err.Error(), "failed to load session snapshot") {
		t.Errorf("expected snapshot load error, got: %v", err)
	}
}

func TestRootCmd_Sync_UnroutedTree_Error(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")
	_ = os.MkdirAll(sessionDir, 0755)

	snap := session.Snapshot{
		Version: 1,
		Key:     "STRAT-77",
		Status:  "in-progress",
		Ticket: jira.Ticket{
			Key:     "STRAT-77",
			Summary: "Unrouted Tree Ticket",
		},
		Tree: &session.DecompositionTree{
			Epics: []session.DecompositionEpic{
				{
					ID:    "EPIC-1",
					Title: "Unrouted Epic",
				},
			},
		},
	}
	snapData, _ := json.MarshalIndent(snap, "", "  ")
	_ = os.WriteFile(filepath.Join(sessionDir, "STRAT-77.json"), snapData, 0644)

	cfgFile := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `
jira:
  url: "https://jira.example.com"
  pat: "test-pat"
  origin_project: "STRAT"
  teams:
    backend:
      delivery_project: "DELIV"
  doc_paths:
    - "` + tmpDir + `"
`
	_ = os.WriteFile(cfgFile, []byte(cfgContent), 0644)

	cmd := newRootCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"STRAT-77", "--sync", "--yes", "--config", cfgFile, "--session-dir", sessionDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error on unrouted tree, got nil")
	}
	if !strings.Contains(err.Error(), "not routed to a delivery project") {
		t.Errorf("expected routing verification error, got: %v", err)
	}
}

func TestRootCmd_Sync_UserAborted(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")
	_ = os.MkdirAll(sessionDir, 0755)

	snap := session.Snapshot{
		Version: 1,
		Key:     "STRAT-88",
		Status:  "finalized",
		Ticket: jira.Ticket{
			Key:     "STRAT-88",
			Summary: "Aborted Sync Ticket",
		},
		Tree: &session.DecompositionTree{
			Epics: []session.DecompositionEpic{
				{
					ID:              "EPIC-1",
					Title:           "Backend Architecture",
					DeliveryProject: "DELIV",
					Tasks: []session.DecompositionTask{
						{
							ID:              "TASK-1",
							Title:           "Database Layer",
							DeliveryProject: "DELIV",
						},
					},
				},
			},
		},
	}
	snapData, _ := json.MarshalIndent(snap, "", "  ")
	_ = os.WriteFile(filepath.Join(sessionDir, "STRAT-88.json"), snapData, 0644)

	cfgFile := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `
jira:
  url: "https://jira.example.com"
  pat: "test-pat"
  origin_project: "STRAT"
  teams:
    backend:
      delivery_project: "DELIV"
  doc_paths:
    - "` + tmpDir + `"
`
	_ = os.WriteFile(cfgFile, []byte(cfgContent), 0644)

	inBuf := strings.NewReader("n\n")
	outBuf := new(bytes.Buffer)

	cmd := newRootCmd()
	cmd.SetIn(inBuf)
	cmd.SetOut(outBuf)
	cmd.SetErr(outBuf)
	cmd.SetArgs([]string{"STRAT-88", "--sync", "--config", cfgFile, "--session-dir", sessionDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error on clean abort, got: %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "Sync aborted by user") {
		t.Errorf("expected 'Sync aborted by user' in stdout, got:\n%s", out)
	}
}

func TestRootCmd_Sync_Success(t *testing.T) {
	issueSeq := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && r.URL.Path == "/rest/api/2/issue" {
			issueSeq++
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			key := fmt.Sprintf("DELIV-%d", issueSeq)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":   fmt.Sprintf("%d", issueSeq),
				"key":  key,
				"self": fmt.Sprintf("https://jira.example.com/rest/api/2/issue/%d", issueSeq),
			})
			return
		}
		if r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/rest/api/2/issueLink" {
			w.WriteHeader(http.StatusCreated)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")
	_ = os.MkdirAll(sessionDir, 0755)

	snap := session.Snapshot{
		Version: 1,
		Key:     "STRAT-500",
		Status:  "finalized",
		Ticket: jira.Ticket{
			Key:     "STRAT-500",
			Summary: "Full Sync Architecture",
		},
		Tree: &session.DecompositionTree{
			Epics: []session.DecompositionEpic{
				{
					ID:              "EPIC-1",
					Title:           "Identity Management",
					DeliveryProject: "DELIV",
					Type:            "Epic",
					Tasks: []session.DecompositionTask{
						{
							ID:              "TASK-2",
							Title:           "OAuth Flow",
							DeliveryProject: "DELIV",
							Type:            "Story",
							DependsOn:       []string{"TASK-1"},
						},
						{
							ID:              "TASK-1",
							Title:           "JWT Signing Service",
							DeliveryProject: "DELIV",
							Type:            "Task",
						},
					},
				},
			},
		},
	}
	snapData, _ := json.MarshalIndent(snap, "", "  ")
	_ = os.WriteFile(filepath.Join(sessionDir, "STRAT-500.json"), snapData, 0644)

	cfgFile := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `
jira:
  url: "` + server.URL + `"
  pat: "test-pat"
  origin_project: "STRAT"
  teams:
    backend:
      delivery_project: "DELIV"
  doc_paths:
    - "` + tmpDir + `"
`
	_ = os.WriteFile(cfgFile, []byte(cfgContent), 0644)

	outBuf := new(bytes.Buffer)
	cmd := newRootCmd()
	cmd.SetOut(outBuf)
	cmd.SetErr(outBuf)
	cmd.SetArgs([]string{"STRAT-500", "--sync", "--yes", "--config", cfgFile, "--session-dir", sessionDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected execute success, got: %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "Synchronized Jira Issues for STRAT-500") {
		t.Errorf("expected summary table header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "DELIV-1") || !strings.Contains(out, "DELIV-2") || !strings.Contains(out, "DELIV-3") {
		t.Errorf("expected created keys DELIV-1, DELIV-2, DELIV-3 in output, got:\n%s", out)
	}

	// Verify snapshot updated on disk
	store := session.NewFileStore(sessionDir)
	loaded, err := store.Load("STRAT-500")
	if err != nil {
		t.Fatalf("failed to load updated snapshot: %v", err)
	}
	if loaded.Status != session.StatusSynced {
		t.Errorf("expected status %s, got %s", session.StatusSynced, loaded.Status)
	}
	if loaded.Tree.Epics[0].Key == "" {
		t.Errorf("expected epic key in snapshot")
	}
	if loaded.Tree.Epics[0].Tasks[0].Key == "" || loaded.Tree.Epics[0].Tasks[1].Key == "" {
		t.Errorf("expected task keys in snapshot")
	}
}

func TestRootCmd_TUI_Flag_Success(t *testing.T) {
	oldRunner := runTUIProgram
	defer func() { runTUIProgram = oldRunner }()

	tuiRan := false
	runTUIProgram = func(m tea.Model) error {
		tuiRan = true
		return nil
	}

	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `
jira:
  url: "https://jira.example.com"
  pat: "test-pat"
  origin_project: "STRAT"
  teams:
    backend:
      delivery_project: "DELIV"
  doc_paths:
    - "` + tmpDir + `"
`
	_ = os.WriteFile(cfgFile, []byte(cfgContent), 0644)

	cmd := newRootCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--tui", "--config", cfgFile, "--session-dir", filepath.Join(tmpDir, "sessions")})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected execute success, got: %v", err)
	}
	if !tuiRan {
		t.Fatalf("expected runTUIProgram to have been called")
	}
}

func TestRootCmd_TUI_Subcommand_WithKey(t *testing.T) {
	oldRunner := runTUIProgram
	defer func() { runTUIProgram = oldRunner }()

	tuiRan := false
	runTUIProgram = func(m tea.Model) error {
		tuiRan = true
		return nil
	}

	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `
jira:
  url: "https://jira.example.com"
  pat: "test-pat"
  origin_project: "STRAT"
  teams:
    backend:
      delivery_project: "DELIV"
  doc_paths:
    - "` + tmpDir + `"
`
	_ = os.WriteFile(cfgFile, []byte(cfgContent), 0644)

	cmd := newRootCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"tui", "STRAT-42", "--config", cfgFile, "--session-dir", filepath.Join(tmpDir, "sessions")})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected execute success, got: %v", err)
	}
	if !tuiRan {
		t.Fatalf("expected runTUIProgram to have been called")
	}
}





