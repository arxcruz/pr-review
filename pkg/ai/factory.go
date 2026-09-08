package ai

import (
	"fmt"
	"strings"

	"github.com/arxcruz/pr-review/pkg/config"
)

// providerDefault holds the default BaseURL and Model for a provider.
type providerDefault struct {
	BaseURL string
	Model   string
}

// providerDefaults is the central defaults table for all known providers.
var providerDefaults = map[string]providerDefault{
	"ollama":    {BaseURL: "http://localhost:11434/v1", Model: "qwen2.5-coder:latest"},
	"vllm":     {BaseURL: "http://localhost:8000/v1", Model: "default"},
	"llamacpp": {BaseURL: "http://localhost:8080/v1", Model: "default"},
	"openai":   {BaseURL: "https://api.openai.com/v1", Model: "gpt-4o"},
	"anthropic": {BaseURL: "https://api.anthropic.com", Model: "claude-3-7-sonnet-20250219"},
	"gemini":   {BaseURL: "https://generativelanguage.googleapis.com", Model: "gemini-2.5-flash"},
}

type Factory struct {
	cfg *config.Config
}

func NewFactory(cfg *config.Config) *Factory {
	return &Factory{cfg: cfg}
}

// GetEngine returns the AI review engine for the requested provider name, target ID, or default provider
func (f *Factory) GetEngine(providerOrTargetID string) (Engine, error) {
	name := strings.TrimSpace(providerOrTargetID)
	if name == "" {
		name = strings.TrimSpace(f.cfg.DefaultAIProvider)
	}
	if name == "" {
		name = "ollama" // fallback
	}

	targets := config.GetAllAITargets(f.cfg)

	// 1. Check for exact match on target ID (case-insensitive)
	for _, t := range targets {
		if strings.EqualFold(t.ID, name) {
			return f.engineFromTarget(t), nil
		}
	}

	// 2. Check for match on endpoint ID or target name
	for _, t := range targets {
		if strings.EqualFold(t.EndpointID, name) || strings.EqualFold(t.Name, name) {
			return f.engineFromTarget(t), nil
		}
	}

	// 3. Check for match on provider type
	for _, t := range targets {
		if strings.EqualFold(t.Provider, name) {
			return f.engineFromTarget(t), nil
		}
	}

	// 4. Fallback aliases
	lowerName := strings.ToLower(name)
	switch lowerName {
	case "claude":
		for _, t := range targets {
			if strings.EqualFold(t.Provider, "anthropic") {
				return f.engineFromTarget(t), nil
			}
		}
	case "google":
		for _, t := range targets {
			if strings.EqualFold(t.Provider, "gemini") {
				return f.engineFromTarget(t), nil
			}
		}
	case "llama", "llama.cpp":
		for _, t := range targets {
			if strings.EqualFold(t.Provider, "llamacpp") {
				return f.engineFromTarget(t), nil
			}
		}
	case "local":
		for _, t := range targets {
			if strings.EqualFold(t.Provider, "ollama") || strings.EqualFold(t.Provider, "vllm") || strings.EqualFold(t.Provider, "llamacpp") {
				return f.engineFromTarget(t), nil
			}
		}
	}

	return nil, fmt.Errorf("unsupported AI provider or target '%s'. Supported providers: vllm, llamacpp, ollama, anthropic, gemini, openai", providerOrTargetID)
}

func (f *Factory) engineFromTarget(t config.AITarget) Engine {
	prov := strings.ToLower(t.Provider)

	// Apply defaults from the central table
	baseURL := t.BaseURL
	model := t.Model
	if defaults, ok := providerDefaults[prov]; ok {
		if baseURL == "" {
			baseURL = defaults.BaseURL
		}
		if model == "" {
			model = defaults.Model
		}
	}

	switch prov {
	case "anthropic", "claude":
		maxTokens := t.MaxTokens
		if maxTokens <= 0 {
			maxTokens = 4096
		}
		return NewAnthropicEngine(config.AnthropicConfig{
			BaseURL:     baseURL,
			APIKey:      t.APIKey,
			Model:       model,
			Temperature: t.Temperature,
			MaxTokens:   maxTokens,
		})
	case "gemini", "google":
		return NewGeminiEngine(config.GeminiConfig{
			BaseURL:     baseURL,
			APIKey:      t.APIKey,
			Model:       model,
			Temperature: t.Temperature,
		})
	default:
		// All OpenAI-compatible providers: ollama, vllm, llamacpp, openai, and any unknown
		return NewOpenAICompatibleEngine(t.Provider, config.OpenAICompatibleConfig{
			BaseURL:     baseURL,
			APIKey:      t.APIKey,
			Model:       model,
			Temperature: t.Temperature,
		})
	}
}
