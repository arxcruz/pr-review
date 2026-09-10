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

// CreateIssueRequest represents parameters to create any Jira issue.
type CreateIssueRequest struct {
	Project            string                 `json:"project"`
	Summary            string                 `json:"summary"`
	Description        string                 `json:"description,omitempty"`
	IssueType          string                 `json:"issue_type"`
	ParentKey          string                 `json:"parent_key,omitempty"`
	AcceptanceCriteria []string               `json:"acceptance_criteria,omitempty"`
	CustomFields       map[string]interface{} `json:"custom_fields,omitempty"`
}

// CreateEpicRequest represents parameters to create an Epic issue.
type CreateEpicRequest struct {
	Project      string                 `json:"project"`
	Summary      string                 `json:"summary"`
	Description  string                 `json:"description,omitempty"`
	IssueType    string                 `json:"issue_type,omitempty"`
	CustomFields map[string]interface{} `json:"custom_fields,omitempty"`
}

// CreateTaskRequest represents parameters to create a Task or Story issue.
type CreateTaskRequest struct {
	Project            string                 `json:"project"`
	Summary            string                 `json:"summary"`
	Description        string                 `json:"description,omitempty"`
	IssueType          string                 `json:"issue_type,omitempty"`
	ParentKey          string                 `json:"parent_key,omitempty"`
	AcceptanceCriteria []string               `json:"acceptance_criteria,omitempty"`
	CustomFields       map[string]interface{} `json:"custom_fields,omitempty"`
}

// CreatedIssue represents the outcome of a Jira issue creation.
type CreatedIssue struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
	URL  string `json:"url"`
}

