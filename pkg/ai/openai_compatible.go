package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/arxcruz/pr-review/pkg/config"
)

// OpenAICompatibleEngine handles providers that speak the OpenAI
// chat-completions wire format: ollama, vllm, llamacpp, openai, and others.
type OpenAICompatibleEngine struct {
	name string
	cfg  config.OpenAICompatibleConfig
}

func NewOpenAICompatibleEngine(name string, cfg config.OpenAICompatibleConfig) *OpenAICompatibleEngine {
	if name == "" {
		name = "openai"
	}
	return &OpenAICompatibleEngine{name: name, cfg: cfg}
}

func (o *OpenAICompatibleEngine) Name() string {
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

func (o *OpenAICompatibleEngine) Review(ctx context.Context, req ReviewRequest) (*ReviewResult, error) {
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

	endpoint := strings.TrimRight(o.cfg.BaseURL, "/")
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint = endpoint + "/chat/completions"
	}

	headers := map[string]string{}
	if o.cfg.APIKey != "" {
		headers["Authorization"] = "Bearer " + o.cfg.APIKey
	}

	var chatResp openAIChatResponse
	if err := doJSONPost(ctx, endpoint, headers, bodyReq, &chatResp); err != nil {
		return nil, fmt.Errorf("%s: %w", o.Name(), err)
	}

	if chatResp.Error != nil {
		return nil, fmt.Errorf("%s error: %s", o.Name(), chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("%s returned empty choices", o.Name())
	}

	return &ReviewResult{
		Provider:   o.Name(),
		Model:      model,
		Content:    strings.TrimSpace(chatResp.Choices[0].Message.Content),
		TokensUsed: chatResp.Usage.TotalTokens,
	}, nil
}
