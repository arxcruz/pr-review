package jira

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arxcruz/pr-review/pkg/config"
)

func TestClient_GetTicket_PAT_Success(t *testing.T) {
	mockResponse := map[string]interface{}{
		"key": "STRAT-123",
		"fields": map[string]interface{}{
			"summary":     "Strategic Initiative for Modernization",
			"description": "Full description of strategic initiative.",
			"status": map[string]interface{}{
				"name": "In Progress",
			},
			"issuetype": map[string]interface{}{
				"name": "Initiative",
			},
			"comment": map[string]interface{}{
				"comments": []map[string]interface{}{
					{
						"id":      "1001",
						"author":  map[string]interface{}{"displayName": "Jane Doe"},
						"body":    "Initial discussion comment.",
						"created": "2026-03-01T10:00:00.000+0000",
					},
				},
			},
			"issuelinks": []map[string]interface{}{
				{
					"id": "2001",
					"type": map[string]interface{}{
						"name":    "Blocks",
						"inward":  "is blocked by",
						"outward": "blocks",
					},
					"outwardIssue": map[string]interface{}{
						"key": "DELIV-456",
						"fields": map[string]interface{}{
							"summary": "Backend implementation task",
							"status":  map[string]interface{}{"name": "To Do"},
						},
					},
				},
				{
					"id": "2002",
					"type": map[string]interface{}{
						"name":    "Relates",
						"inward":  "relates to",
						"outward": "relates to",
					},
					"inwardIssue": map[string]interface{}{
						"key": "ARCH-789",
						"fields": map[string]interface{}{
							"summary": "Architecture decision doc",
							"status":  map[string]interface{}{"name": "Done"},
						},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/issue/STRAT-123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-pat" {
			t.Errorf("expected Authorization: Bearer test-pat, got: %s", authHeader)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ticket, err := client.GetTicket(context.Background(), "STRAT-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ticket.Key != "STRAT-123" {
		t.Errorf("expected Key STRAT-123, got %s", ticket.Key)
	}
	if ticket.Summary != "Strategic Initiative for Modernization" {
		t.Errorf("expected Summary, got %s", ticket.Summary)
	}
	if ticket.Description != "Full description of strategic initiative." {
		t.Errorf("expected Description, got %s", ticket.Description)
	}
	if ticket.Status != "In Progress" {
		t.Errorf("expected Status In Progress, got %s", ticket.Status)
	}
	if ticket.IssueType != "Initiative" {
		t.Errorf("expected IssueType Initiative, got %s", ticket.IssueType)
	}
	if len(ticket.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(ticket.Comments))
	}
	if ticket.Comments[0].Author != "Jane Doe" || ticket.Comments[0].Body != "Initial discussion comment." {
		t.Errorf("unexpected comment details: %+v", ticket.Comments[0])
	}
	if len(ticket.IssueLinks) != 2 {
		t.Fatalf("expected 2 issue links, got %d", len(ticket.IssueLinks))
	}
	if ticket.IssueLinks[0].Key != "DELIV-456" || ticket.IssueLinks[0].Relationship != "blocks" {
		t.Errorf("unexpected issue link 0: %+v", ticket.IssueLinks[0])
	}
	if ticket.IssueLinks[1].Key != "ARCH-789" || ticket.IssueLinks[1].Relationship != "relates to" {
		t.Errorf("unexpected issue link 1: %+v", ticket.IssueLinks[1])
	}
}

func TestClient_GetTicket_BasicAuth_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("user@example.com:api-token"))
		if authHeader != expected {
			t.Errorf("expected Basic auth header %s, got: %s", expected, authHeader)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		mockResp := map[string]interface{}{
			"key": "STRAT-100",
			"fields": map[string]interface{}{
				"summary": "Basic Auth Ticket",
				"status":  map[string]interface{}{"name": "Open"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResp)
	}))
	defer server.Close()

	cfg := config.JiraConfig{
		URL:   server.URL,
		Token: "user@example.com:api-token",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ticket, err := client.GetTicket(context.Background(), "STRAT-100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ticket.Key != "STRAT-100" || ticket.Summary != "Basic Auth Ticket" {
		t.Errorf("unexpected ticket: %+v", ticket)
	}
}

func TestClient_NewClient_BasicAuthWithUserField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("user@example.com:api-token"))
		if authHeader != expected {
			t.Errorf("expected Basic auth header %s, got: %s", expected, authHeader)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		mockResp := map[string]interface{}{
			"key": "STRAT-100",
			"fields": map[string]interface{}{
				"summary": "Basic Auth Ticket with User Field",
				"status":  map[string]interface{}{"name": "Open"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResp)
	}))
	defer server.Close()

	cfg := config.JiraConfig{
		URL:   server.URL,
		User:  "user@example.com",
		Token: "api-token",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ticket, err := client.GetTicket(context.Background(), "STRAT-100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ticket.Summary != "Basic Auth Ticket with User Field" {
		t.Errorf("unexpected summary: %s", ticket.Summary)
	}
}

func TestClient_NewClient_AtlassianTokenMissingUser(t *testing.T) {
	cfg := config.JiraConfig{
		URL:   "https://test.atlassian.net",
		Token: "ATATT3xFfGF0BtmdK9fOINFDhNr...",
	}

	_, err := NewClient(cfg)
	if err == nil {
		t.Fatal("expected error for ATATT token without user, got nil")
	}
	if !strings.Contains(err.Error(), "Atlassian Cloud API token requires user/email") {
		t.Errorf("expected helpful Atlassian token error, got: %v", err)
	}
}

func TestClient_GetTicket_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errorMessages":["Authentication failed. Please verify your credentials."]}`))
	}))
	defer server.Close()

	cfg := config.JiraConfig{
		URL: server.URL,
		PAT: "invalid-pat",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.GetTicket(context.Background(), "STRAT-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") && !strings.Contains(err.Error(), "401") {
		t.Errorf("expected actionable authentication error, got: %v", err)
	}
}

