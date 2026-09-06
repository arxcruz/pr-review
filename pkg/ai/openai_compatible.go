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

type OpenAIEngine struct {
	name string
	cfg  config.OpenAIConfig
}

func NewOpenAIEngine(cfg config.OpenAIConfig) *OpenAIEngine {
	return NewOpenAICompatibleEngine("openai", cfg)
}

func NewOpenAICompatibleEngine(name string, cfg config.OpenAIConfig) *OpenAIEngine {
	if name == "" {
		name = "openai"
	}
	if cfg.BaseURL == "" {
		if name == "vllm" {
			cfg.BaseURL = "http://localhost:8000/v1"
		} else if name == "llamacpp" {
			cfg.BaseURL = "http://localhost:8080/v1"
		} else {
			cfg.BaseURL = "https://api.openai.com/v1"
		}
	}
	if cfg.Model == "" {
		if name == "vllm" || name == "llamacpp" {
			cfg.Model = "default"
		} else {
			cfg.Model = "gpt-4o"
		}
	}
	return &OpenAIEngine{name: name, cfg: cfg}
}

func (o *OpenAIEngine) Name() string {
	if o.name != "" {
		return o.name
	}
	return "openai"
}

type openAIChatRequest struct {
	Model       string              `json:"model"`
	Messages    []openAIChatMessage `json:"messages"`
	Temperature float64             `json:"temperature,omitempty"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func (o *OpenAIEngine) Review(ctx context.Context, req ReviewRequest) (*ReviewResult, error) {
	model := o.cfg.Model
	if req.Model != "" {
		model = req.Model
	}

	temp := o.cfg.Temperature
	if req.Temperature > 0 {
		temp = req.Temperature
	}

	sysPrompt, userPrompt := BuildReviewPrompt(req)

	bodyReq := openAIChatRequest{
		Model:       model,
		Temperature: temp,
		Messages: []openAIChatMessage{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	payload, err := json.Marshal(bodyReq)
	if err != nil {
		return nil, fmt.Errorf("failed to encode openai request: %w", err)
	}

	endpoint := strings.TrimRight(o.cfg.BaseURL, "/")
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint = endpoint + "/chat/completions"
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create openai http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if o.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+o.cfg.APIKey)
	}

	client := &http.Client{Timeout: 15 * time.Minute}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to openai compatible endpoint at %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read openai response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai API returned error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp openAIChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse openai json response: %w", err)
	}

	if chatResp.Error != nil {
		return nil, fmt.Errorf("openai error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("openai returned empty choices")
	}

	return &ReviewResult{
		Provider:   o.Name(),
		Model:      model,
		Content:    strings.TrimSpace(chatResp.Choices[0].Message.Content),
		TokensUsed: chatResp.Usage.TotalTokens,
	}, nil
}
