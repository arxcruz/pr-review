package jira

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arxcruz/pr-review/pkg/config"
)

// Client interface defines Jira operations.
type Client interface {
	GetTicket(ctx context.Context, key string) (*Ticket, error)
	GetRecentTickets(ctx context.Context, project string) ([]RecentTicket, error)
}

// HTTPClient is the concrete implementation of Client talking to Jira REST API.
type HTTPClient struct {
	baseURL    string
	authHeader string
	client     *http.Client
}

// NewClient creates a new Jira API client from JiraConfig.
func NewClient(cfg config.JiraConfig) (Client, error) {
	baseURL := strings.TrimRight(cfg.URL, "/")
	if baseURL == "" {
		return nil, fmt.Errorf("jira url cannot be empty")
	}

	authHeader := ""
	if cfg.Token != "" {
		if strings.HasPrefix(cfg.Token, "Bearer ") || strings.HasPrefix(cfg.Token, "Basic ") {
			authHeader = cfg.Token
		} else if strings.Contains(cfg.Token, ":") {
			authHeader = "Basic " + base64.StdEncoding.EncodeToString([]byte(cfg.Token))
		} else {
			authHeader = "Bearer " + cfg.Token
		}
	} else if cfg.PAT != "" {
		authHeader = "Bearer " + cfg.PAT
	}

	return &HTTPClient{
		baseURL:    baseURL,
		authHeader: authHeader,
		client:     &http.Client{Timeout: 30 * time.Second},
	}, nil
}

type jiraIssueResponse struct {
	Key    string                 `json:"key"`
	Fields jiraIssueFieldsResponse `json:"fields"`
}

type jiraIssueFieldsResponse struct {
	Summary     string            `json:"summary"`
	Description json.RawMessage   `json:"description"`
	Status      *jiraNamedItem    `json:"status"`
	IssueType   *jiraNamedItem    `json:"issuetype"`
	Comment     *jiraCommentBlock `json:"comment"`
	IssueLinks  []jiraIssueLink   `json:"issuelinks"`
}

type jiraNamedItem struct {
	Name string `json:"name"`
}

type jiraCommentBlock struct {
	Comments []jiraCommentItem `json:"comments"`
}

type jiraCommentItem struct {
	ID      string          `json:"id"`
	Author  jiraAuthor      `json:"author"`
	Body    json.RawMessage `json:"body"`
	Created string          `json:"created"`
}

type jiraAuthor struct {
	DisplayName string `json:"displayName"`
	Name        string `json:"name"`
}

type jiraIssueLink struct {
	ID           string             `json:"id"`
	Type         jiraLinkType       `json:"type"`
	InwardIssue  *jiraLinkedIssue   `json:"inwardIssue"`
	OutwardIssue *jiraLinkedIssue   `json:"outwardIssue"`
}

