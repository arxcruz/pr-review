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

type OllamaEngine struct {
	cfg config.OllamaConfig
}

func NewOllamaEngine(cfg config.OllamaConfig) *OllamaEngine {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:11434"
	}
	if cfg.Model == "" {
		cfg.Model = "qwen2.5-coder:latest"
	}
	return &OllamaEngine{cfg: cfg}
}

func (o *OllamaEngine) Name() string {
	return "ollama"
}

type ollamaChatRequest struct {
	Model    string               `json:"model"`
	Messages []ollamaChatMessage  `json:"messages"`
	Stream   bool                 `json:"stream"`
	Options  *ollamaChatOptions   `json:"options,omitempty"`
}

type ollamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
}

type ollamaChatResponse struct {
	Model      string            `json:"model"`
	Message    ollamaChatMessage `json:"message"`
	Done       bool              `json:"done"`
	TotalNano  int64             `json:"total_duration"`
	PromptEval int               `json:"prompt_eval_count"`
	EvalCount  int               `json:"eval_count"`
	Error      string            `json:"error,omitempty"`
}

func (o *OllamaEngine) Review(ctx context.Context, req ReviewRequest) (*ReviewResult, error) {
	model := o.cfg.Model
	if req.Model != "" {
		model = req.Model
	}

	temp := o.cfg.Temperature
	if req.Temperature > 0 {
		temp = req.Temperature
	}

	sysPrompt, userPrompt := BuildReviewPrompt(req)

	bodyReq := ollamaChatRequest{
		Model:  model,
		Stream: false,
		Messages: []ollamaChatMessage{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: userPrompt},
		},
		Options: &ollamaChatOptions{
			Temperature: temp,
		},
	}

	payload, err := json.Marshal(bodyReq)
	if err != nil {
		return nil, fmt.Errorf("failed to encode ollama request: %w", err)
	}

	endpoint := strings.TrimRight(o.cfg.BaseURL, "/") + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create ollama http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Minute}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ollama at %s: %w", o.cfg.BaseURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read ollama response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp ollamaChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse ollama json response: %w", err)
	}

	if chatResp.Error != "" {
		return nil, fmt.Errorf("ollama response error: %s", chatResp.Error)
	}

	return &ReviewResult{
		Provider:   "ollama",
		Model:      model,
		Content:    strings.TrimSpace(chatResp.Message.Content),
		TokensUsed: chatResp.PromptEval + chatResp.EvalCount,
	}, nil
}
