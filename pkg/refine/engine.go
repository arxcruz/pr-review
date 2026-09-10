package refine

import (
	"context"
	"fmt"
	"time"

	"github.com/arxcruz/pr-review/pkg/ai"
	"github.com/arxcruz/pr-review/pkg/session"
)

// FrontierOptions defines tuning parameters for frontier generation.
type FrontierOptions struct {
	DocContext  string
	Guidelines  string
	Model       string
	Temperature float64
}

// Engine drives the interactive ticket refinement process using AI.
type Engine struct {
	aiEngine ai.Engine
}

// NewEngine creates a new refinement engine backed by the provided AI engine.
func NewEngine(aiEngine ai.Engine) *Engine {
	return &Engine{aiEngine: aiEngine}
}

// GenerateFrontier analyzes the ticket, doc context, and past rounds to formulate questions
// and updates the active session snapshot's CurrentFrontier and metadata.
func (e *Engine) GenerateFrontier(ctx context.Context, snap *session.Snapshot, opts FrontierOptions) ([]session.Question, error) {
	if snap == nil {
		return nil, fmt.Errorf("session snapshot is required")
	}

	sysPrompt, userPrompt := BuildFrontierPrompt(snap.Ticket, snap.Rounds, opts.DocContext, opts.Guidelines)

	genRes, err := e.aiEngine.Generate(ctx, ai.PromptRequest{
		SystemPrompt: sysPrompt,
		UserPrompt:   userPrompt,
		Model:        opts.Model,
		Temperature:  opts.Temperature,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate frontier questions via %s: %w", e.aiEngine.Name(), err)
	}

	questions, err := ParseQuestions(genRes.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	now := time.Now().UTC()
	snap.CurrentFrontier = questions
	snap.UpdatedAt = now
	if snap.Status == "" || snap.Status == "new" {
		snap.Status = "in-progress"
	}

	return questions, nil
}

// AdvanceRound commits answered CurrentFrontier questions into a new snapshot Round
// and resets CurrentFrontier for subsequent refinement.
func AdvanceRound(snap *session.Snapshot) {
	if snap != nil {
		snap.AdvanceRound()
	}
}
