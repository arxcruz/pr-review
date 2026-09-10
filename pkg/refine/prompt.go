package refine

import (
	"fmt"
	"strings"
	"time"

	"github.com/arxcruz/pr-review/pkg/jira"
	"github.com/arxcruz/pr-review/pkg/session"
)

const defaultGuidelines = `Refinement Principles:
1. Identify high-impact architectural decisions (data storage, sync vs async communication, team boundaries, operational scalability).
2. Clarify ambiguous requirements or unstated non-functional requirements (SLAs, security, auditability).
3. Align decisions with existing repository architecture and documented ADRs.
4. Keep questions actionable and concise with 2 to 4 distinct options.
5. Provide a clear, opinionated recommendation with rationale for each question.
6. When all major decisions are settled, return an empty list [] to signal the frontier is resolved.`

// BuildFrontierPrompt constructs system and user prompts incorporating domain guidelines,
// ticket details, multi-repo doc context, and previous refinement rounds.
func BuildFrontierPrompt(ticket jira.Ticket, rounds []session.Round, docContext string, guidelines string) (string, string) {
	effectiveGuidelines := strings.TrimSpace(guidelines)
	if effectiveGuidelines == "" {
		effectiveGuidelines = defaultGuidelines
	}

	systemPrompt := fmt.Sprintf(`You are an expert principal software architect and technical lead conducting structured, interactive refinement of strategic Jira tickets into decomposed delivery items.

Your task is to examine the provided Strategic Ticket, multi-repo architectural documentation, domain guidelines, and any previously answered questions to formulate the next batch of "frontier questions".

Frontier questions focus on:
- Key architectural trade-offs (e.g. data consistency, protocols, frameworks)
- Component and team delivery boundaries
- Unresolved non-functional requirements (rate limits, security, failover)
- Resolving ambiguity in business requirements

Guidelines:
%s

Output Format:
You MUST respond with a valid JSON array of question objects. Do not include extraneous conversational text outside the JSON.
Each question object must follow this exact structure:
[
  {
    "id": "Q1",
    "title": "Short title of the question",
    "explanation": "Why this decision matters and background context",
    "options": ["Option A", "Option B", "Option C"],
    "recommendation": "Option A"
  }
]

If all critical architectural questions and ambiguities have already been addressed and the ticket is ready for decomposition into Epics and Tasks, respond with an empty JSON array: []`, effectiveGuidelines)

	var sb strings.Builder
	formatTicketPromptContext(&sb, ticket, rounds, docContext, guidelines)
	sb.WriteString("\nPlease formulate the next batch of frontier questions as a JSON array (or [] if complete).")

	return systemPrompt, sb.String()
}

// BuildDecompositionPrompt constructs system and user prompts to synthesize settled refinement
// rounds and architectural context into a full Decomposition Tree JSON payload.
func BuildDecompositionPrompt(ticket jira.Ticket, rounds []session.Round, docContext string, guidelines string) (string, string) {
	effectiveGuidelines := strings.TrimSpace(guidelines)
	if effectiveGuidelines == "" {
		effectiveGuidelines = defaultGuidelines
	}

	systemPrompt := fmt.Sprintf(`You are an expert principal software architect and technical lead.
Your task is to synthesize the provided Strategic Ticket, architectural documentation, refinement guidelines, and the settled answers from interactive refinement rounds into a comprehensive Decomposition Tree of Epics and Tasks/Stories.

Decomposition Rules:
1. Break down the initiative into coherent, delivery-sized Epics.
2. Under each Epic, create concrete Stories or Tasks with clear titles and technical descriptions.
3. Every task MUST have explicit, verifiable acceptance criteria (acceptance_criteria list).
4. Identify dependency relationships between tasks (depends_on list containing prerequisite task IDs). Dependencies must be an acyclic directed graph (no circular dependencies).
5. All task and epic IDs must be unique (e.g. EPIC-1, TASK-1, TASK-2, etc.).
6. Target Delivery Projects may be left empty or suggested if obvious from the context.

Guidelines:
%s

Output Format:
You MUST respond with a valid JSON object matching the Decomposition Tree schema below. Do not include markdown conversational prose outside the JSON.
{
  "epics": [
    {
      "id": "EPIC-1",
      "title": "Short title of epic",
      "description": "Detailed description of epic scope and purpose",
      "delivery_project": "",
      "tasks": [
        {
          "id": "TASK-1",
          "title": "Short task title",
          "description": "Technical implementation instructions",
          "type": "Story",
          "acceptance_criteria": [
            "Acceptance criterion 1",
            "Acceptance criterion 2"
          ],
          "delivery_project": "",
          "depends_on": []
        }
      ]
    }
  ]
}`, effectiveGuidelines)

	var sb strings.Builder
	formatTicketPromptContext(&sb, ticket, rounds, docContext, guidelines)
	sb.WriteString("\nPlease synthesize the settled requirements, ticket specifications, and architectural context into a complete Decomposition Tree matching the JSON schema.")

	return systemPrompt, sb.String()
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
