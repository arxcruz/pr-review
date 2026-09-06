package ai

import (
	"fmt"
	"strings"

	"github.com/arxcruz/pr-review/pkg/config"
)

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
	switch strings.ToLower(t.Provider) {
	case "ollama":
		return NewOllamaEngine(config.OllamaConfig{
			BaseURL:     t.BaseURL,
			Model:       t.Model,
			Temperature: t.Temperature,
		})
	case "anthropic", "claude":
		return NewAnthropicEngine(config.AnthropicConfig{
			BaseURL:     t.BaseURL,
			APIKey:      t.APIKey,
			Model:       t.Model,
			Temperature: t.Temperature,
			MaxTokens:   t.MaxTokens,
		})
	case "gemini", "google":
		return NewGeminiEngine(config.GeminiConfig{
			BaseURL:     t.BaseURL,
			APIKey:      t.APIKey,
			Model:       t.Model,
			Temperature: t.Temperature,
		})
	case "vllm":
		return NewOpenAICompatibleEngine("vllm", config.OpenAIConfig{
			BaseURL:     t.BaseURL,
			APIKey:      t.APIKey,
			Model:       t.Model,
			Temperature: t.Temperature,
		})
	case "llamacpp", "llama":
		return NewOpenAICompatibleEngine("llamacpp", config.OpenAIConfig{
			BaseURL:     t.BaseURL,
			Model:       t.Model,
			Temperature: t.Temperature,
		})
	default:
		return NewOpenAICompatibleEngine(t.Provider, config.OpenAIConfig{
			BaseURL:     t.BaseURL,
			APIKey:      t.APIKey,
			Model:       t.Model,
			Temperature: t.Temperature,
		})
	}
}
