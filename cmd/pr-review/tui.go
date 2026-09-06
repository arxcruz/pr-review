package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/arxcruz/pr-review/pkg/tui"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive Terminal User Interface (TUI)",
	Long:  `Launch an interactive terminal dashboard to browse PRs, inspect diffs, view cached reviews, and generate AI reviews.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTUI()
	},
}

func runTUI() error {
	if aiProvider != "" {
		appConfig.DefaultAIProvider = aiProvider
	}

	model := tui.NewModel(appConfig, reviewEngine)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
