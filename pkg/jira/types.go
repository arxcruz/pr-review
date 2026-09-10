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

// CreateIssueLinkRequest represents parameters to link two Jira issues via /rest/api/2/issueLink.
type CreateIssueLinkRequest struct {
	LinkType   string `json:"link_type"`   // e.g. "Relates", "implements", "Blocks"
	InwardKey  string `json:"inward_key"`  // Key of inward issue
	OutwardKey string `json:"outward_key"` // Key of outward issue
	Comment    string `json:"comment,omitempty"`
}

// StrategicLinkRequest represents parameters to link a delivery issue back to an origin strategic ticket.
type StrategicLinkRequest struct {
	ChildKey       string `json:"child_key"`
	OriginKey      string `json:"origin_key"`
	LinkType       string `json:"link_type,omitempty"`
	ForceIssueLink bool   `json:"force_issue_link,omitempty"`
}

// StrategicLinkResult represents the outcome of linking a child issue to an origin strategic ticket.
type StrategicLinkResult struct {
	ChildKey  string `json:"child_key"`
	OriginKey string `json:"origin_key"`
	Method    string `json:"method"` // "parent" or "issue_link"
	LinkType  string `json:"link_type,omitempty"`
}


