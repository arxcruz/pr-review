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

func extractJSONPayload(raw string) string {
	// If markdown code fences are present, find any block tagged with ```json or ``` that parses
	lower := strings.ToLower(raw)
	if idx := strings.Index(lower, "```json"); idx != -1 {
		after := raw[idx+7:]
		if end := strings.Index(after, "```"); end != -1 {
			return strings.TrimSpace(after[:end])
		}
	}

	// Find the outermost JSON array or object
	firstBracket := strings.Index(raw, "[")
	lastBracket := strings.LastIndex(raw, "]")
	if firstBracket != -1 && lastBracket != -1 && lastBracket > firstBracket {
		return strings.TrimSpace(raw[firstBracket : lastBracket+1])
	}

	firstBrace := strings.Index(raw, "{")
	lastBrace := strings.LastIndex(raw, "}")
	if firstBrace != -1 && lastBrace != -1 && lastBrace > firstBrace {
		return strings.TrimSpace(raw[firstBrace : lastBrace+1])
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