func TestClient_GetTicket_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorMessages":["Issue Does Not Exist"]}`))
	}))
	defer server.Close()

	cfg := config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.GetTicket(context.Background(), "NONEXISTENT-999")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "404") {
		t.Errorf("expected not found error message, got: %v", err)
	}
}

func TestClient_GetTicket_InvalidKey_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errorMessages":["The issue key 'INVALID' is not valid."]}`))
	}))
	defer server.Close()

	cfg := config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.GetTicket(context.Background(), "INVALID")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid jira ticket key") && !strings.Contains(err.Error(), "400") {
		t.Errorf("expected invalid key error message, got: %v", err)
	}
}


func TestClient_GetTicket_ADFDescription_Success(t *testing.T) {
	mockResponse := map[string]interface{}{
		"key": "STRAT-ADF",
		"fields": map[string]interface{}{
			"summary": "Ticket with ADF description",
			"description": map[string]interface{}{
				"version": 1,
				"type":    "doc",
				"content": []interface{}{
					map[string]interface{}{
						"type": "paragraph",
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": "First line of ADF description.",
							},
						},
					},
					map[string]interface{}{
						"type": "paragraph",
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": "Second line of ADF description.",
							},
						},
					},
				},
			},
			"status": map[string]interface{}{
				"name": "To Do",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ticket, err := client.GetTicket(context.Background(), "STRAT-ADF")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(ticket.Description, "First line of ADF description.") {
		t.Errorf("expected description to contain first line, got: %s", ticket.Description)
	}
	if !strings.Contains(ticket.Description, "Second line of ADF description.") {
		t.Errorf("expected description to contain second line, got: %s", ticket.Description)
	}
}