type jiraLinkType struct {
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

type jiraLinkedIssue struct {
	Key    string                 `json:"key"`
	Fields jiraLinkedIssueFields `json:"fields"`
}

func (i *jiraLinkedIssue) statusName() string {
	if i != nil && i.Fields.Status != nil {
		return i.Fields.Status.Name
	}
	return ""
}

type jiraLinkedIssueFields struct {
	Summary string         `json:"summary"`
	Status  *jiraNamedItem `json:"status"`
}

type jiraErrorResponse struct {
	ErrorMessages []string          `json:"errorMessages"`
	Errors        map[string]string `json:"errors"`
}

// GetTicket fetches ticket details from Jira REST API by key.
func (c *HTTPClient) GetTicket(ctx context.Context, key string) (*Ticket, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("ticket key cannot be empty")
	}

	reqURL := fmt.Sprintf("%s/rest/api/2/issue/%s", c.baseURL, url.PathEscape(key))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira connection failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode == http.StatusBadRequest {
		errDetail := c.parseErrorMessage(bodyBytes)
		if errDetail != "" {
			return nil, fmt.Errorf("invalid jira ticket key %q (HTTP 400): %s", key, errDetail)
		}
		return nil, fmt.Errorf("invalid jira ticket key %q (HTTP 400)", key)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		errDetail := c.parseErrorMessage(bodyBytes)
		if errDetail != "" {
			return nil, fmt.Errorf("jira authentication failed (HTTP %d): %s", resp.StatusCode, errDetail)
		}
		return nil, fmt.Errorf("jira authentication failed (HTTP %d): check configured token or pat for %s", resp.StatusCode, c.baseURL)
	}

	if resp.StatusCode == http.StatusNotFound {
		errDetail := c.parseErrorMessage(bodyBytes)
		if errDetail != "" {
			return nil, fmt.Errorf("jira ticket %q not found (HTTP 404): %s", key, errDetail)
		}
		return nil, fmt.Errorf("jira ticket %q not found (HTTP 404): verify ticket key and permissions", key)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errDetail := c.parseErrorMessage(bodyBytes)
		if errDetail != "" {
			return nil, fmt.Errorf("jira API request failed (HTTP %d): %s", resp.StatusCode, errDetail)
		}
		return nil, fmt.Errorf("jira API request failed (HTTP %d)", resp.StatusCode)
	}

	var issueResp jiraIssueResponse
	if err := json.Unmarshal(bodyBytes, &issueResp); err != nil {
		return nil, fmt.Errorf("failed to decode jira response: %w", err)
	}

	ticket := &Ticket{
		Key:         issueResp.Key,
		Summary:     issueResp.Fields.Summary,
		Description: parseRawStringOrADF(issueResp.Fields.Description),
	}

	if issueResp.Fields.Status != nil {
		ticket.Status = issueResp.Fields.Status.Name
	}
	if issueResp.Fields.IssueType != nil {
		ticket.IssueType = issueResp.Fields.IssueType.Name
	}

	if issueResp.Fields.Comment != nil {
		for _, com := range issueResp.Fields.Comment.Comments {
			author := com.Author.DisplayName
			if author == "" {
				author = com.Author.Name
			}
			createdTime, _ := parseJiraTime(com.Created)
			ticket.Comments = append(ticket.Comments, Comment{
				ID:      com.ID,
				Author:  author,
				Body:    parseRawStringOrADF(com.Body),
				Created: createdTime,
			})
		}
	}

	for _, link := range issueResp.Fields.IssueLinks {
		if link.OutwardIssue != nil {
			rel := link.Type.Outward
			if rel == "" {
				rel = link.Type.Name
			}
			ticket.IssueLinks = append(ticket.IssueLinks, IssueLink{
				Relationship: rel,
				Key:          link.OutwardIssue.Key,
				Summary:      link.OutwardIssue.Fields.Summary,
				Status:       link.OutwardIssue.statusName(),
			})
		}
		if link.InwardIssue != nil {
			rel := link.Type.Inward
			if rel == "" {
				rel = link.Type.Name
			}
			ticket.IssueLinks = append(ticket.IssueLinks, IssueLink{
				Relationship: rel,
				Key:          link.InwardIssue.Key,
				Summary:      link.InwardIssue.Fields.Summary,
				Status:       link.InwardIssue.statusName(),
			})
		}
	}

	return ticket, nil
}

type jiraSearchResponse struct {
	StartAt    int                 `json:"startAt"`
	MaxResults int                 `json:"maxResults"`
	Total      int                 `json:"total"`
	Issues     []jiraSearchIssue   `json:"issues"`
}

type jiraSearchIssue struct {
	Key    string                 `json:"key"`
	Fields jiraSearchIssueFields `json:"fields"`
}

type jiraSearchIssueFields struct {
	Summary  string         `json:"summary"`
	Status   *jiraNamedItem `json:"status"`
	Assignee *jiraAssignee  `json:"assignee"`
}

type jiraAssignee struct {
	DisplayName string `json:"displayName"`
	Name        string `json:"name"`
}

