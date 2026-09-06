package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoadAndDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
default_ai_provider: anthropic
ai:
  anthropic:
    model: claude-3-7-sonnet-20250219
    api_key: test-key-123
git:
  github:
    token: gh-token-456
projects:
  - id: test-repo
    provider: github
    owner: arxcruz
    repo: pr-review
    description: "PR review TUI and CLI tool written in Go"
    default_filters:
      labels: ["backend", "needs-review"]
      authors: ["arxcruz"]
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, path, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if path != cfgPath {
		t.Errorf("expected path %s, got %s", cfgPath, path)
	}
	if cfg.DefaultAIProvider != "anthropic" {
		t.Errorf("expected default provider anthropic, got %s", cfg.DefaultAIProvider)
	}
	if cfg.AI.Anthropic.APIKey != "test-key-123" {
		t.Errorf("expected api key test-key-123, got %s", cfg.AI.Anthropic.APIKey)
	}
	if len(cfg.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(cfg.Projects))
	}
	if cfg.Projects[0].Owner != "arxcruz" || cfg.Projects[0].Repo != "pr-review" {
		t.Errorf("unexpected project owner/repo: %s/%s", cfg.Projects[0].Owner, cfg.Projects[0].Repo)
	}
	if cfg.Projects[0].Description != "PR review TUI and CLI tool written in Go" {
		t.Errorf("expected description %q, got %q", "PR review TUI and CLI tool written in Go", cfg.Projects[0].Description)
	}
}

func TestEnvVarExpansion(t *testing.T) {
	t.Setenv("TEST_PR_TOKEN", "secret-token-val")
	input := []byte("token: ${TEST_PR_TOKEN}\nkey: $TEST_PR_TOKEN")
	output := string(ExpandEnvVars(input))
	expected := "token: secret-token-val\nkey: secret-token-val"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

func TestExpandPath(t *testing.T) {
	t.Setenv("MY_DIR", "subfolder")
	home, _ := os.UserHomeDir()

	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"reviews", "reviews"},
		{"${MY_DIR}/reviews", "subfolder/reviews"},
		{"~/reviews", filepath.Join(home, "reviews")},
	}

	for _, tc := range tests {
		actual := ExpandPath(tc.input)
		if actual != tc.expected {
			t.Errorf("ExpandPath(%q) = %q; expected %q", tc.input, actual, tc.expected)
		}
	}
}

func TestReviewsDirLoading(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
default_ai_provider: ollama
reviews_dir: "~/custom-reviews"
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, _, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	home, _ := os.UserHomeDir()
	expectedDir := filepath.Join(home, "custom-reviews")
	if cfg.ReviewsDir != expectedDir {
		t.Errorf("expected ReviewsDir %q, got %q", expectedDir, cfg.ReviewsDir)
	}
}

func TestKeybindingsLoading(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
default_ai_provider: ollama
keybindings:
  up: "e"
  down: ["n", "down"]
  run_review: "x"
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, _, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.Keybindings.Up) != 1 || cfg.Keybindings.Up[0] != "e" {
		t.Errorf("expected Keybindings.Up to be [\"e\"], got %v", cfg.Keybindings.Up)
	}

	if len(cfg.Keybindings.Down) != 2 || cfg.Keybindings.Down[0] != "n" || cfg.Keybindings.Down[1] != "down" {
		t.Errorf("expected Keybindings.Down to be [\"n\", \"down\"], got %v", cfg.Keybindings.Down)
	}

	if len(cfg.Keybindings.RunReview) != 1 || cfg.Keybindings.RunReview[0] != "x" {
		t.Errorf("expected Keybindings.RunReview to be [\"x\"], got %v", cfg.Keybindings.RunReview)
	}

	// Non-overridden keybinding should fall back to default
	if len(cfg.Keybindings.Quit) == 0 || cfg.Keybindings.Quit[0] != "q" {
		t.Errorf("expected default Keybindings.Quit fallback, got %v", cfg.Keybindings.Quit)
	}
}

