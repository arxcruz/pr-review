package review

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arxcruz/pr-review/pkg/config"
)

func TestReviewFileName(t *testing.T) {
	tests := []struct {
		name     string
		proj     config.ProjectConfig
		prNumber int
		expected string
	}{
		{
			name: "GitHub project with repo",
			proj: config.ProjectConfig{
				ID:       "backend",
				Provider: "github",
				Owner:    "myorg",
				Repo:     "backend-service",
			},
			prNumber: 42,
			expected: "backend-service-42.md",
		},
		{
			name: "GitLab project with project_path",
			proj: config.ProjectConfig{
				ID:          "frontend",
				Provider:    "gitlab",
				ProjectPath: "mygroup/subgroup/frontend-ui",
			},
			prNumber: 101,
			expected: "frontend-ui-101.md",
		},
		{
			name: "Gerrit project with slashed repo",
			proj: config.ProjectConfig{
				ID:       "nova",
				Provider: "gerrit",
				Repo:     "openstack/nova",
			},
			prNumber: 887,
			expected: "openstack-nova-887.md",
		},
		{
			name: "Fallback to project ID",
			proj: config.ProjectConfig{
				ID: "custom-proj",
			},
			prNumber: 7,
			expected: "custom-proj-7.md",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := ReviewFileName(tc.proj, tc.prNumber)
			if actual != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, actual)
			}
		})
	}
}

func TestReviewFilePathWithReviewsDir(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ReviewsDir = "/custom/path/reviews"
	svc := NewService(cfg)

	proj := config.ProjectConfig{
		ID:    "backend",
		Owner: "myorg",
		Repo:  "backend",
	}

	expected := "/custom/path/reviews/backend-42.md"
	actual := svc.ReviewFilePath(proj, 42)
	if actual != expected {
		t.Errorf("expected %q, got %q", expected, actual)
	}
}

func TestGetCachedReview(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.ReviewsDir = tmpDir
	svc := NewService(cfg)

	proj := config.ProjectConfig{
		ID:    "backend",
		Owner: "myorg",
		Repo:  "backend",
	}

	// 1. Initially does not exist
	content, path, exists := svc.GetCachedReview(proj, 42)
	if exists {
		t.Errorf("expected review not to exist initially")
	}
	if path != filepath.Join(tmpDir, "backend-42.md") {
		t.Errorf("expected path %s, got %s", filepath.Join(tmpDir, "backend-42.md"), path)
	}

	// 2. Create the file and verify found
	expectedContent := "### AI Review for PR 42"
	if err := os.WriteFile(path, []byte(expectedContent), 0644); err != nil {
		t.Fatalf("failed to write test review: %v", err)
	}

	content, foundPath, exists := svc.GetCachedReview(proj, 42)
	if !exists {
		t.Fatalf("expected review to exist after creating file")
	}
	if content != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, content)
	}
	if foundPath != path {
		t.Errorf("expected foundPath %s, got %s", path, foundPath)
	}
}

func TestSanitizeForFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"qwen2.5-coder:latest", "qwen2-5-coder-latest"},
		{"claude-3-7-sonnet@20250219", "claude-3-7-sonnet20250219"},
		{"gemini-2.5-pro", "gemini-2-5-pro"},
		{"deepseek-r1:70b_q4", "deepseek-r1-70b-q4"},
		{"--test---model--", "test-model"},
	}

	for _, tc := range tests {
		actual := SanitizeForFilename(tc.input)
		if actual != tc.expected {
			t.Errorf("SanitizeForFilename(%q) = %q, want %q", tc.input, actual, tc.expected)
		}
	}
}

func TestReviewFileNameWithAI(t *testing.T) {
	proj := config.ProjectConfig{
		ID:    "pr-review",
		Owner: "arxcruz",
		Repo:  "pr-review",
	}

	res := ReviewFileNameWithAI(proj, 5, "ollama", "qwen2.5-coder:latest")
	expected := "pr-review-5-ollama-qwen2-5-coder-latest.md"
	if res != expected {
		t.Errorf("expected %q, got %q", expected, res)
	}

	res2 := ReviewFileNameWithAI(proj, 5, "anthropic", "claude-3-7-sonnet")
	expected2 := "pr-review-5-anthropic-claude-3-7-sonnet.md"
	if res2 != expected2 {
		t.Errorf("expected %q, got %q", expected2, res2)
	}
}

func TestFormatAndParseReviewMarkdown(t *testing.T) {
	meta := ReviewMetadata{
		Provider:  "anthropic",
		Model:     "claude-3-7-sonnet",
		Project:   "pr-review",
		PR:        42,
	}
	bodyContent := "### Summary\n- Looks good to me!\n- Clean implementation."

	formatted := FormatReviewMarkdown(meta, bodyContent)

	parsedMeta, parsedBody := ParseReviewMarkdown(formatted)

	if parsedMeta.Provider != meta.Provider {
		t.Errorf("expected provider %q, got %q", meta.Provider, parsedMeta.Provider)
	}
	if parsedMeta.Model != meta.Model {
		t.Errorf("expected model %q, got %q", meta.Model, parsedMeta.Model)
	}
	if parsedMeta.Project != meta.Project {
		t.Errorf("expected project %q, got %q", meta.Project, parsedMeta.Project)
	}
	if parsedMeta.PR != meta.PR {
		t.Errorf("expected PR %d, got %d", meta.PR, parsedMeta.PR)
	}
	if parsedBody != bodyContent {
		t.Errorf("expected body %q, got %q", bodyContent, parsedBody)
	}

	// Test fallback with plain markdown without frontmatter
	plain := "Just raw review without frontmatter"
	plainMeta, plainBody := ParseReviewMarkdown(plain)
	if plainMeta.Provider != "" {
		t.Errorf("expected empty provider for plain review, got %q", plainMeta.Provider)
	}
	if plainBody != plain {
		t.Errorf("expected raw content returned as body, got %q", plainBody)
	}
}

