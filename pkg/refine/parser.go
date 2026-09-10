package refine

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/arxcruz/pr-review/pkg/session"
)

// ParseQuestions extracts and normalizes structured frontier questions from an LLM response.
func ParseQuestions(raw string) ([]session.Question, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []session.Question{}, nil
	}

	cleaned := extractJSONPayload(trimmed)

	var rawList []session.Question
	if err := json.Unmarshal([]byte(cleaned), &rawList); err != nil {
		// Attempt object wrapping if LLM returned a single question object or { "questions": [...] }
		type wrapper struct {
			Questions []session.Question `json:"questions"`
		}
		var wrap wrapper
		if errWrap := json.Unmarshal([]byte(cleaned), &wrap); errWrap == nil && len(wrap.Questions) > 0 {
			rawList = wrap.Questions
		} else {
			var single session.Question
			if errSingle := json.Unmarshal([]byte(cleaned), &single); errSingle == nil && single.Title != "" {
				rawList = []session.Question{single}
			} else {
				return nil, fmt.Errorf("failed to parse frontier questions json: %w", err)
			}
		}
	}

	questions := make([]session.Question, 0, len(rawList))
	for i, q := range rawList {
		id := strings.TrimSpace(q.ID)
		if id == "" {
			id = fmt.Sprintf("Q%d", i+1)
		}

		title := strings.TrimSpace(q.Title)
		if title == "" {
			title = id
		}

		explanation := strings.TrimSpace(q.Explanation)
		recommendation := strings.TrimSpace(q.Recommendation)

		var options []string
		for _, opt := range q.Options {
			optTrimmed := strings.TrimSpace(opt)
			if optTrimmed != "" {
				options = append(options, optTrimmed)
			}
		}

		questions = append(questions, session.Question{
			ID:             id,
			Title:          title,
			Explanation:    explanation,
			Options:        options,
			Recommendation: recommendation,
			Answer:         strings.TrimSpace(q.Answer),
		})
	}

	return questions, nil
}

// ValidateDecompositionTree validates a decomposition tree against structural and dependency rules.
func ValidateDecompositionTree(tree *session.DecompositionTree) error {
	if tree == nil {
		return fmt.Errorf("decomposition tree is nil")
	}
	return tree.Validate()
}

// ParseDecompositionTree extracts and validates a DecompositionTree from an LLM response string.
func ParseDecompositionTree(raw string) (*session.DecompositionTree, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("empty response for decomposition tree")
	}

	cleaned := extractJSONPayload(trimmed)

	var tree session.DecompositionTree
	if err := json.Unmarshal([]byte(cleaned), &tree); err != nil {
		// If LLM returned an array of epics directly [ { "id": "EPIC-1", ... } ]
		var epics []session.DecompositionEpic
		if errArray := json.Unmarshal([]byte(cleaned), &epics); errArray == nil && len(epics) > 0 {
			tree.Epics = epics
		} else {
			return nil, fmt.Errorf("failed to parse decomposition tree JSON: %w", err)
		}
	}

	if err := ValidateDecompositionTree(&tree); err != nil {
		return nil, fmt.Errorf("invalid decomposition tree: %w", err)
	}

	return &tree, nil
}

func extractJSONPayload(raw string) string {
	// If markdown code fences are present, find any block tagged with ```json or ``` that parses
	lower := strings.ToLower(raw)
	if idx := strings.Index(lower, "```json"); idx != -1 {
		after := raw[idx+7:]
		if end := strings.Index(after, "```"); end != -1 {
			return strings.TrimSpace(after[:end])
		}
	}

	// Find outermost JSON object or array based on whichever begins first
	firstBrace := strings.Index(raw, "{")
	firstBracket := strings.Index(raw, "[")

	if firstBrace != -1 && (firstBracket == -1 || firstBrace < firstBracket) {
		lastBrace := strings.LastIndex(raw, "}")
		if lastBrace > firstBrace {
			return strings.TrimSpace(raw[firstBrace : lastBrace+1])
		}
	} else if firstBracket != -1 && (firstBrace == -1 || firstBracket < firstBrace) {
		lastBracket := strings.LastIndex(raw, "]")
		if lastBracket > firstBracket {
			return strings.TrimSpace(raw[firstBracket : lastBracket+1])
		}
	}

	if strings.Contains(raw, "```") {
		start := strings.Index(raw, "```")
		after := raw[start+3:]
		if end := strings.Index(after, "```"); end != -1 {
			return strings.TrimSpace(after[:end])
		}
	}

	return raw
}
