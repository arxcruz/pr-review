package docscan

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// truncateUTF8 safely truncates a string up to maxBytes without splitting UTF-8 runes.
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	if maxBytes <= 0 {
		return ""
	}
	idx := maxBytes
	for idx > 0 && !utf8.RuneStart(s[idx]) {
		idx--
	}
	return s[:idx]
}

// FormatPromptContext collates documents into a structured Markdown section for LLM prompts,
// truncating individual documents and total output to respect context limits.
func FormatPromptContext(docs []Document, maxTotalBytes, maxDocBytes int) string {
	if len(docs) == 0 {
		return ""
	}

	if maxTotalBytes <= 0 {
		maxTotalBytes = DefaultMaxTotalBytes
	}
	if maxDocBytes <= 0 {
		maxDocBytes = DefaultMaxDocBytes
	}

	var sb strings.Builder
	sb.WriteString("## Architectural & Domain Documentation Context\n\n")

	for i, doc := range docs {
		content := strings.TrimSpace(doc.Content)
		if len(content) > maxDocBytes {
			notice := fmt.Sprintf("\n... [Content truncated: original size %d bytes] ...", len(doc.Content))
			if maxDocBytes > len(notice) {
				content = truncateUTF8(content, maxDocBytes-len(notice)) + notice
			} else {
				content = truncateUTF8(content, maxDocBytes)
			}
		}

		repoName := filepath.Base(doc.RepoPath)
		if repoName == "." || repoName == "/" || repoName == "" {
			repoName = doc.RepoPath
		}

		titleStr := ""
		if doc.Title != "" && doc.Title != filepath.Base(doc.RelativePath) {
			titleStr = fmt.Sprintf(" - %s", doc.Title)
		}

		docBlock := fmt.Sprintf("### [%s] %s%s\n\n```markdown\n%s\n```\n\n", repoName, doc.RelativePath, titleStr, content)

		// Check total budget
		currentLen := sb.Len()
		if currentLen+len(docBlock) > maxTotalBytes {
			remainingDocs := len(docs) - i
			omissionNotice := fmt.Sprintf("> Note: %d documentation file(s) omitted to stay within prompt context budget.\n", remainingDocs)

			// Determine budget left after reserving space for omission notice
			usableBudget := maxTotalBytes - currentLen - len(omissionNotice)
			headerPrefix := fmt.Sprintf("### [%s] %s%s\n\n```markdown\n", repoName, doc.RelativePath, titleStr)
			footerSuffix := "\n... [Documentation truncated to fit context budget] ...\n```\n\n"

			minNeeded := len(headerPrefix) + len(footerSuffix) + 20
			if usableBudget >= minNeeded {
				availContentLen := usableBudget - len(headerPrefix) - len(footerSuffix)
				if availContentLen > len(content) {
					availContentLen = len(content)
				}
				if availContentLen > 0 {
					partialContent := truncateUTF8(content, availContentLen)
					sb.WriteString(headerPrefix)
					sb.WriteString(partialContent)
					sb.WriteString(footerSuffix)
					remainingDocs--
					if remainingDocs > 0 {
						omissionNotice = fmt.Sprintf("> Note: %d documentation file(s) omitted to stay within prompt context budget.\n", remainingDocs)
						sb.WriteString(omissionNotice)
					}
					break
				}
			}

			sb.WriteString(omissionNotice)
			break
		}

		sb.WriteString(docBlock)
	}

	return strings.TrimSpace(sb.String()) + "\n"
}

// FormatPromptContext formats the given documents using the scanner's configured limits.
func (s *Scanner) FormatPromptContext(docs []Document) string {
	return FormatPromptContext(docs, s.cfg.MaxTotalBytes, s.cfg.MaxDocBytes)
}