func TestClient_GetRecentTickets_Success(t *testing.T) {
	mockResponse := map[string]interface{}{
		"startAt":    0,
		"maxResults": 50,
		"total":      2,
		"issues": []map[string]interface{}{
			{
				"key": "STRAT-10",
				"fields": map[string]interface{}{
					"summary": "Implement Auth Subsystem",
					"status": map[string]interface{}{
						"name": "In Progress",
					},
					"assignee": map[string]interface{}{
						"displayName": "Alice Smith",
						"name":        "asmith",
					},
				},
			},
			{
				"key": "STRAT-11",
				"fields": map[string]interface{}{
					"summary": "Database Sharding Plan",
					"status": map[string]interface{}{
						"name": "Open",
					},
					"assignee": nil,
				},
			},
		},
	}

	var interceptedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		interceptedURL = r.URL.String()
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/rest/api/2/search") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	tickets, err := client.GetRecentTickets(context.Background(), "STRAT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(interceptedURL, "jql=project") || !strings.Contains(interceptedURL, "resolution+%3D+Unresolved") && !strings.Contains(interceptedURL, "resolution = Unresolved") && !strings.Contains(interceptedURL, "resolution%20%3D%20Unresolved") {
		t.Errorf("expected JQL with unresolved resolution, got URL: %s", interceptedURL)
	}

	if len(tickets) != 2 {
		t.Fatalf("expected 2 tickets, got %d", len(tickets))
	}

	if tickets[0].Key != "STRAT-10" || tickets[0].Summary != "Implement Auth Subsystem" || tickets[0].Status != "In Progress" || tickets[0].Assignee != "Alice Smith" {
		t.Errorf("unexpected ticket 0: %+v", tickets[0])
	}

	if tickets[1].Key != "STRAT-11" || tickets[1].Summary != "Database Sharding Plan" || tickets[1].Status != "Open" || tickets[1].Assignee != "" {
		t.Errorf("unexpected ticket 1: %+v", tickets[1])
	}
}

func TestClient_GetRecentTickets_EmptyProject(t *testing.T) {
	client, err := NewClient(config.JiraConfig{URL: "https://jira.example.com", Token: "abc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.GetRecentTickets(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty project, got nil")
	}
}

func TestClient_GetRecentTickets_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errorMessages":["Unauthorized"]}`))
	}))
	defer server.Close()

	client, err := NewClient(config.JiraConfig{URL: server.URL, Token: "bad"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.GetRecentTickets(context.Background(), "STRAT")
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("expected auth failure error, got: %v", err)
	}
}

func TestClient_CreateEpic_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/rest/api/2/issue" {
			t.Errorf("expected path /rest/api/2/issue, got %s", r.URL.Path)
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-pat" {
			t.Errorf("expected Authorization Bearer test-pat, got %s", authHeader)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		fields, ok := payload["fields"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected fields object in payload")
		}

		proj, _ := fields["project"].(map[string]interface{})
		if proj["key"] != "DELIV" {
			t.Errorf("expected project DELIV, got %v", proj["key"])
		}

		if fields["summary"] != "Delivery Infrastructure Epic" {
			t.Errorf("expected summary 'Delivery Infrastructure Epic', got %v", fields["summary"])
		}

		if fields["description"] != "Epic description for delivery team" {
			t.Errorf("expected description 'Epic description for delivery team', got %v", fields["description"])
		}

		issueType, _ := fields["issuetype"].(map[string]interface{})
		if issueType["name"] != "Epic" {
			t.Errorf("expected issuetype Epic, got %v", issueType["name"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "10100",
			"key":  "DELIV-10",
			"self": "https://jira.example.com/rest/api/2/issue/10100",
		})
	}))
	defer server.Close()

	cfg := config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	created, err := client.CreateEpic(context.Background(), CreateEpicRequest{
		Project:     "DELIV",
		Summary:     "Delivery Infrastructure Epic",
		Description: "Epic description for delivery team",
		IssueType:   "Epic",
	})
	if err != nil {
		t.Fatalf("unexpected error creating epic: %v", err)
	}

	if created.ID != "10100" {
		t.Errorf("expected ID 10100, got %s", created.ID)
	}
	if created.Key != "DELIV-10" {
		t.Errorf("expected Key DELIV-10, got %s", created.Key)
	}
	if created.Self != "https://jira.example.com/rest/api/2/issue/10100" {
		t.Errorf("expected Self https://jira.example.com/rest/api/2/issue/10100, got %s", created.Self)
	}
	expectedURL := server.URL + "/browse/DELIV-10"
	if created.URL != expectedURL {
		t.Errorf("expected URL %s, got %s", expectedURL, created.URL)
	}
}

