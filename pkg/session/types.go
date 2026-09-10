package session

import (
	"fmt"
	"strings"
	"time"

	"github.com/arxcruz/pr-review/pkg/jira"
)

// Question represents a frontier question posed during refinement.
type Question struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Explanation    string   `json:"explanation,omitempty"`
	Options        []string `json:"options,omitempty"`
	Recommendation string   `json:"recommendation,omitempty"`
	Answer         string   `json:"answer,omitempty"`
}

// Round represents a completed round of frontier questions and answers.
type Round struct {
	Number     int        `json:"number"`
	Questions  []Question `json:"questions"`
	AnsweredAt *time.Time `json:"answered_at,omitempty"`
}

// DecompositionTask represents a decomposed task or story within an epic.
type DecompositionTask struct {
	ID                 string   `json:"id"`
	Key                string   `json:"key,omitempty"`
	Title              string   `json:"title"`
	Description        string   `json:"description,omitempty"`
	Type               string   `json:"type,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	DeliveryProject    string   `json:"delivery_project,omitempty"`
	DependsOn          []string `json:"depends_on,omitempty"`
}

// DecompositionEpic represents a decomposed epic containing tasks.
type DecompositionEpic struct {
	ID              string              `json:"id"`
	Key             string              `json:"key,omitempty"`
	Title           string              `json:"title"`
	Description     string              `json:"description,omitempty"`
	DeliveryProject string              `json:"delivery_project,omitempty"`
	Tasks           []DecompositionTask `json:"tasks,omitempty"`
}

// DecompositionTree represents the hierarchical breakdown of the strategic ticket.
type DecompositionTree struct {
	Epics []DecompositionEpic `json:"epics,omitempty"`
}

// Snapshot represents the persisted state of an active or completed refinement session.
type Snapshot struct {
	Version         int                `json:"version"`
	Key             string             `json:"key"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
	Status          string             `json:"status"`
	Ticket          jira.Ticket        `json:"ticket"`
	Rounds          []Round            `json:"rounds,omitempty"`
	CurrentFrontier []Question         `json:"current_frontier,omitempty"`
	Tree            *DecompositionTree `json:"tree,omitempty"`
}

const (
	// StatusNew represents an initialized session snapshot before refinement begins.
	StatusNew = "new"
	// StatusInProgress represents a session actively undergoing frontier question refinement.
	StatusInProgress = "in-progress"
	// StatusFinalized represents a completed session where all frontier questions have been resolved.
	StatusFinalized = "finalized"
)

// AddUserRequirement appends an answered requirement round and advances the snapshot.
func (s *Snapshot) AddUserRequirement(reqText string) {
	if s == nil {
		return
	}
	trimmed := strings.TrimSpace(reqText)
	if trimmed == "" {
		return
	}
	s.CurrentFrontier = []Question{
		{
			ID:             fmt.Sprintf("USER-REQ-%d", len(s.Rounds)+1),
			Title:          "Additional User Requirement",
			Explanation:    "User provided extra architectural requirements or context",
			Recommendation: trimmed,
			Answer:         trimmed,
		},
	}
	s.AdvanceRound()
}

// AdvanceRound archives current frontier questions into a completed Round and clears CurrentFrontier.
func (s *Snapshot) AdvanceRound() {
	if s == nil || len(s.CurrentFrontier) == 0 {
		return
	}
	now := time.Now().UTC()
	round := Round{
		Number:     len(s.Rounds) + 1,
		Questions:  s.CurrentFrontier,
		AnsweredAt: &now,
	}
	s.Rounds = append(s.Rounds, round)
	s.CurrentFrontier = nil
	s.UpdatedAt = now
}

