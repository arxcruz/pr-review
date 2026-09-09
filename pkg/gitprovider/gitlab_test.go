package gitprovider

import (
	"testing"

	"github.com/arxcruz/pr-review/pkg/config"
)

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"gitlab.example.com", "https://gitlab.example.com"},
		{"gitlab.example.com/", "https://gitlab.example.com"},
		{"https://gitlab.example.com", "https://gitlab.example.com"},
		{"https://gitlab.example.com/", "https://gitlab.example.com"},
		{"http://localhost:8080/", "http://localhost:8080"},
	}

	for _, tt := range tests {
		got := normalizeBaseURL(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeBaseURL(%q) = %q; expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestGitLabGetProjectPath(t *testing.T) {
	gp := &GitLabProvider{}

	t.Run("explicit project_path", func(t *testing.T) {
		proj := config.ProjectConfig{
			ID:          "my-proj",
			ProjectPath: "group/subgroup/repo",
			Owner:       "ignored",
			Repo:        "ignored",
		}
		path, err := gp.getProjectPath(proj)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "group/subgroup/repo" {
			t.Errorf("expected group/subgroup/repo, got %s", path)
		}
	})

	t.Run("fallback to owner/repo", func(t *testing.T) {
		proj := config.ProjectConfig{
			ID:    "my-proj-jobs",
			Owner: "my-group",
			Repo:  "my-repo",
		}
		path, err := gp.getProjectPath(proj)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "my-group/my-repo" {
			t.Errorf("expected my-group/my-repo, got %s", path)
		}
	})

	t.Run("fallback to repo only", func(t *testing.T) {
		proj := config.ProjectConfig{
			ID:   "my-proj",
			Repo: "single-repo",
		}
		path, err := gp.getProjectPath(proj)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "single-repo" {
			t.Errorf("expected single-repo, got %s", path)
		}
	})

	t.Run("fallback to slash in ID", func(t *testing.T) {
		proj := config.ProjectConfig{
			ID: "org/project",
		}
		path, err := gp.getProjectPath(proj)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "org/project" {
			t.Errorf("expected org/project, got %s", path)
		}
	})

	t.Run("error when no path info provided", func(t *testing.T) {
		proj := config.ProjectConfig{
			ID: "invalid-proj",
		}
		_, err := gp.getProjectPath(proj)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
