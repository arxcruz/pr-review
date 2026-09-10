package ai

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/arxcruz/pr-review/pkg/config"
)

type GeminiEngine struct {
	cfg config.GeminiConfig
}

func NewGeminiEngine(cfg config.GeminiConfig) *GeminiEngine {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://generativelanguage.googleapis.com"
	}
	if cfg.Model == "" {
		cfg.Model = "gemini-2.5-flash"
	}
	return &GeminiEngine{cfg: cfg}
}

func (g *GeminiEngine) Name() string {
	return "gemini"
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  *struct {
		Temperature float64 `json:"temperature,omitempty"`
	} `json:"generationConfig,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
			Role  string       `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata,omitempty"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

func (g *GeminiEngine) Review(ctx context.Context, req ReviewRequest) (*ReviewResult, error) {
	sysPrompt, userPrompt := BuildReviewPrompt(req)
	genRes, err := g.Generate(ctx, PromptRequest{
		SystemPrompt: sysPrompt,
		UserPrompt:   userPrompt,
		Model:        req.Model,
		Temperature:  req.Temperature,
	})
	if err != nil {
		return nil, err
	}
	return &ReviewResult{
		Provider:   genRes.Provider,
		Model:      genRes.Model,
		Content:    genRes.Content,
		TokensUsed: genRes.TokensUsed,
	}, nil
}

func (g *GeminiEngine) Generate(ctx context.Context, req PromptRequest) (*GenerateResult, error) {
	if g.cfg.APIKey == "" {
		return nil, fmt.Errorf("gemini API key is missing. Set it in config.yaml or GEMINI_API_KEY environment variable")
	}

	model := g.cfg.Model
	if req.Model != "" {
		model = req.Model
	}

	temp := g.cfg.Temperature
	if req.Temperature > 0 {
		temp = req.Temperature
	}

	bodyReq := geminiRequest{
		Contents: []geminiContent{
			{
				Role:  "user",
				Parts: []geminiPart{{Text: req.UserPrompt}},
			},
		},
		GenerationConfig: &struct {
			Temperature float64 `json:"temperature,omitempty"`
		}{
			Temperature: temp,
		},
	}

	if req.SystemPrompt != "" {
		bodyReq.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: req.SystemPrompt}},
		}
	}

	cleanBase := strings.TrimRight(g.cfg.BaseURL, "/")
	cleanModel := strings.TrimPrefix(model, "models/")
	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", cleanBase, cleanModel, url.QueryEscape(g.cfg.APIKey))

	var geminiResp geminiResponse
	if err := doJSONPost(ctx, endpoint, nil, bodyReq, &geminiResp); err != nil {
		return nil, fmt.Errorf("gemini: %w", err)
	}

	if geminiResp.Error != nil {
		return nil, fmt.Errorf("gemini error (%s): %s", geminiResp.Error.Status, geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("gemini returned no candidate responses")
	}

	var sb strings.Builder
	for _, p := range geminiResp.Candidates[0].Content.Parts {
		sb.WriteString(p.Text)
	}

	tokens := 0
	if geminiResp.UsageMetadata != nil {
		tokens = geminiResp.UsageMetadata.TotalTokenCount
	}

	return &GenerateResult{
		Provider:   "gemini",
		Model:      model,
		Content:    strings.TrimSpace(sb.String()),
		TokensUsed: tokens,
	}, nil
}