func TestClient_CreateTask_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/rest/api/2/issue" {
			t.Errorf("expected path /rest/api/2/issue, got %s", r.URL.Path)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		fields, ok := payload["fields"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected fields object in payload")
		}

		proj, _ := fields["project"].(map[string]interface{})
		if proj["key"] != "CORE" {
			t.Errorf("expected project CORE, got %v", proj["key"])
		}

		if fields["summary"] != "Implement Database Connection Pool" {
			t.Errorf("expected summary, got %v", fields["summary"])
		}

		desc, _ := fields["description"].(string)
		if !strings.Contains(desc, "Initial pool implementation") {
			t.Errorf("expected description to contain body, got: %s", desc)
		}
		if !strings.Contains(desc, "Acceptance Criteria:") || !strings.Contains(desc, "- Connection pool respects max limit") {
			t.Errorf("expected description to contain acceptance criteria, got: %s", desc)
		}

		issueType, _ := fields["issuetype"].(map[string]interface{})
		if issueType["name"] != "Task" {
			t.Errorf("expected issuetype Task, got %v", issueType["name"])
		}

		parent, _ := fields["parent"].(map[string]interface{})
		if parent["key"] != "DELIV-10" {
			t.Errorf("expected parent key DELIV-10, got %v", parent["key"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "10101",
			"key":  "CORE-55",
			"self": "https://jira.example.com/rest/api/2/issue/10101",
		})
	}))
	defer server.Close()

	client, err := NewClient(config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	created, err := client.CreateTask(context.Background(), CreateTaskRequest{
		Project:     "CORE",
		Summary:     "Implement Database Connection Pool",
		Description: "Initial pool implementation",
		ParentKey:   "DELIV-10",
		AcceptanceCriteria: []string{
			"Connection pool respects max limit",
			"Idle connections timeout after 30 seconds",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error creating task: %v", err)
	}

	if created.ID != "10101" {
		t.Errorf("expected ID 10101, got %s", created.ID)
	}
	if created.Key != "CORE-55" {
		t.Errorf("expected Key CORE-55, got %s", created.Key)
	}
	if created.Self != "https://jira.example.com/rest/api/2/issue/10101" {
		t.Errorf("expected Self https://jira.example.com/rest/api/2/issue/10101, got %s", created.Self)
	}
	expectedURL := server.URL + "/browse/CORE-55"
	if created.URL != expectedURL {
		t.Errorf("expected URL %s, got %s", expectedURL, created.URL)
	}
}

func TestClient_CreateTask_Story(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		fields := payload["fields"].(map[string]interface{})

		issueType, _ := fields["issuetype"].(map[string]interface{})
		if issueType["name"] != "Story" {
			t.Errorf("expected issuetype Story, got %v", issueType["name"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "10102",
			"key":  "WEB-42",
			"self": "https://jira.example.com/rest/api/2/issue/10102",
		})
	}))
	defer server.Close()

	client, err := NewClient(config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	created, err := client.CreateTask(context.Background(), CreateTaskRequest{
		Project:   "WEB",
		Summary:   "Add login button",
		IssueType: "Story",
	})
	if err != nil {
		t.Fatalf("unexpected error creating story: %v", err)
	}

	if created.Key != "WEB-42" {
		t.Errorf("expected Key WEB-42, got %s", created.Key)
	}
}

func TestClient_CreateIssue_ValidationErrors(t *testing.T) {
	client, err := NewClient(config.JiraConfig{
		URL: "https://jira.example.com",
		PAT: "test-pat",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()

	// Missing project
	_, err = client.CreateIssue(ctx, CreateIssueRequest{
		Project:   "",
		Summary:   "Some summary",
		IssueType: "Task",
	})
	if err == nil || !strings.Contains(err.Error(), "project is required") {
		t.Errorf("expected 'project is required' error, got: %v", err)
	}

	// Missing summary
	_, err = client.CreateIssue(ctx, CreateIssueRequest{
		Project:   "DELIV",
		Summary:   "   ",
		IssueType: "Task",
	})
	if err == nil || !strings.Contains(err.Error(), "summary is required") {
		t.Errorf("expected 'summary is required' error, got: %v", err)
	}

	// Missing issue type
	_, err = client.CreateIssue(ctx, CreateIssueRequest{
		Project:   "DELIV",
		Summary:   "Some summary",
		IssueType: "",
	})
	if err == nil || !strings.Contains(err.Error(), "issue type is required") {
		t.Errorf("expected 'issue type is required' error, got: %v", err)
	}
}

func TestClient_CreateIssue_HTTP400(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"errorMessages": []string{"Project DELIV does not exist"},
			"errors": map[string]string{
				"summary": "Summary is too long",
			},
		})
	}))
	defer server.Close()

	client, err := NewClient(config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.CreateIssue(context.Background(), CreateIssueRequest{
		Project:   "DELIV",
		Summary:   "Invalid summary",
		IssueType: "Task",
	})
	if err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 400") || !strings.Contains(err.Error(), "Project DELIV does not exist") {
		t.Errorf("unexpected error format: %v", err)
	}
}

