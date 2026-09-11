package refine

import (
	"fmt"
	"strings"
	"time"

	"github.com/arxcruz/pr-review/pkg/jira"
	"github.com/arxcruz/pr-review/pkg/session"
)

// BuildFrontierPrompt constructs system and user prompts incorporating domain guidelines,
// ticket details, multi-repo doc context, and previous refinement rounds.
func BuildFrontierPrompt(ps *PromptSet, ticket jira.Ticket, rounds []session.Round, docContext string) (string, string, error) {
	systemPrompt, err := renderPromptTemplate("Frontier Prompt", ps.Frontier, ps.Guidelines)
	if err != nil {
		return "", "", err
	}

	var sb strings.Builder
	formatTicketPromptContext(&sb, ticket, rounds, docContext, ps.Guidelines)
	sb.WriteString("\nPlease formulate the next batch of frontier questions as a JSON array (or [] if complete).")

	return systemPrompt, sb.String(), nil
}

// BuildDecompositionPrompt constructs system and user prompts to synthesize settled refinement
// rounds and architectural context into a full Decomposition Tree JSON payload.
func BuildDecompositionPrompt(ps *PromptSet, ticket jira.Ticket, rounds []session.Round, docContext string) (string, string, error) {
	systemPrompt, err := renderPromptTemplate("Decomposition Prompt", ps.Decomposition, ps.Guidelines)
	if err != nil {
		return "", "", err
	}

	var sb strings.Builder
	formatTicketPromptContext(&sb, ticket, rounds, docContext, ps.Guidelines)
	sb.WriteString("\nPlease synthesize the settled requirements, ticket specifications, and architectural context into a complete Decomposition Tree matching the JSON schema.")

	return systemPrompt, sb.String(), nil
}

func formatTicketPromptContext(sb *strings.Builder, ticket jira.Ticket, rounds []session.Round, docContext string, guidelines string) {
	sb.WriteString("## Strategic Ticket Details\n")
	sb.WriteString(fmt.Sprintf("- **Key**: %s\n", ticket.Key))
	sb.WriteString(fmt.Sprintf("- **Summary**: %s\n", ticket.Summary))
	if ticket.IssueType != "" {
		sb.WriteString(fmt.Sprintf("- **Issue Type**: %s\n", ticket.IssueType))
	}
	if ticket.Status != "" {
		sb.WriteString(fmt.Sprintf("- **Status**: %s\n", ticket.Status))
	}

	sb.WriteString("\n### Description\n")
	if strings.TrimSpace(ticket.Description) != "" {
		sb.WriteString(strings.TrimSpace(ticket.Description))
		sb.WriteString("\n")
	} else {
		sb.WriteString("(No description provided)\n")
	}

	if len(ticket.IssueLinks) > 0 {
		sb.WriteString(fmt.Sprintf("\n### Linked Issues (%d)\n", len(ticket.IssueLinks)))
		for _, link := range ticket.IssueLinks {
			statusStr := ""
			if link.Status != "" {
				statusStr = fmt.Sprintf(" [%s]", link.Status)
			}
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s%s\n", link.Relationship, link.Key, link.Summary, statusStr))
		}
	}

	if len(ticket.Comments) > 0 {
		sb.WriteString(fmt.Sprintf("\n### Comments (%d)\n", len(ticket.Comments)))
		for _, c := range ticket.Comments {
			dateStr := ""
			if !c.Created.IsZero() {
				dateStr = fmt.Sprintf(" (%s)", c.Created.Format(time.RFC822))
			}
			sb.WriteString(fmt.Sprintf("- **%s**%s: %s\n", c.Author, dateStr, strings.TrimSpace(c.Body)))
		}
	}

	if strings.TrimSpace(docContext) != "" {
		sb.WriteString("\n")
		sb.WriteString(strings.TrimSpace(docContext))
		sb.WriteString("\n")
	}

	if strings.TrimSpace(guidelines) != "" {
		sb.WriteString("\n## Domain & Refinement Guidelines\n")
		sb.WriteString(strings.TrimSpace(guidelines))
		sb.WriteString("\n")
	}

	if len(rounds) > 0 {
		sb.WriteString("\n## Previous Refinement Rounds & User Answers\n")
		for _, r := range rounds {
			sb.WriteString(fmt.Sprintf("### Round %d\n", r.Number))
			for _, q := range r.Questions {
				answerStr := q.Answer
				if answerStr == "" {
					answerStr = "(Unanswered)"
				}
				sb.WriteString(fmt.Sprintf("- **%s: %s**\n", q.ID, q.Title))
				if len(q.Options) > 0 {
					sb.WriteString(fmt.Sprintf("  Options: %s\n", strings.Join(q.Options, ", ")))
				}
				if q.Recommendation != "" {
					sb.WriteString(fmt.Sprintf("  Recommended: %s\n", q.Recommendation))
				}
				sb.WriteString(fmt.Sprintf("  **User Chosen Answer**: %s\n", answerStr))
			}
		}
	}
}
