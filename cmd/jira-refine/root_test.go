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
