package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJiraConfigLoad_Full(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
jira:
  url: "https://jira.example.com"
  token: "my-jira-token"
  origin_project: "ORIGIN"
  teams:
    backend:
      delivery_project: "BACK"
      issue_types:
        epic: "Feature"
        task: "Sub-task"
        story: "User Story"
    frontend:
      delivery_project: "FRONT"
  doc_paths:
    - "~/repos/backend/docs"
    - "~/repos/frontend/docs"
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, _, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Jira.URL != "https://jira.example.com" {
		t.Errorf("expected URL https://jira.example.com, got %q", cfg.Jira.URL)
	}
	if cfg.Jira.Token != "my-jira-token" {
		t.Errorf("expected Token my-jira-token, got %q", cfg.Jira.Token)
	}
	if cfg.Jira.GetAuthToken() != "my-jira-token" {
		t.Errorf("expected GetAuthToken() my-jira-token, got %q", cfg.Jira.GetAuthToken())
	}
	if cfg.Jira.OriginProject != "ORIGIN" {
		t.Errorf("expected OriginProject ORIGIN, got %q", cfg.Jira.OriginProject)
	}
	if len(cfg.Jira.Teams) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(cfg.Jira.Teams))
	}

	backend, ok := cfg.Jira.Teams["backend"]
	if !ok {
		t.Fatalf("expected backend team in teams map")
	}
	if backend.DeliveryProject != "BACK" {
		t.Errorf("expected DeliveryProject BACK, got %q", backend.DeliveryProject)
	}
	if backend.IssueTypes.Epic != "Feature" {
		t.Errorf("expected Epic Feature, got %q", backend.IssueTypes.Epic)
	}
	if backend.IssueTypes.Task != "Sub-task" {
		t.Errorf("expected Task Sub-task, got %q", backend.IssueTypes.Task)
	}
	if backend.IssueTypes.Story != "User Story" {
		t.Errorf("expected Story User Story, got %q", backend.IssueTypes.Story)
	}

	frontend, ok := cfg.Jira.Teams["frontend"]
	if !ok {
		t.Fatalf("expected frontend team in teams map")
	}
	if frontend.DeliveryProject != "FRONT" {
		t.Errorf("expected DeliveryProject FRONT, got %q", frontend.DeliveryProject)
	}
	// Defaults applied for missing issue types
	if frontend.IssueTypes.Epic != "Epic" {
		t.Errorf("expected default Epic Epic, got %q", frontend.IssueTypes.Epic)
	}
	if frontend.IssueTypes.Task != "Task" {
		t.Errorf("expected default Task Task, got %q", frontend.IssueTypes.Task)
	}
	if frontend.IssueTypes.Story != "Story" {
		t.Errorf("expected default Story Story, got %q", frontend.IssueTypes.Story)
	}

	home, _ := os.UserHomeDir()
	expectedDoc0 := filepath.Join(home, "repos/backend/docs")
	expectedDoc1 := filepath.Join(home, "repos/frontend/docs")
	if len(cfg.Jira.DocPaths) != 2 {
		t.Fatalf("expected 2 doc paths, got %d", len(cfg.Jira.DocPaths))
	}
	if cfg.Jira.DocPaths[0] != expectedDoc0 {
		t.Errorf("expected DocPaths[0] %q, got %q", expectedDoc0, cfg.Jira.DocPaths[0])
	}
	if cfg.Jira.DocPaths[1] != expectedDoc1 {
		t.Errorf("expected DocPaths[1] %q, got %q", expectedDoc1, cfg.Jira.DocPaths[1])
	}

	if err := cfg.Jira.Validate(); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestJiraConfig_PATAuth(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
jira:
  url: "https://jira.enterprise.org"
  pat: "personal-access-token"
  origin_project: "STRAT"
  teams:
    infra:
      delivery_project: "OPS"
  doc_paths:
    - "/docs"
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, _, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Jira.PAT != "personal-access-token" {
		t.Errorf("expected PAT personal-access-token, got %q", cfg.Jira.PAT)
	}
	if cfg.Jira.GetAuthToken() != "personal-access-token" {
		t.Errorf("expected GetAuthToken() personal-access-token, got %q", cfg.Jira.GetAuthToken())
	}
	if err := cfg.Jira.Validate(); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestJiraConfig_EnvVars(t *testing.T) {
	t.Setenv("JIRA_URL", "https://jira.env.com")
	t.Setenv("JIRA_TOKEN", "token-from-env")
	t.Setenv("JIRA_ORIGIN_PROJECT", "ENVORIGIN")

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
jira:
  teams:
    ops:
      delivery_project: "OPS"
  doc_paths:
    - "/docs"
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, _, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Jira.URL != "https://jira.env.com" {
		t.Errorf("expected URL from env, got %q", cfg.Jira.URL)
	}
	if cfg.Jira.Token != "token-from-env" {
		t.Errorf("expected Token from env, got %q", cfg.Jira.Token)
	}
	if cfg.Jira.OriginProject != "ENVORIGIN" {
		t.Errorf("expected OriginProject from env, got %q", cfg.Jira.OriginProject)
	}

	if err := cfg.Jira.Validate(); err != nil {
		t.Fatalf("expected valid config via env vars, got: %v", err)
	}
}

func TestJiraConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		cfg         JiraConfig
		expectedErr string
	}{
		{
			name: "missing URL",
			cfg: JiraConfig{
				Token:         "tok",
				OriginProject: "STRAT",
				Teams: map[string]JiraTeamConfig{
					"ops": {DeliveryProject: "OPS"},
				},
				DocPaths: []string{"/docs"},
			},
			expectedErr: "jira url is required",
		},
		{
			name: "invalid URL scheme",
			cfg: JiraConfig{
				URL:           "ftp://jira.example.com",
				Token:         "tok",
				OriginProject: "STRAT",
				Teams: map[string]JiraTeamConfig{
					"ops": {DeliveryProject: "OPS"},
				},
				DocPaths: []string{"/docs"},
			},
			expectedErr: "jira url must be a valid http or https url",
		},
		{
			name: "missing credentials",
			cfg: JiraConfig{
				URL:           "https://jira.example.com",
				OriginProject: "STRAT",
				Teams: map[string]JiraTeamConfig{
					"ops": {DeliveryProject: "OPS"},
				},
				DocPaths: []string{"/docs"},
			},
			expectedErr: "jira auth credentials (token or pat) are required",
		},
		{
			name: "missing origin project",
			cfg: JiraConfig{
				URL:   "https://jira.example.com",
				Token: "tok",
				Teams: map[string]JiraTeamConfig{
					"ops": {DeliveryProject: "OPS"},
				},
				DocPaths: []string{"/docs"},
			},
			expectedErr: "jira origin project is required",
		},
		{
			name: "missing teams",
			cfg: JiraConfig{
				URL:           "https://jira.example.com",
				Token:         "tok",
				OriginProject: "STRAT",
				DocPaths:      []string{"/docs"},
			},
			expectedErr: "at least one team delivery project mapping is required",
		},
		{
			name: "team missing delivery project",
			cfg: JiraConfig{
				URL:           "https://jira.example.com",
				Token:         "tok",
				OriginProject: "STRAT",
				Teams: map[string]JiraTeamConfig{
					"backend": {},
				},
				DocPaths: []string{"/docs"},
			},
			expectedErr: `jira team "backend": delivery_project is required`,
		},
		{
			name: "missing doc paths",
			cfg: JiraConfig{
				URL:           "https://jira.example.com",
				Token:         "tok",
				OriginProject: "STRAT",
				Teams: map[string]JiraTeamConfig{
					"backend": {DeliveryProject: "BACK"},
				},
			},
			expectedErr: "at least one multi-repo documentation path is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.expectedErr)
			}
			if !strings.Contains(err.Error(), tc.expectedErr) {
				t.Errorf("expected error containing %q, got %q", tc.expectedErr, err.Error())
			}
		})
	}
}

func TestJiraConfig_HelpersAndDefaults(t *testing.T) {
	var empty JiraConfig
	if empty.IsConfigured() {
		t.Errorf("expected empty JiraConfig to not be configured")
	}

	defaultTypes := DefaultJiraIssueTypes()
	if defaultTypes.Epic != "Epic" || defaultTypes.Task != "Task" || defaultTypes.Story != "Story" {
		t.Errorf("unexpected DefaultJiraIssueTypes: %+v", defaultTypes)
	}
}
