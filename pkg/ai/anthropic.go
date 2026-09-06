package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/arxcruz/pr-review/pkg/config"
)

type AnthropicEngine struct {
	cfg config.AnthropicConfig
}

func NewAnthropicEngine(cfg config.AnthropicConfig) *AnthropicEngine {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.anthropic.com"
	}
	if cfg.Model == "" {
		cfg.Model = "claude-3-7-sonnet-20250219"
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 4096
	}
	return &AnthropicEngine{cfg: cfg}
}

func (a *AnthropicEngine) Name() string {
	return "anthropic"
}

type anthropicMessageRequest struct {
	Model       string                  `json:"model"`
	MaxTokens   int                     `json:"max_tokens"`
	System      string                  `json:"system,omitempty"`
	Messages    []anthropicMessage      `json:"messages"`
	Temperature float64                 `json:"temperature,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (a *AnthropicEngine) Review(ctx context.Context, req ReviewRequest) (*ReviewResult, error) {
	if a.cfg.APIKey == "" {
		return nil, fmt.Errorf("anthropic API key is missing. Set it in config.yaml or ANTHROPIC_API_KEY environment variable")
	}

	model := a.cfg.Model
	if req.Model != "" {
		model = req.Model
	}

	temp := a.cfg.Temperature
	if req.Temperature > 0 {
		temp = req.Temperature
	}

	sysPrompt, userPrompt := BuildReviewPrompt(req)

	bodyReq := anthropicMessageRequest{
		Model:       model,
		MaxTokens:   a.cfg.MaxTokens,
		System:      sysPrompt,
		Temperature: temp,
		Messages: []anthropicMessage{
			{Role: "user", Content: userPrompt},
		},
	}

	payload, err := json.Marshal(bodyReq)
	if err != nil {
		return nil, fmt.Errorf("failed to encode anthropic request: %w", err)
	}

	endpoint := strings.TrimRight(a.cfg.BaseURL, "/") + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create anthropic http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 15 * time.Minute}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to anthropic at %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read anthropic response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic API returned error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse anthropic response: %w", err)
	}

	if anthropicResp.Error != nil {
		return nil, fmt.Errorf("anthropic error (%s): %s", anthropicResp.Error.Type, anthropicResp.Error.Message)
	}

	var sb strings.Builder
	for _, block := range anthropicResp.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}

	return &ReviewResult{
		Provider:   "anthropic",
		Model:      model,
		Content:    strings.TrimSpace(sb.String()),
		TokensUsed: anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
	}, nil
}
