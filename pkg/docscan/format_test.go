package docscan_test

import (
	"strings"
	"testing"

	"github.com/arxcruz/pr-review/pkg/docscan"
)

func TestFormatPromptContext_Empty(t *testing.T) {
	out := docscan.FormatPromptContext(nil, 1000, 500)
	if out != "" {
		t.Errorf("expected empty string for nil docs, got %q", out)
	}

	out = docscan.FormatPromptContext([]docscan.Document{}, 1000, 500)
	if out != "" {
		t.Errorf("expected empty string for empty docs, got %q", out)
	}
}

func TestFormatPromptContext_Formatting(t *testing.T) {
	docs := []docscan.Document{
		{
			RepoPath:     "/home/user/repo-a",
			RelativePath: "CONTEXT.md",
			Title:        "Repo A Architecture",
			Content:      "# Repo A Architecture\n\nDomain rules for Repo A.",
		},
		{
			RepoPath:     "/home/user/repo-a",
			RelativePath: "docs/adr/0001-events.md",
			Title:        "Event Driven",
			Content:      "# Event Driven\n\nWe use Kafka.",
		},
	}

	out := docscan.FormatPromptContext(docs, 10000, 5000)

	if !strings.Contains(out, "CONTEXT.md") {
		t.Errorf("expected output to contain CONTEXT.md")
	}
	if !strings.Contains(out, "Domain rules for Repo A.") {
		t.Errorf("expected output to contain Repo A content")
	}
	if !strings.Contains(out, "0001-events.md") {
		t.Errorf("expected output to contain 0001-events.md")
	}
	if !strings.Contains(out, "We use Kafka.") {
		t.Errorf("expected output to contain Kafka content")
	}
}

func TestFormatPromptContext_TruncatePerDoc(t *testing.T) {
	longContent := strings.Repeat("A quick brown fox jumps over the lazy dog. ", 50)
	docs := []docscan.Document{
		{
			RepoPath:     "/repo",
			RelativePath: "CONTEXT.md",
			Title:        "Long Doc",
			Content:      longContent,
		},
	}

	maxDocBytes := 100
	maxTotalBytes := 5000
	out := docscan.FormatPromptContext(docs, maxTotalBytes, maxDocBytes)

	if !strings.Contains(out, "truncated") {
		t.Errorf("expected truncation message in output, got: %s", out)
	}
	if len(out) > 500 {
		t.Errorf("expected total formatted size to be bounded, got %d bytes", len(out))
	}
}

func TestFormatPromptContext_TruncateTotalContext(t *testing.T) {
	docs := []docscan.Document{
		{
			RepoPath:     "/repo",
			RelativePath: "CONTEXT.md",
			Title:        "Doc 1",
			Content:      "Doc 1 content is here and has some length to it.",
		},
		{
			RepoPath:     "/repo",
			RelativePath: "docs/adr/0001-foo.md",
			Title:        "Doc 2",
			Content:      "Doc 2 content is also here and takes up space.",
		},
		{
			RepoPath:     "/repo",
			RelativePath: "docs/adr/0002-bar.md",
			Title:        "Doc 3",
			Content:      "Doc 3 content should exceed the tight total budget.",
		},
	}

	maxTotalBytes := 220
	maxDocBytes := 1000
	out := docscan.FormatPromptContext(docs, maxTotalBytes, maxDocBytes)

	if len(out) > maxTotalBytes+100 {
		t.Errorf("expected formatted output to respect budget, got length %d (budget %d)", len(out), maxTotalBytes)
	}
	if !strings.Contains(out, "omitted") && !strings.Contains(out, "truncated") {
		t.Errorf("expected omission or truncation notice, got: %s", out)
	}
}

func TestScanner_FormatPromptContext_Integration(t *testing.T) {
	scanner := docscan.NewScanner(docscan.Config{
		MaxTotalBytes: 5000,
		MaxDocBytes:   1000,
	})

	docs := []docscan.Document{
		{
			RepoPath:     "/repo",
			RelativePath: "CONTEXT.md",
			Title:        "Overview",
			Content:      "System overview",
		},
	}

	formatted := scanner.FormatPromptContext(docs)
	if !strings.Contains(formatted, "## Architectural & Domain Documentation Context") {
		t.Errorf("expected header in formatted prompt, got: %s", formatted)
	}
	if !strings.Contains(formatted, "System overview") {
		t.Errorf("expected content in formatted prompt, got: %s", formatted)
	}
}

func TestFormatPromptContext_UTF8Safety(t *testing.T) {
	// 3-byte UTF-8 runes (Japanese characters: こんにちは)
	japaneseText := "こんにちは世界！" + strings.Repeat("あいうえお", 20)
	docs := []docscan.Document{
		{
			RepoPath:     "/repo",
			RelativePath: "CONTEXT.md",
			Title:        "UTF8 Doc",
			Content:      japaneseText,
		},
	}

	// Truncate at an offset that falls in the middle of a 3-byte rune
	out := docscan.FormatPromptContext(docs, 200, 50)
	// Ensure result is valid UTF-8
	if !strings.Contains(out, "UTF8 Doc") {
		t.Errorf("expected doc title in output")
	}
	for i, r := range out {
		if r == '\uFFFD' {
			t.Errorf("found invalid UTF-8 replacement rune at position %d", i)
		}
	}
}



