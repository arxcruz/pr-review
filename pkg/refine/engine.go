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

// DecompositionOptions defines tuning parameters for decomposition tree generation.
type DecompositionOptions struct {
	DocContext  string
	Guidelines  string
	Model       string
	Temperature float64
	MaxRetries  int
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

// GenerateDecompositionTree prompts the AI engine to synthesize settled refinement rounds
// into a validated Decomposition Tree, retrying if the response is malformed JSON or invalid schema,
// and persists the resulting tree into the session snapshot.
func (e *Engine) GenerateDecompositionTree(ctx context.Context, snap *session.Snapshot, opts DecompositionOptions) (*session.DecompositionTree, error) {
	if snap == nil {
		return nil, fmt.Errorf("session snapshot is required")
	}

	maxRetries := opts.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 2
	}

	sysPrompt, baseUserPrompt := BuildDecompositionPrompt(snap.Ticket, snap.Rounds, opts.DocContext, opts.Guidelines)
	currentUserPrompt := baseUserPrompt

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		genRes, err := e.aiEngine.Generate(ctx, ai.PromptRequest{
			SystemPrompt: sysPrompt,
			UserPrompt:   currentUserPrompt,
			Model:        opts.Model,
			Temperature:  opts.Temperature,
		})
		if err != nil {
			return nil, fmt.Errorf("failed generating decomposition tree via %s: %w", e.aiEngine.Name(), err)
		}

		tree, parseErr := ParseDecompositionTree(genRes.Content)
		if parseErr == nil {
			now := time.Now().UTC()
			snap.Tree = tree
			snap.UpdatedAt = now
			return tree, nil
		}

		lastErr = parseErr
		if attempt < maxRetries {
			currentUserPrompt = fmt.Sprintf("%s\n\n### RETRY FEEDBACK\nYour previous response failed validation: %s\nPlease regenerate the decomposition tree adhering strictly to valid JSON and schema requirements.", baseUserPrompt, parseErr.Error())
		}
	}

	return nil, fmt.Errorf("failed to generate valid decomposition tree after %d retries: %w", maxRetries, lastErr)
}
