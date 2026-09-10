package session

import (
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
