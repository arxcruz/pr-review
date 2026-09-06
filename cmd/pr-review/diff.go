package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff <project/repo> <pr-number>",
	Short: "View the diff of a pull request or merge request",
	Long: `Display the raw or syntax-colored git diff of a specific pull request or merge request.

Example:
  pr-review diff myorg/backend 42`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		projectID := args[0]
		prNumber, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid PR number '%s': %w", args[1], err)
		}

		proj, err := reviewEngine.GitManager().FindProject(projectID)
		if err != nil {
			return err
		}

		provider, err := reviewEngine.GitManager().GetProvider(*proj)
		if err != nil {
			return err
		}

		pr, err := provider.GetPullRequest(ctx, *proj, prNumber)
		if err != nil {
			return err
		}

		diff, err := provider.GetDiff(ctx, *proj, prNumber)
		if err != nil {
			return err
		}

		fmt.Printf("Diff for [%s] #%d (%s -> %s):\n", proj.ID, pr.Number, pr.SourceBranch, pr.TargetBranch)
		fmt.Println(diff)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
}
