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

