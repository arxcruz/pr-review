package docscan_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arxcruz/pr-review/pkg/docscan"
)

func TestScanner_DiscoverDocs(t *testing.T) {
	repoDir := t.TempDir()

	// Create CONTEXT.md
	contextContent := "# System Domain\n\nCore domain glossary and architectural rules."
	if err := os.WriteFile(filepath.Join(repoDir, "CONTEXT.md"), []byte(contextContent), 0644); err != nil {
		t.Fatalf("failed to write CONTEXT.md: %v", err)
	}

	// Create docs/adr directory with 2 ADRs
	adrDir := filepath.Join(repoDir, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0755); err != nil {
		t.Fatalf("failed to create adr dir: %v", err)
	}

	adr2 := "# 0002: Use PostgreSQL\n\nDecision to use PostgreSQL."
	if err := os.WriteFile(filepath.Join(adrDir, "0002-postgres.md"), []byte(adr2), 0644); err != nil {
		t.Fatalf("failed to write adr2: %v", err)
	}

	adr1 := "# 0001: Event Driven Architecture\n\nDecision on events."
	if err := os.WriteFile(filepath.Join(adrDir, "0001-events.md"), []byte(adr1), 0644); err != nil {
		t.Fatalf("failed to write adr1: %v", err)
	}

	// Non-markdown file in adr dir should be ignored
	if err := os.WriteFile(filepath.Join(adrDir, "notes.txt"), []byte("not markdown"), 0644); err != nil {
		t.Fatalf("failed to write notes.txt: %v", err)
	}

	scanner := docscan.NewScanner(docscan.DefaultConfig())
	res, err := scanner.Scan([]string{repoDir})
	if err != nil {
		t.Fatalf("Scan returned unexpected error: %v", err)
	}

	if len(res.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %v", res.Warnings)
	}

	if len(res.Documents) != 3 {
		t.Fatalf("expected 3 documents, got %d", len(res.Documents))
	}

	// CONTEXT.md should be first
	if res.Documents[0].RelativePath != "CONTEXT.md" {
		t.Errorf("expected first doc to be CONTEXT.md, got %s", res.Documents[0].RelativePath)
	}
	if res.Documents[0].Title != "System Domain" {
		t.Errorf("expected title 'System Domain', got %q", res.Documents[0].Title)
	}
	if res.Documents[0].Content != contextContent {
		t.Errorf("expected content match for CONTEXT.md")
	}

	// ADRs should be sorted alphabetically
	if res.Documents[1].RelativePath != filepath.Join("docs", "adr", "0001-events.md") {
		t.Errorf("expected second doc to be 0001-events.md, got %s", res.Documents[1].RelativePath)
	}
	if res.Documents[1].Title != "0001: Event Driven Architecture" {
		t.Errorf("expected title '0001: Event Driven Architecture', got %q", res.Documents[1].Title)
	}

	if res.Documents[2].RelativePath != filepath.Join("docs", "adr", "0002-postgres.md") {
		t.Errorf("expected third doc to be 0002-postgres.md, got %s", res.Documents[2].RelativePath)
	}
	if res.Documents[2].Title != "0002: Use PostgreSQL" {
		t.Errorf("expected title '0002: Use PostgreSQL', got %q", res.Documents[2].Title)
	}
}

func TestScanner_NonExistentAndInvalidPaths(t *testing.T) {
	nonExistentDir := filepath.Join(t.TempDir(), "does_not_exist")
	tmpFile := filepath.Join(t.TempDir(), "a_file.txt")
	if err := os.WriteFile(tmpFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write tmp file: %v", err)
	}

	var capturedWarnings []string
	cfg := docscan.DefaultConfig()
	cfg.WarnFunc = func(w string) {
		capturedWarnings = append(capturedWarnings, w)
	}

	scanner := docscan.NewScanner(cfg)
	res, err := scanner.Scan([]string{nonExistentDir, tmpFile, ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Documents) != 0 {
		t.Errorf("expected 0 documents, got %d", len(res.Documents))
	}

	if len(res.Warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d: %v", len(res.Warnings), res.Warnings)
	}

	if len(capturedWarnings) != 2 {
		t.Errorf("expected WarnFunc to be called 2 times, got %d", len(capturedWarnings))
	}
}

func TestScanner_MultiContextRepo(t *testing.T) {
	repoDir := t.TempDir()

	// Root CONTEXT-MAP.md
	contextMap := "# Context Map\n\nMap of contexts."
	if err := os.WriteFile(filepath.Join(repoDir, "CONTEXT-MAP.md"), []byte(contextMap), 0644); err != nil {
		t.Fatalf("failed to write CONTEXT-MAP.md: %v", err)
	}

	// Root ADR
	rootAdrDir := filepath.Join(repoDir, "docs", "adr")
	if err := os.MkdirAll(rootAdrDir, 0755); err != nil {
		t.Fatalf("failed to create root adr dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootAdrDir, "0001-global.md"), []byte("# Global ADR 1\n\nGlobal decision."), 0644); err != nil {
		t.Fatalf("failed to write global adr: %v", err)
	}

	// Sub-context: src/billing
	billingDir := filepath.Join(repoDir, "src", "billing")
	billingAdrDir := filepath.Join(billingDir, "docs", "adr")
	if err := os.MkdirAll(billingAdrDir, 0755); err != nil {
		t.Fatalf("failed to create billing adr dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(billingDir, "CONTEXT.md"), []byte("# Billing Context\n\nBilling rules."), 0644); err != nil {
		t.Fatalf("failed to write billing CONTEXT.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(billingAdrDir, "0001-stripe.md"), []byte("# Stripe Integration\n\nStripe decision."), 0644); err != nil {
		t.Fatalf("failed to write billing adr: %v", err)
	}

	scanner := docscan.NewScanner(docscan.DefaultConfig())
	res, err := scanner.Scan([]string{repoDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %v", res.Warnings)
	}

	// Expected: CONTEXT-MAP.md, docs/adr/0001-global.md, src/billing/CONTEXT.md, src/billing/docs/adr/0001-stripe.md
	if len(res.Documents) != 4 {
		t.Fatalf("expected 4 documents, got %d", len(res.Documents))
	}

	paths := make([]string, len(res.Documents))
	for i, d := range res.Documents {
		paths[i] = d.RelativePath
	}

	expectedPaths := []string{
		"CONTEXT-MAP.md",
		filepath.Join("docs", "adr", "0001-global.md"),
		filepath.Join("src", "billing", "CONTEXT.md"),
		filepath.Join("src", "billing", "docs", "adr", "0001-stripe.md"),
	}

	for i, exp := range expectedPaths {
		if paths[i] != exp {
			t.Errorf("doc[%d] expected %s, got %s", i, exp, paths[i])
		}
	}
}

func TestScanner_UnreadableFile(t *testing.T) {
	// Skip if running as root where 0000 permissions don't prevent reading
	if os.Geteuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	repoDir := t.TempDir()
	unreadableFile := filepath.Join(repoDir, "CONTEXT.md")
	if err := os.WriteFile(unreadableFile, []byte("content"), 0000); err != nil {
		t.Fatalf("failed to create unreadable file: %v", err)
	}

	scanner := docscan.NewScanner(docscan.DefaultConfig())
	res, err := scanner.Scan([]string{repoDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Warnings) == 0 {
		t.Errorf("expected warning for unreadable file, got 0 warnings")
	}
}