func TestLoadReviewGuidelinesFromDefaultFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	reviewPath := filepath.Join(tmpDir, "review.md")

	customGuidelines := "# Custom Review Guidelines\nCheck for security and simplicity."
	if err := os.WriteFile(reviewPath, []byte(customGuidelines), 0644); err != nil {
		t.Fatalf("failed to write review.md: %v", err)
	}

	yamlContent := `default_ai_provider: ollama`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write config.yaml: %v", err)
	}

	cfg, _, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.ReviewGuidelines != customGuidelines {
		t.Errorf("expected guidelines %q, got %q", customGuidelines, cfg.ReviewGuidelines)
	}
}

func TestLoadReviewGuidelinesFromCustomFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	customPath := filepath.Join(tmpDir, "special-guidelines.md")

	customGuidelines := "# Special Team Guidelines\nFocus on test coverage."
	if err := os.WriteFile(customPath, []byte(customGuidelines), 0644); err != nil {
		t.Fatalf("failed to write special-guidelines.md: %v", err)
	}

	yamlContent := `
default_ai_provider: ollama
review_file: special-guidelines.md
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write config.yaml: %v", err)
	}

	cfg, _, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.ReviewGuidelines != customGuidelines {
		t.Errorf("expected guidelines %q, got %q", customGuidelines, cfg.ReviewGuidelines)
	}
}

func TestSaveConfigDoesNotIncludeReviewGuidelines(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "saved_config.yaml")

	cfg := DefaultConfig()
	if err := SaveConfig(cfg, targetPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read saved config: %v", err)
	}

	if string(data) == "" {
		t.Fatal("expected non-empty saved config")
	}

	if contains(string(data), "review_guidelines") {
		t.Errorf("saved config should not contain review_guidelines, got:\n%s", string(data))
	}
}

func contains(s, substr string) bool {
	return filepath.Clean(s) != "" && (len(s) >= len(substr) && (s == substr || (len(s) > 0 && len(substr) > 0 && searchSubstr(s, substr))))
}

func searchSubstr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGetAllAITargetsMultiModelsAndEndpoints(t *testing.T) {
	cfg := DefaultConfig()
	// Add multiple models under ollama
	cfg.AI.Ollama.Models = []string{"qwen2.5-coder:latest", "deepseek-coder-v2:16b", "llama3.3:70b"}
	// Add custom multi-endpoints
	cfg.AI.Endpoints = []AIEndpointConfig{
		{
			ID:       "gpu-vllm",
			Name:     "GPU Cluster",
			Provider: "vllm",
			BaseURL:  "http://192.168.1.100:8000/v1",
			APIKey:   "secret-token",
			Models:   []string{"deepseek-ai/DeepSeek-Coder-V2-Instruct", "Qwen/Qwen2.5-Coder-32B-Instruct"},
		},
		{
			ID:       "local-vllm",
			Name:     "Local vLLM",
			Provider: "vllm",
			BaseURL:  "http://localhost:8000/v1",
			Model:    "Qwen/Qwen2.5-Coder-7B-Instruct",
		},
	}

	targets := GetAllAITargets(cfg)
	if len(targets) == 0 {
		t.Fatal("expected targets, got empty slice")
	}

	// Verify custom endpoints are present
	foundGPUVllm := 0
	foundLocalVllm := false
	foundOllamaModels := 0

	for _, target := range targets {
		if target.EndpointID == "gpu-vllm" {
			foundGPUVllm++
			if target.BaseURL != "http://192.168.1.100:8000/v1" {
				t.Errorf("unexpected BaseURL: %s", target.BaseURL)
			}
			if target.APIKey != "secret-token" {
				t.Errorf("unexpected APIKey: %s", target.APIKey)
			}
		}
		if target.ID == "local-vllm" && target.Model == "Qwen/Qwen2.5-Coder-7B-Instruct" {
			foundLocalVllm = true
		}
		if target.Provider == "ollama" {
			foundOllamaModels++
		}
	}

	if foundGPUVllm != 2 {
		t.Errorf("expected 2 targets for gpu-vllm, got %d", foundGPUVllm)
	}
	if !foundLocalVllm {
		t.Errorf("expected local-vllm target to be found")
	}
	if foundOllamaModels < 3 {
		t.Errorf("expected at least 3 ollama targets, got %d", foundOllamaModels)
	}
}
