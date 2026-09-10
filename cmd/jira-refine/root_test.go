package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arxcruz/pr-review/pkg/jira"
	"github.com/arxcruz/pr-review/pkg/session"
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
			Summary: "Resumed Initiative",
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



