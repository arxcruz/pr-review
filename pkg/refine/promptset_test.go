package refine_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arxcruz/pr-review/pkg/refine"
)

func TestDefaultPromptSet(t *testing.T) {
	ps, err := refine.DefaultPromptSet()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(ps.Frontier, "{{.Guidelines}}") {
		t.Errorf("expected Frontier template to reference .Guidelines, got: %s", ps.Frontier)
	}
	if !strings.Contains(ps.Decomposition, "{{.Guidelines}}") {
		t.Errorf("expected Decomposition template to reference .Guidelines, got: %s", ps.Decomposition)
	}
	if strings.TrimSpace(ps.Guidelines) == "" {
		t.Errorf("expected non-empty default Guidelines")
	}
}

func TestLoadPromptSet_ExplicitPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.md")
	content := "## Frontier Prompt\nCustom frontier {{.Guidelines}}\n\n## Decomposition Prompt\nCustom decomposition {{.Guidelines}}\n\n## Guidelines\nCustom rules.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ps, err := refine.LoadPromptSet(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ps.Frontier != "Custom frontier {{.Guidelines}}" {
		t.Errorf("unexpected Frontier content: %q", ps.Frontier)
	}
	if ps.Guidelines != "Custom rules." {
		t.Errorf("unexpected Guidelines content: %q", ps.Guidelines)
	}
}

func TestLoadPromptSet_ExplicitPathMissing(t *testing.T) {
	_, err := refine.LoadPromptSet(filepath.Join(t.TempDir(), "does-not-exist.md"), nil)
	if err == nil {
		t.Fatal("expected error for missing explicit prompt file, got nil")
	}
}

func TestLoadPromptSet_AutoWritesDefaultOnFirstRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var msg bytes.Buffer
	ps, err := refine.LoadPromptSet("", &msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(ps.Guidelines) == "" {
		t.Errorf("expected default Guidelines to be loaded")
	}

	configPath := filepath.Join(home, ".config", "jira-refine", "jira-refine.md")
	if _, statErr := os.Stat(configPath); statErr != nil {
		t.Fatalf("expected default prompt file to be written to %s: %v", configPath, statErr)
	}
	if !strings.Contains(msg.String(), configPath) {
		t.Errorf("expected message to mention %s, got: %s", configPath, msg.String())
	}

	// Second run should read the now-existing file without rewriting/erroring.
	msg.Reset()
	if _, err := refine.LoadPromptSet("", &msg); err != nil {
		t.Fatalf("unexpected error on second load: %v", err)
	}
	if msg.Len() != 0 {
		t.Errorf("expected no message when config file already exists, got: %s", msg.String())
	}
}

func TestLoadPromptSet_MissingRequiredSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.md")
	if err := os.WriteFile(path, []byte("## Guidelines\nOnly guidelines here.\n"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := refine.LoadPromptSet(path, nil)
	if err == nil {
		t.Fatal("expected error for prompt file missing required sections, got nil")
	}
}
