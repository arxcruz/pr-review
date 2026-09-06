package ai

import (
	"fmt"
	"strings"
)

// BuildReviewPrompt creates the structured system and user prompts for the AI
func BuildReviewPrompt(req ReviewRequest) (systemPrompt string, userPrompt string) {
	guidelines := req.Guidelines
	if strings.TrimSpace(guidelines) == "" {
		guidelines = `Focus on:
1. Logic errors, race conditions, edge cases, and potential bugs
2. Security issues or vulnerabilities
3. Performance impacts or unnecessary allocations
4. Code readability, idiomatic patterns, and maintainability
Provide constructive, specific feedback with concise code snippets if suggesting improvements.
Structure your review with:
- Summary of changes
- Key observations & potential issues (categorized by severity: High, Medium, Low)
- Positive highlights
- Final recommendation (Approve, Request Changes, Comment)`
	}

	systemPrompt = fmt.Sprintf(`You are an expert senior software engineer and code reviewer.
Review the pull request / merge request diff provided by the user.

Review Guidelines:
%s

Mandatory Review Output Rules:
- **Be specific & actionable**: For any suggestion or issue found, explicitly state the filename and the line number (or surrounding context/function name) from the diff.
- **Provide concrete code snippets**: Whenever suggesting a fix or improvement, show the exact code snippet or diff (e.g. ` + "```diff\n- original_code\n+ suggested_code\n```" + `).
- Format your response in clean Markdown with clear headings and bullet points.
- Be constructive, direct, and concise.`, guidelines)

	var sb strings.Builder
	if strings.TrimSpace(req.ProjectDescription) != "" {
		sb.WriteString("## Project Context\n")
		if req.ProjectID != "" {
			sb.WriteString(fmt.Sprintf("- **Project**: %s\n", req.ProjectID))
		}
		sb.WriteString(fmt.Sprintf("%s\n\n", strings.TrimSpace(req.ProjectDescription)))
	} else if req.ProjectID != "" {
		sb.WriteString(fmt.Sprintf("## Project Context\n- **Project**: %s\n\n", req.ProjectID))
	}

	sb.WriteString("## Pull Request Details\n")
	sb.WriteString(fmt.Sprintf("- **Title**: %s\n", req.PR.Title))
	sb.WriteString(fmt.Sprintf("- **Author**: %s\n", req.PR.Author))
	sb.WriteString(fmt.Sprintf("- **Source Branch**: %s\n", req.PR.SourceBranch))
	sb.WriteString(fmt.Sprintf("- **Target Branch**: %s\n", req.PR.TargetBranch))
	if len(req.PR.Labels) > 0 {
		sb.WriteString(fmt.Sprintf("- **Labels**: %s\n", strings.Join(req.PR.Labels, ", ")))
	}
	if strings.TrimSpace(req.PR.Description) != "" {
		sb.WriteString(fmt.Sprintf("\n### Description\n%s\n", req.PR.Description))
	}

	sb.WriteString("\n## Git Diff\n```diff\n")
	// If diff is too huge, truncate with note
	const maxDiffLen = 120000 // approx 30k tokens
	if len(req.Diff) > maxDiffLen {
		sb.WriteString(req.Diff[:maxDiffLen])
		sb.WriteString(fmt.Sprintf("\n... [Diff truncated: total size %d bytes] ...\n", len(req.Diff)))
	} else {
		sb.WriteString(req.Diff)
	}
	sb.WriteString("\n```\n")
	sb.WriteString("\nPlease perform a thorough code review based on the diff above.")

	userPrompt = sb.String()
	return systemPrompt, userPrompt
}
