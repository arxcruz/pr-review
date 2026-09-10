package jira

import (
	"bytes"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"
)

// FormatSummary formats a Ticket for human-readable terminal output.
func (t *Ticket) FormatSummary() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("=== Strategic Ticket: %s ===\n", t.Key))
	sb.WriteString(fmt.Sprintf("Summary:     %s\n", t.Summary))
	if t.IssueType != "" {
		sb.WriteString(fmt.Sprintf("Type:        %s\n", t.IssueType))
	}
	if t.Status != "" {
		sb.WriteString(fmt.Sprintf("Status:      %s\n", t.Status))
	}

	sb.WriteString("\n--- Description ---\n")
	if strings.TrimSpace(t.Description) != "" {
		sb.WriteString(strings.TrimSpace(t.Description))
		sb.WriteString("\n")
	} else {
		sb.WriteString("(No description)\n")
	}

	sb.WriteString(fmt.Sprintf("\n--- Linked Issues (%d) ---\n", len(t.IssueLinks)))
	if len(t.IssueLinks) == 0 {
		sb.WriteString("No linked issues.\n")
	} else {
		for _, link := range t.IssueLinks {
			statusStr := ""
			if link.Status != "" {
				statusStr = fmt.Sprintf(" [%s]", link.Status)
			}
			sb.WriteString(fmt.Sprintf("• [%s] %s: %s%s\n", link.Relationship, link.Key, link.Summary, statusStr))
		}
	}

	sb.WriteString(fmt.Sprintf("\n--- Comments (%d) ---\n", len(t.Comments)))
	if len(t.Comments) == 0 {
		sb.WriteString("No comments.\n")
	} else {
		for _, c := range t.Comments {
			dateStr := ""
			if !c.Created.IsZero() {
				dateStr = fmt.Sprintf(" (%s)", c.Created.Format(time.RFC822))
			}
			sb.WriteString(fmt.Sprintf("• %s%s:\n  %s\n", c.Author, dateStr, strings.TrimSpace(c.Body)))
		}
	}

	return sb.String()
}

// FormatTicketSummary formats a Ticket for human-readable terminal output.
func FormatTicketSummary(t *Ticket) string {
	if t == nil {
		return ""
	}
	return t.FormatSummary()
}

// FormatRecentTicketsTable formats a list of recent tickets as an aligned table.
func FormatRecentTicketsTable(tickets []RecentTicket) string {
	if len(tickets) == 0 {
		return "No unresolved tickets found.\n"
	}

	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "KEY\tSTATUS\tASSIGNEE\tSUMMARY")
	for _, t := range tickets {
		status := t.Status
		if status == "" {
			status = "-"
		}
		assignee := t.Assignee
		if assignee == "" {
			assignee = "Unassigned"
		}
		summary := strings.ReplaceAll(t.Summary, "\t", " ")
		summary = strings.ReplaceAll(summary, "\n", " ")
		summary = strings.ReplaceAll(summary, "\r", " ")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.Key, status, assignee, strings.TrimSpace(summary))
	}
	w.Flush()
	return buf.String()
}