func TestListReviewsForPR(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.ReviewsDir = tmpDir
	svc := NewService(cfg)

	proj := config.ProjectConfig{
		ID:    "myproject",
		Owner: "myorg",
		Repo:  "myrepo",
	}

	// Create 3 reviews for PR 12
	rev1 := FormatReviewMarkdown(ReviewMetadata{Provider: "ollama", Model: "qwen2.5-coder"}, "Review 1 from Ollama")
	rev2 := FormatReviewMarkdown(ReviewMetadata{Provider: "anthropic", Model: "claude-3-7-sonnet"}, "Review 2 from Claude")
	revLegacy := "Legacy review without frontmatter"

	file1 := filepath.Join(tmpDir, "myrepo-12-ollama-qwen2-5-coder.md")
	file2 := filepath.Join(tmpDir, "myrepo-12-anthropic-claude-3-7-sonnet.md")
	fileLegacy := filepath.Join(tmpDir, "myrepo-12.md")
	otherPRFile := filepath.Join(tmpDir, "myrepo-99-ollama-model.md")

	if err := os.WriteFile(file1, []byte(rev1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte(rev2), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileLegacy, []byte(revLegacy), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherPRFile, []byte("Other PR"), 0644); err != nil {
		t.Fatal(err)
	}

	reviews := svc.ListReviewsForPR(proj, 12)
	if len(reviews) != 3 {
		t.Fatalf("expected 3 reviews for PR 12, got %d", len(reviews))
	}

	foundProviders := make(map[string]bool)
	for _, r := range reviews {
		if r.Provider != "" {
			foundProviders[r.Provider] = true
		}
	}

	if !foundProviders["ollama"] {
		t.Errorf("expected to find ollama review")
	}
	if !foundProviders["anthropic"] {
		t.Errorf("expected to find anthropic review")
	}
}

func TestDeleteReviewFunctions(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.ReviewsDir = tmpDir
	svc := NewService(cfg)

	proj1 := config.ProjectConfig{ID: "proj1", Repo: "repo1"}
	proj2 := config.ProjectConfig{ID: "proj2", Repo: "repo2"}
	_ = proj2

	// Create reviews
	f1 := filepath.Join(tmpDir, "repo1-10-ollama-model.md")
	f2 := filepath.Join(tmpDir, "repo1-10-anthropic-model.md")
	f3 := filepath.Join(tmpDir, "repo1-20-ollama-model.md")
	f4 := filepath.Join(tmpDir, "repo2-5-gemini-model.md")

	for _, f := range []string{f1, f2, f3, f4} {
		if err := os.WriteFile(f, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// 1. Test DeleteReviewFile
	if err := svc.DeleteReviewFile(f1); err != nil {
		t.Errorf("failed to delete f1: %v", err)
	}
	if _, err := os.Stat(f1); !os.IsNotExist(err) {
		t.Errorf("expected f1 to be deleted")
	}

	// 2. Test DeleteReviewsForPR on proj1 PR 10 (should delete f2)
	deleted, err := svc.DeleteReviewsForPR(proj1, 10)
	if err != nil {
		t.Errorf("DeleteReviewsForPR error: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted review for PR 10, got %d", deleted)
	}
	if _, err := os.Stat(f2); !os.IsNotExist(err) {
		t.Errorf("expected f2 to be deleted")
	}

	// 3. Test DeleteAllReviews for specific project proj1 (should delete f3, keep f4)
	deletedProj1, err := svc.DeleteAllReviews(&proj1)
	if err != nil {
		t.Errorf("DeleteAllReviews for proj1 error: %v", err)
	}
	if deletedProj1 != 1 {
		t.Errorf("expected 1 deleted review for proj1, got %d", deletedProj1)
	}
	if _, err := os.Stat(f3); !os.IsNotExist(err) {
		t.Errorf("expected f3 to be deleted")
	}
	if _, err := os.Stat(f4); os.IsNotExist(err) {
		t.Errorf("expected f4 to remain")
	}

	// 4. Test DeleteAllReviews across all projects (should delete f4)
	deletedAll, err := svc.DeleteAllReviews(nil)
	if err != nil {
		t.Errorf("DeleteAllReviews error: %v", err)
	}
	if deletedAll != 1 {
		t.Errorf("expected 1 deleted review, got %d", deletedAll)
	}
	if _, err := os.Stat(f4); !os.IsNotExist(err) {
		t.Errorf("expected f4 to be deleted")
	}
}