// GetRecentTickets queries Jira for recent unresolved tickets in a project.
func (c *HTTPClient) GetRecentTickets(ctx context.Context, project string) ([]RecentTicket, error) {
	project = strings.TrimSpace(project)
	if project == "" {
		return nil, fmt.Errorf("project cannot be empty")
	}

	jql := fmt.Sprintf("project = %q AND resolution = Unresolved ORDER BY updated DESC", project)
	params := url.Values{}
	params.Set("jql", jql)
	params.Set("fields", "key,summary,status,assignee")
	params.Set("maxResults", "50")

	reqURL := fmt.Sprintf("%s/rest/api/2/search?%s", c.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira connection failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode == http.StatusBadRequest {
		errDetail := c.parseErrorMessage(bodyBytes)
		if errDetail != "" {
			return nil, fmt.Errorf("invalid jira search query (HTTP 400): %s", errDetail)
		}
		return nil, fmt.Errorf("invalid jira search query (HTTP 400)")
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		errDetail := c.parseErrorMessage(bodyBytes)
		if errDetail != "" {
			return nil, fmt.Errorf("jira authentication failed (HTTP %d): %s", resp.StatusCode, errDetail)
		}
		return nil, fmt.Errorf("jira authentication failed (HTTP %d): check configured token or pat for %s", resp.StatusCode, c.baseURL)
	}

	if resp.StatusCode == http.StatusNotFound {
		errDetail := c.parseErrorMessage(bodyBytes)
		if errDetail != "" {
			return nil, fmt.Errorf("jira resource not found (HTTP 404): %s", errDetail)
		}
		return nil, fmt.Errorf("jira resource not found (HTTP 404)")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errDetail := c.parseErrorMessage(bodyBytes)
		if errDetail != "" {
			return nil, fmt.Errorf("jira API request failed (HTTP %d): %s", resp.StatusCode, errDetail)
		}
		return nil, fmt.Errorf("jira API request failed (HTTP %d)", resp.StatusCode)
	}

	var searchResp jiraSearchResponse
	if err := json.Unmarshal(bodyBytes, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode jira response: %w", err)
	}

	tickets := make([]RecentTicket, 0, len(searchResp.Issues))
	for _, issue := range searchResp.Issues {
		t := RecentTicket{
			Key:     issue.Key,
			Summary: issue.Fields.Summary,
		}
		if issue.Fields.Status != nil {
			t.Status = issue.Fields.Status.Name
		}
		if issue.Fields.Assignee != nil {
			if issue.Fields.Assignee.DisplayName != "" {
				t.Assignee = issue.Fields.Assignee.DisplayName
			} else {
				t.Assignee = issue.Fields.Assignee.Name
			}
		}
		tickets = append(tickets, t)
	}

	return tickets, nil
}

func (c *HTTPClient) parseErrorMessage(body []byte) string {
	var errResp jiraErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil {
		if len(errResp.ErrorMessages) > 0 {
			return strings.Join(errResp.ErrorMessages, "; ")
		}
		if len(errResp.Errors) > 0 {
			var msgs []string
			for k, v := range errResp.Errors {
				msgs = append(msgs, fmt.Sprintf("%s: %s", k, v))
			}
			return strings.Join(msgs, "; ")
		}
	}
	return strings.TrimSpace(string(body))
}

func parseRawStringOrADF(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	// Try unmarshaling as plain string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	// Otherwise, it could be ADF document structure or complex object
	var doc map[string]interface{}
	if err := json.Unmarshal(raw, &doc); err == nil {
		var sb strings.Builder
		extractADFText(doc, &sb)
		return strings.TrimSpace(sb.String())
	}

	return string(raw)
}

func extractADFText(node map[string]interface{}, sb *strings.Builder) {
	if text, ok := node["text"].(string); ok {
		sb.WriteString(text)
	}
	if content, ok := node["content"].([]interface{}); ok {
		for _, child := range content {
			if childMap, ok := child.(map[string]interface{}); ok {
				extractADFText(childMap, sb)
				nodeType, _ := childMap["type"].(string)
				if nodeType == "paragraph" || nodeType == "heading" {
					sb.WriteString("\n")
				}
			}
		}
	}
}

func parseJiraTime(timeStr string) (time.Time, error) {
	if timeStr == "" {
		return time.Time{}, nil
	}
	// Jira commonly uses "2026-03-01T10:00:00.000+0000" or RFC3339
	formats := []string{
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05.000Z0700",
		"2006-01-02T15:04:05.000Z07:00",
		time.RFC3339,
		"2006-01-02T15:04:05-0700",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, timeStr); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time: %s", timeStr)
}