func TestClient_CreateIssue_HTTP401(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"errorMessages": []string{"Unauthorized token"},
		})
	}))
	defer server.Close()

	client, err := NewClient(config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.CreateIssue(context.Background(), CreateIssueRequest{
		Project:   "DELIV",
		Summary:   "Some summary",
		IssueType: "Task",
	})
	if err == nil {
		t.Fatal("expected error on HTTP 401, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") || !strings.Contains(err.Error(), "Unauthorized token") {
		t.Errorf("unexpected error format: %v", err)
	}
}

func TestClient_CreateIssueLink_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/rest/api/2/issueLink" {
			t.Errorf("expected path /rest/api/2/issueLink, got %s", r.URL.Path)
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-pat" {
			t.Errorf("expected Authorization Bearer test-pat, got %s", authHeader)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		linkType, ok := payload["type"].(map[string]interface{})
		if !ok || linkType["name"] != "Relates" {
			t.Errorf("expected link type Relates, got %v", linkType)
		}

		inward, ok := payload["inwardIssue"].(map[string]interface{})
		if !ok || inward["key"] != "ORIGIN-1" {
			t.Errorf("expected inwardIssue ORIGIN-1, got %v", inward)
		}

		outward, ok := payload["outwardIssue"].(map[string]interface{})
		if !ok || outward["key"] != "DELIV-10" {
			t.Errorf("expected outwardIssue DELIV-10, got %v", outward)
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := NewClient(config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	err = client.CreateIssueLink(context.Background(), CreateIssueLinkRequest{
		LinkType:   "Relates",
		InwardKey:  "ORIGIN-1",
		OutwardKey: "DELIV-10",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_CreateIssueLink_WithComment_AndCustomType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		linkType, _ := payload["type"].(map[string]interface{})
		if linkType["name"] != "implements" {
			t.Errorf("expected link type implements, got %v", linkType["name"])
		}

		comment, _ := payload["comment"].(map[string]interface{})
		if comment["body"] != "Linked during refinement" {
			t.Errorf("expected comment body 'Linked during refinement', got %v", comment["body"])
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	err = client.CreateIssueLink(context.Background(), CreateIssueLinkRequest{
		LinkType:   "implements",
		InwardKey:  "ORIGIN-1",
		OutwardKey: "DELIV-10",
		Comment:    "Linked during refinement",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_CreateIssueLink_ValidationErrors(t *testing.T) {
	client, err := NewClient(config.JiraConfig{
		URL: "https://jira.example.com",
		PAT: "test-pat",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()

	// Missing inward key
	err = client.CreateIssueLink(ctx, CreateIssueLinkRequest{
		LinkType:   "Relates",
		InwardKey:  "",
		OutwardKey: "DELIV-10",
	})
	if err == nil || !strings.Contains(err.Error(), "inward issue key is required") {
		t.Errorf("expected 'inward issue key is required' error, got: %v", err)
	}

	// Missing outward key
	err = client.CreateIssueLink(ctx, CreateIssueLinkRequest{
		LinkType:   "Relates",
		InwardKey:  "ORIGIN-1",
		OutwardKey: "",
	})
	if err == nil || !strings.Contains(err.Error(), "outward issue key is required") {
		t.Errorf("expected 'outward issue key is required' error, got: %v", err)
	}

	// Same key
	err = client.CreateIssueLink(ctx, CreateIssueLinkRequest{
		LinkType:   "Relates",
		InwardKey:  "ORIGIN-1",
		OutwardKey: "ORIGIN-1",
	})
	if err == nil || !strings.Contains(err.Error(), "cannot link an issue to itself") {
		t.Errorf("expected 'cannot link an issue to itself' error, got: %v", err)
	}
}

func TestClient_CreateIssueLink_HTTP400(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"errorMessages": []string{"Issue does not exist or you do not have permission to view it."},
		})
	}))
	defer server.Close()

	client, err := NewClient(config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	err = client.CreateIssueLink(context.Background(), CreateIssueLinkRequest{
		LinkType:   "Relates",
		InwardKey:  "NOTEXIST-1",
		OutwardKey: "DELIV-10",
	})
	if err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 400") || !strings.Contains(err.Error(), "Issue does not exist") {
		t.Errorf("unexpected error format: %v", err)
	}
}

func TestClient_CreateDependencyLink_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/rest/api/2/issueLink" {
			t.Errorf("expected path /rest/api/2/issueLink, got %s", r.URL.Path)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		linkType, ok := payload["type"].(map[string]interface{})
		if !ok || linkType["name"] != "Blocks" {
			t.Errorf("expected link type Blocks, got %v", linkType)
		}

		// blocker blocks blocked:
		// outwardIssue is blocker (blocks)
		// inwardIssue is blocked (is blocked by)
		inward, ok := payload["inwardIssue"].(map[string]interface{})
		if !ok || inward["key"] != "CORE-20" {
			t.Errorf("expected inwardIssue CORE-20 (blocked), got %v", inward)
		}

		outward, ok := payload["outwardIssue"].(map[string]interface{})
		if !ok || outward["key"] != "CORE-10" {
			t.Errorf("expected outwardIssue CORE-10 (blocker), got %v", outward)
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := NewClient(config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// CORE-20 is blocked by CORE-10 (CORE-10 is prerequisite/blocker)
	err = client.CreateDependencyLink(context.Background(), "CORE-20", "CORE-10")
	if err != nil {
		t.Fatalf("unexpected error creating dependency link: %v", err)
	}
}

func TestClient_CreateDependencyLink_ValidationErrors(t *testing.T) {
	client, err := NewClient(config.JiraConfig{
		URL: "https://jira.example.com",
		PAT: "test-pat",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()

	// Empty blocked key
	err = client.CreateDependencyLink(ctx, "", "CORE-10")
	if err == nil || !strings.Contains(err.Error(), "blocked issue key is required") {
		t.Errorf("expected 'blocked issue key is required' error, got: %v", err)
	}

	// Empty blocker key
	err = client.CreateDependencyLink(ctx, "CORE-20", "")
	if err == nil || !strings.Contains(err.Error(), "blocker issue key is required") {
		t.Errorf("expected 'blocker issue key is required' error, got: %v", err)
	}

	// Self dependency
	err = client.CreateDependencyLink(ctx, "CORE-20", "CORE-20")
	if err == nil || !strings.Contains(err.Error(), "cannot depend on itself") {
		t.Errorf("expected 'cannot depend on itself' error, got: %v", err)
	}
}

func TestClient_SetParentLink_StandardHierarchy_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT method, got %s", r.Method)
		}
		if r.URL.Path != "/rest/api/2/issue/DELIV-10" {
			t.Errorf("expected path /rest/api/2/issue/DELIV-10, got %s", r.URL.Path)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		fields, ok := payload["fields"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected fields in payload: %v", payload)
		}

		parent, ok := fields["parent"].(map[string]interface{})
		if !ok || parent["key"] != "ORIGIN-100" {
			t.Errorf("expected parent key ORIGIN-100, got %v", parent)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	err = client.SetParentLink(context.Background(), "DELIV-10", "ORIGIN-100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_SetParentLink_CustomPortfolioField_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT method, got %s", r.Method)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		fields := payload["fields"].(map[string]interface{})
		if fields["customfield_10014"] != "ORIGIN-100" {
			t.Errorf("expected customfield_10014 ORIGIN-100, got %v", fields["customfield_10014"])
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(config.JiraConfig{
		URL:             server.URL,
		PAT:             "test-pat",
		ParentLinkField: "customfield_10014",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	err = client.SetParentLink(context.Background(), "DELIV-10", "ORIGIN-100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_SetParentLink_HTTP400_Unsupported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"errorMessages": []string{"Field 'parent' cannot be set. It is not on the appropriate screen, or unknown."},
		})
	}))
	defer server.Close()

	client, err := NewClient(config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	err = client.SetParentLink(context.Background(), "DELIV-10", "ORIGIN-100")
	if err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 400") || !strings.Contains(err.Error(), "Field 'parent' cannot be set") {
		t.Errorf("unexpected error format: %v", err)
	}
}

func TestClient_SetParentLink_ValidationErrors(t *testing.T) {
	client, err := NewClient(config.JiraConfig{
		URL: "https://jira.example.com",
		PAT: "test-pat",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()

	// Missing child key
	err = client.SetParentLink(ctx, "", "ORIGIN-100")
	if err == nil || !strings.Contains(err.Error(), "child issue key is required") {
		t.Errorf("expected 'child issue key is required' error, got: %v", err)
	}

	// Missing parent key
	err = client.SetParentLink(ctx, "DELIV-10", "")
	if err == nil || !strings.Contains(err.Error(), "parent issue key is required") {
		t.Errorf("expected 'parent issue key is required' error, got: %v", err)
	}

	// Same key
	err = client.SetParentLink(ctx, "DELIV-10", "DELIV-10")
	if err == nil || !strings.Contains(err.Error(), "cannot be its own parent") {
		t.Errorf("expected 'cannot be its own parent' error, got: %v", err)
	}
}

func TestClient_LinkStrategicTicket_ParentLinkSupported(t *testing.T) {
	putCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/rest/api/2/issue/DELIV-10" {
			putCalled = true
			var payload map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			fields := payload["fields"].(map[string]interface{})
			parent := fields["parent"].(map[string]interface{})
			if parent["key"] != "ORIGIN-100" {
				t.Errorf("expected parent key ORIGIN-100, got %v", parent["key"])
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewClient(config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	result, err := client.LinkStrategicTicket(context.Background(), StrategicLinkRequest{
		ChildKey:  "DELIV-10",
		OriginKey: "ORIGIN-100",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !putCalled {
		t.Errorf("expected PUT to be called for parent link")
	}
	if result.Method != "parent" {
		t.Errorf("expected method 'parent', got %q", result.Method)
	}
	if result.ChildKey != "DELIV-10" || result.OriginKey != "ORIGIN-100" {
		t.Errorf("unexpected result keys: %+v", result)
	}
}

func TestClient_LinkStrategicTicket_FallbackToIssueLink(t *testing.T) {
	putCalled := false
	linkCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/rest/api/2/issue/DELIV-10" {
			putCalled = true
			// Simulate Jira instance without Portfolio hierarchy support (HTTP 400)
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errorMessages": []string{"Field 'parent' cannot be set for issue type Epic"},
			})
			return
		}

		if r.Method == http.MethodPost && r.URL.Path == "/rest/api/2/issueLink" {
			linkCalled = true
			var payload map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&payload)

			linkType, _ := payload["type"].(map[string]interface{})
			if linkType["name"] != "Relates" {
				t.Errorf("expected fallback link type Relates, got %v", linkType["name"])
			}

			inward, _ := payload["inwardIssue"].(map[string]interface{})
			if inward["key"] != "ORIGIN-100" {
				t.Errorf("expected inward ORIGIN-100, got %v", inward["key"])
			}

			outward, _ := payload["outwardIssue"].(map[string]interface{})
			if outward["key"] != "DELIV-10" {
				t.Errorf("expected outward DELIV-10, got %v", outward["key"])
			}

			w.WriteHeader(http.StatusCreated)
			return
		}

		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client, err := NewClient(config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	result, err := client.LinkStrategicTicket(context.Background(), StrategicLinkRequest{
		ChildKey:  "DELIV-10",
		OriginKey: "ORIGIN-100",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !putCalled {
		t.Errorf("expected PUT to be attempted first")
	}
	if !linkCalled {
		t.Errorf("expected fallback issueLink POST to be called")
	}
	if result.Method != "issue_link" {
		t.Errorf("expected method 'issue_link', got %q", result.Method)
	}
	if result.LinkType != "Relates" {
		t.Errorf("expected link type 'Relates', got %q", result.LinkType)
	}
}

func TestClient_LinkStrategicTicket_ConfiguredLinkTypeFallback(t *testing.T) {
	linkCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errorMessages": []string{"parent field not allowed"},
			})
			return
		}

		if r.Method == http.MethodPost && r.URL.Path == "/rest/api/2/issueLink" {
			linkCalled = true
			var payload map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&payload)

			linkType, _ := payload["type"].(map[string]interface{})
			if linkType["name"] != "implements" {
				t.Errorf("expected configured link type implements, got %v", linkType["name"])
			}

			w.WriteHeader(http.StatusCreated)
			return
		}

		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client, err := NewClient(config.JiraConfig{
		URL:      server.URL,
		PAT:      "test-pat",
		LinkType: "implements",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	result, err := client.LinkStrategicTicket(context.Background(), StrategicLinkRequest{
		ChildKey:  "DELIV-10",
		OriginKey: "ORIGIN-100",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !linkCalled {
		t.Errorf("expected issueLink POST")
	}
	if result.Method != "issue_link" || result.LinkType != "implements" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestClient_LinkStrategicTicket_ForceIssueLink(t *testing.T) {
	putCalled := false
	linkCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			putCalled = true
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == http.MethodPost && r.URL.Path == "/rest/api/2/issueLink" {
			linkCalled = true
			w.WriteHeader(http.StatusCreated)
			return
		}

		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client, err := NewClient(config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	result, err := client.LinkStrategicTicket(context.Background(), StrategicLinkRequest{
		ChildKey:       "DELIV-10",
		OriginKey:      "ORIGIN-100",
		LinkType:       "relates to",
		ForceIssueLink: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if putCalled {
		t.Errorf("expected PUT parent link to be skipped when ForceIssueLink is true")
	}
	if !linkCalled {
		t.Errorf("expected issueLink POST to be called")
	}
	if result.Method != "issue_link" || result.LinkType != "relates to" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestClient_LinkStrategicTicket_BothFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"errorMessages": []string{"Operation forbidden"},
		})
	}))
	defer server.Close()

	client, err := NewClient(config.JiraConfig{
		URL: server.URL,
		PAT: "test-pat",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.LinkStrategicTicket(context.Background(), StrategicLinkRequest{
		ChildKey:  "DELIV-10",
		OriginKey: "ORIGIN-100",
	})
	if err == nil {
		t.Fatal("expected error when both fail, got nil")
	}
	if !strings.Contains(err.Error(), "parent link failed") || !strings.Contains(err.Error(), "fallback issue link failed") {
		t.Errorf("expected combined descriptive error, got: %v", err)
	}
}

func TestClient_LinkStrategicTicket_ValidationErrors(t *testing.T) {
	client, err := NewClient(config.JiraConfig{
		URL: "https://jira.example.com",
		PAT: "test-pat",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()

	// Missing child key
	_, err = client.LinkStrategicTicket(ctx, StrategicLinkRequest{
		ChildKey:  "",
		OriginKey: "ORIGIN-100",
	})
	if err == nil || !strings.Contains(err.Error(), "child issue key is required") {
		t.Errorf("expected 'child issue key is required' error, got: %v", err)
	}

	// Missing origin key
	_, err = client.LinkStrategicTicket(ctx, StrategicLinkRequest{
		ChildKey:  "DELIV-10",
		OriginKey: "",
	})
	if err == nil || !strings.Contains(err.Error(), "origin strategic ticket key is required") {
		t.Errorf("expected 'origin strategic ticket key is required' error, got: %v", err)
	}

	// Same key
	_, err = client.LinkStrategicTicket(ctx, StrategicLinkRequest{
		ChildKey:  "ORIGIN-100",
		OriginKey: "ORIGIN-100",
	})
	if err == nil || !strings.Contains(err.Error(), "cannot link an issue to itself") {
		t.Errorf("expected 'cannot link an issue to itself' error, got: %v", err)
	}
}





