package ai

import (
	"strings"
	"testing"

	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/arxcruz/pr-review/pkg/gitprovider"
)

func TestBuildReviewPrompt(t *testing.T) {
	req := ReviewRequest{
		PR: &gitprovider.PullRequest{
			Title:        "Fix race condition in worker pool",
			Author:       "arxcruz",
			SourceBranch: "fix/worker-race",
			TargetBranch: "main",
			Labels:       []string{"bug", "core"},
			Description:  "This PR fixes synchronization in the worker pool.",
		},
		Diff:               "diff --git a/worker.go b/worker.go\n+ mutex.Lock()",
		Guidelines:         "Be thorough on concurrency.",
		ProjectID:          "backend",
		ProjectDescription: "High throughput distributed worker queue in Go",
	}

	sysPrompt, userPrompt := BuildReviewPrompt(req)

	if !strings.Contains(sysPrompt, "Be thorough on concurrency.") {
		t.Errorf("expected custom guidelines in sysPrompt, got: %s", sysPrompt)
	}
	if !strings.Contains(userPrompt, "## Project Context") {
		t.Errorf("expected project context section in userPrompt, got: %s", userPrompt)
	}
	if !strings.Contains(userPrompt, "- **Project**: backend") {
		t.Errorf("expected project ID in userPrompt, got: %s", userPrompt)
	}
	if !strings.Contains(userPrompt, "High throughput distributed worker queue in Go") {
		t.Errorf("expected project description in userPrompt, got: %s", userPrompt)
	}
	if !strings.Contains(userPrompt, "Fix race condition in worker pool") {
		t.Errorf("expected PR title in userPrompt, got: %s", userPrompt)
	}
	if !strings.Contains(userPrompt, "+ mutex.Lock()") {
		t.Errorf("expected diff in userPrompt, got: %s", userPrompt)
	}
}

func TestBuildReviewPromptWithoutProjectDescription(t *testing.T) {
	req := ReviewRequest{
		PR: &gitprovider.PullRequest{
			Title:  "Simple tweak",
			Author: "dev",
		},
		Diff: "+ fmt.Println(1)",
	}

	_, userPrompt := BuildReviewPrompt(req)
	if strings.Contains(userPrompt, "## Project Context") {
		t.Errorf("did not expect project context section when project description is empty, got: %s", userPrompt)
	}
}

func TestFactoryProviders(t *testing.T) {
	cfg := config.DefaultConfig()
	factory := NewFactory(cfg)

	tests := []struct {
		providerName string
		expectedName string
	}{
		{"local", "ollama"},
		{"ollama", "ollama"},
		{"vllm", "vllm"},
		{"llamacpp", "llamacpp"},
		{"llama.cpp", "llamacpp"},
		{"anthropic", "anthropic"},
		{"claude", "anthropic"},
		{"gemini", "gemini"},
		{"openai", "openai"},
	}

	for _, tt := range tests {
		engine, err := factory.GetEngine(tt.providerName)
		if err != nil {
			t.Fatalf("unexpected error getting engine for %s: %v", tt.providerName, err)
		}
		if engine.Name() != tt.expectedName {
			t.Errorf("expected engine %s, got %s", tt.expectedName, engine.Name())
		}
	}
}

func TestFactoryCustomTargets(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AI.Endpoints = []config.AIEndpointConfig{
		{
			ID:       "remote-vllm",
			Name:     "Remote vLLM",
			Provider: "vllm",
			BaseURL:  "http://192.168.1.100:8000/v1",
			Models:   []string{"deepseek-ai/DeepSeek-Coder-V2", "Qwen/Qwen2.5-Coder-32B"},
		},
	}

	factory := NewFactory(cfg)

	// Test resolution by exact target ID
	engine1, err := factory.GetEngine("remote-vllm:Qwen/Qwen2.5-Coder-32B")
	if err != nil {
		t.Fatalf("failed to resolve target by ID: %v", err)
	}
	if engine1.Name() != "vllm" {
		t.Errorf("expected engine name vllm, got %s", engine1.Name())
	}

	// Test resolution by endpoint ID
	engine2, err := factory.GetEngine("remote-vllm")
	if err != nil {
		t.Fatalf("failed to resolve target by endpoint ID: %v", err)
	}
	if engine2.Name() != "vllm" {
		t.Errorf("expected engine name vllm, got %s", engine2.Name())
	}
}
