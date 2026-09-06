package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/arxcruz/pr-review/pkg/tui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	globalConfigFlag bool
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage pr-review configuration",
	Long:  `Inspect, create, and manage configuration files for pr-review.`,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate an example YAML configuration file",
	Long: `Creates a template configuration file in the current directory (or user config dir if --global is specified).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var targetPath string
		if globalConfigFlag {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			targetPath = filepath.Join(home, ".config", "pr-review", "config.yaml")
		} else {
			targetPath = ".pr-review.yaml"
		}

		if _, err := os.Stat(targetPath); err == nil {
			return fmt.Errorf("config file already exists at %s", targetPath)
		}

		starterConfig := config.DefaultConfig()
		starterConfig.Projects = []config.ProjectConfig{
			{
				ID:       "github.com/arxcruz/pr-review",
				Provider: "github",
				Owner:    "arxcruz",
				Repo:     "pr-review",
				DefaultFilters: config.FilterCriteria{
					Labels:  []string{},
					Authors: []string{},
				},
			},
			{
				ID:          "gitlab.com/example-group/sample-repo",
				Provider:    "gitlab",
				ProjectPath: "example-group/sample-repo",
				DefaultFilters: config.FilterCriteria{
					Labels:  []string{},
					Authors: []string{},
				},
			},
		}

		if err := config.SaveConfig(starterConfig, targetPath); err != nil {
			return err
		}

		fmt.Printf("✓ Created template configuration file at %s\n", targetPath)

		reviewPath := filepath.Join(filepath.Dir(targetPath), "review.md")
		if _, err := os.Stat(reviewPath); os.IsNotExist(err) {
			if err := os.WriteFile(reviewPath, []byte(config.DefaultReviewGuidelines()), 0644); err == nil {
				fmt.Printf("✓ Created default review guidelines file at %s\n", reviewPath)
			}
		}

		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show active configuration (with secrets masked)",
	RunE: func(cmd *cobra.Command, args []string) error {
		displayCfg := *appConfig
		if displayCfg.Git.GitHub.Token != "" {
			displayCfg.Git.GitHub.Token = maskString(displayCfg.Git.GitHub.Token)
		}
		if displayCfg.Git.GitLab.Token != "" {
			displayCfg.Git.GitLab.Token = maskString(displayCfg.Git.GitLab.Token)
		}
		if displayCfg.Git.Gerrit.Password != "" {
			displayCfg.Git.Gerrit.Password = maskString(displayCfg.Git.Gerrit.Password)
		}
		if displayCfg.AI.Anthropic.APIKey != "" {
			displayCfg.AI.Anthropic.APIKey = maskString(displayCfg.AI.Anthropic.APIKey)
		}
		if displayCfg.AI.Gemini.APIKey != "" {
			displayCfg.AI.Gemini.APIKey = maskString(displayCfg.AI.Gemini.APIKey)
		}
		if displayCfg.AI.OpenAI.APIKey != "" {
			displayCfg.AI.OpenAI.APIKey = maskString(displayCfg.AI.OpenAI.APIKey)
		}

		data, err := yaml.Marshal(displayCfg)
		if err != nil {
			return err
		}

		fmt.Println(string(data))
		return nil
	},
}

var configWizardCmd = &cobra.Command{
	Use:     "wizard",
	Aliases: []string{"wizzard", "add-provider", "add-ai"},
	Short:   "Interactive TUI wizard to add or configure AI providers, endpoints, and models",
	Long: `Launch an interactive Terminal User Interface (TUI) wizard to easily configure
AI engines (Ollama, vLLM, llama.cpp, Anthropic Claude, Google Gemini, OpenAI,
or custom OpenAI-compatible endpoints) and models directly into your config.yaml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.RunWizard(configFile)
	},
}

func maskString(s string) string {
	if len(s) <= 8 {
		return "********"
	}
	return s[:4] + "..." + s[len(s)-4:]
}

func init() {
	configInitCmd.Flags().BoolVarP(&globalConfigFlag, "global", "g", false, "Write config to ~/.config/pr-review/config.yaml")

	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configWizardCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(configWizardCmd)
}
