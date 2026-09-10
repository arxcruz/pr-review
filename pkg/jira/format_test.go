package jira

import (
	"strings"
	"testing"
	"time"
)

func TestFormatTicketSummary(t *testing.T) {
	ticket := &Ticket{
		Key:         "STRAT-100",
		Summary:     "Refactor Authentication Architecture",
		Description: "Detailed description of the strategic initiative.",
		Status:      "In Progress",
		IssueType:   "Initiative",
		Comments: []Comment{
			{
				ID:      "1",
				Author:  "Alice Bob",
				Body:    "Please review the threat model first.",
				Created: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
			},
		},
		IssueLinks: []IssueLink{
			{
				Relationship: "blocks",
				Key:          "DELIV-200",
				Summary:      "Implement OAuth2 flow",
				Status:       "To Do",
			},
			{
				Relationship: "is blocked by",
				Key:          "INFRA-300",
				Summary:      "Provision Redis cluster",
				Status:       "Done",
			},
		},
	}

	formatted := FormatTicketSummary(ticket)

	expectedSubstrings := []string{
		"STRAT-100",
		"Refactor Authentication Architecture",
		"In Progress",
		"Initiative",
		"Detailed description of the strategic initiative.",
		"Alice Bob",
		"Please review the threat model first.",
		"DELIV-200",
		"blocks",
		"Implement OAuth2 flow",
		"INFRA-300",
		"is blocked by",
		"Provision Redis cluster",
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(formatted, sub) {
			t.Errorf("expected output to contain %q, got:\n%s", sub, formatted)
		}
	}
}

func TestFormatTicketSummary_EmptyFields(t *testing.T) {
	ticket := &Ticket{
		Key:     "EMPTY-1",
		Summary: "Ticket with minimal fields",
		Status:  "Open",
	}

	formatted := FormatTicketSummary(ticket)
	if !strings.Contains(formatted, "EMPTY-1") {
		t.Errorf("expected formatted output to contain EMPTY-1, got:\n%s", formatted)
	}
	if !strings.Contains(formatted, "Ticket with minimal fields") {
		t.Errorf("expected formatted output to contain summary, got:\n%s", formatted)
	}
}

func TestFormatRecentTicketsTable_WithTickets(t *testing.T) {
	tickets := []RecentTicket{
		{
			Key:      "STRAT-10",
			Summary:  "Implement Auth Subsystem",
			Status:   "In Progress",
			Assignee: "Alice Smith",
		},
		{
			Key:      "STRAT-11",
			Summary:  "Database Sharding Plan",
			Status:   "Open",
			Assignee: "",
		},
	}

	table := FormatRecentTicketsTable(tickets)

	expectedHeaders := []string{"KEY", "STATUS", "ASSIGNEE", "SUMMARY"}
	for _, h := range expectedHeaders {
		if !strings.Contains(table, h) {
			t.Errorf("expected table to contain header %q, got:\n%s", h, table)
		}
	}

	expectedSubstrings := []string{
		"STRAT-10", "In Progress", "Alice Smith", "Implement Auth Subsystem",
		"STRAT-11", "Open", "Unassigned", "Database Sharding Plan",
	}
	for _, sub := range expectedSubstrings {
		if !strings.Contains(table, sub) {
			t.Errorf("expected table to contain %q, got:\n%s", sub, table)
		}
	}
}

func TestFormatRecentTicketsTable_Empty(t *testing.T) {
	table := FormatRecentTicketsTable(nil)
	if !strings.Contains(table, "No unresolved tickets found") {
		t.Errorf("expected message for empty tickets, got: %q", table)
	}

	table = FormatRecentTicketsTable([]RecentTicket{})
	if !strings.Contains(table, "No unresolved tickets found") {
		t.Errorf("expected message for empty tickets, got: %q", table)
	}
}

