package ai

import (
	"context"

	"github.com/arxcruz/pr-review/pkg/gitprovider"
)

// ReviewRequest encapsulates all necessary context for an AI code review
type ReviewRequest struct {
	PR                 *gitprovider.PullRequest
	Diff               string
	Guidelines         string
	ProjectID          string  // Optional project identifier
	ProjectDescription string  // Optional project description / architecture context
	Model              string  // Optional model override
	Temperature        float64 // Optional temperature override
}

// ReviewResult encapsulates the output of an AI code review
type ReviewResult struct {
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	Summary     string `json:"summary"`
	Content     string `json:"content"`
	TokensUsed  int    `json:"tokens_used,omitempty"`
}

// PromptRequest encapsulates generic prompt inputs for AI generation
type PromptRequest struct {
	SystemPrompt string  `json:"system_prompt,omitempty"`
	UserPrompt   string  `json:"user_prompt"`
	Model        string  `json:"model,omitempty"`
	Temperature  float64 `json:"temperature,omitempty"`
}

// GenerateResult encapsulates the output of an AI generation
type GenerateResult struct {
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	Content    string `json:"content"`
	TokensUsed int    `json:"tokens_used,omitempty"`
}

// Engine represents an AI provider capable of reviewing code or generating text
type Engine interface {
	Name() string
	Review(ctx context.Context, req ReviewRequest) (*ReviewResult, error)
	Generate(ctx context.Context, req PromptRequest) (*GenerateResult, error)
}

