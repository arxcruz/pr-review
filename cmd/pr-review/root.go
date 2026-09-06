package main

import (
	"fmt"
	"os"

	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/arxcruz/pr-review/pkg/review"
	"github.com/spf13/cobra"
)

var (
	configFile   string
	aiProvider   string
	aiModel      string
	appConfig    *config.Config
	reviewEngine *review.Service
)

var rootCmd = &cobra.Command{
	Use:   "pr-review",
	Short: "AI-powered Code Reviewer for GitHub, GitLab, and Gerrit",
	Long: `pr-review is an extensible CLI and interactive TUI tool to list, inspect, and review
pull requests (GitHub), merge requests (GitLab), and changes (Gerrit) using AI (Local/Ollama, Anthropic Claude, Google Gemini, OpenAI).`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Don't require config load error for config init or wizard
		if cmd.Name() == "init" || cmd.Name() == "wizard" || cmd.Name() == "wizzard" || cmd.Name() == "add-provider" || cmd.Name() == "add-ai" {
			return nil
		}

		cfg, loadedPath, err := config.LoadConfig(configFile)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
		appConfig = cfg
		reviewEngine = review.NewService(appConfig)

		_ = loadedPath
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTUI()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Path to YAML configuration file")
	rootCmd.PersistentFlags().StringVar(&aiProvider, "provider", "", "AI provider to use (ollama/local, vllm, llamacpp, anthropic, gemini, openai)")
	rootCmd.PersistentFlags().StringVar(&aiModel, "model", "", "AI model override (e.g. qwen2.5-coder, claude-3-7-sonnet-20250219, gemini-2.5-flash, gpt-4o)")
}
