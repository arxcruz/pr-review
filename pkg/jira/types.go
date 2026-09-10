package jira

import "time"

// Comment represents a Jira issue comment.
type Comment struct {
	ID      string    `json:"id"`
	Author  string    `json:"author"`
	Body    string    `json:"body"`
	Created time.Time `json:"created"`
}

// IssueLink represents an inward or outward link from a Jira issue.
type IssueLink struct {
	Relationship string `json:"relationship"` // e.g. "blocks", "is blocked by", "relates to"
	Key          string `json:"key"`
	Summary      string `json:"summary"`
	Status       string `json:"status"`
}

// Ticket represents a fetched Jira Strategic Ticket with all required details.
type Ticket struct {
	Key         string      `json:"key"`
	Summary     string      `json:"summary"`
	Description string      `json:"description"`
	Status      string      `json:"status"`
	IssueType   string      `json:"issue_type"`
	Comments    []Comment   `json:"comments"`
	IssueLinks  []IssueLink `json:"issue_links"`
}

// RecentTicket represents a summary of a Jira ticket from search results.
type RecentTicket struct {
	Key      string `json:"key"`
	Summary  string `json:"summary"`
	Status   string `json:"status"`
	Assignee string `json:"assignee"`
}

