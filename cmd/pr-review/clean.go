package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/spf13/cobra"
)

var (
	cleanAll     bool
	cleanForce   bool
	cleanPR      int
	cleanProject string
	cleanFile    string
)

var cleanCmd = &cobra.Command{
	Use:     "clean [project/repo] [pr-number]",
	Aliases: []string{"delete", "rm"},
	Short:   "Delete cached AI review markdown files",
	Long: `Delete cached AI review markdown files from disk.

You can delete a specific review file, all reviews for a PR, all reviews for a project,
or all cached reviews entirely.

Examples:
  pr-review clean myorg/backend 42         # Delete all reviews for PR #42 in myorg/backend
  pr-review clean backend 42 --force       # Delete without confirmation prompt
  pr-review clean backend --all            # Delete all reviews for project backend
  pr-review clean --all                    # Delete all reviews across all projects
  pr-review clean --file reviews/repo-42.md # Delete specific review file`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Specific file deletion
		if cleanFile != "" {
			if !cleanForce && !confirmPrompt(fmt.Sprintf("Delete review file %s?", cleanFile)) {
				fmt.Println("Aborted.")
				return nil
			}
			if err := reviewEngine.DeleteReviewFile(cleanFile); err != nil {
				return err
			}
			fmt.Printf("✓ Successfully deleted review file: %s\n", cleanFile)
			return nil
		}

		var targetProject *config.ProjectConfig
		prNumber := cleanPR

		if len(args) > 0 {
			// First arg could be project ID or repo
			proj, err := reviewEngine.GitManager().FindProject(args[0])
			if err == nil {
				targetProject = proj
			} else if cleanProject == "" {
				cleanProject = args[0]
			}
		}

		if len(args) > 1 {
			num, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid PR number '%s': %w", args[1], err)
			}
			prNumber = num
		}

		if targetProject == nil && cleanProject != "" {
			proj, err := reviewEngine.GitManager().FindProject(cleanProject)
			if err != nil {
				return err
			}
			targetProject = proj
		}

		// Delete all reviews across all projects or specific project
		if cleanAll {
			var promptMsg string
			if targetProject != nil {
				promptMsg = fmt.Sprintf("Delete all cached reviews for project '%s'?", targetProject.ID)
			} else {
				promptMsg = "Delete all cached reviews across all projects?"
			}

			if !cleanForce && !confirmPrompt(promptMsg) {
				fmt.Println("Aborted.")
				return nil
			}

			count, err := reviewEngine.DeleteAllReviews(targetProject)
			if err != nil {
				return fmt.Errorf("failed to delete some reviews: %w", err)
			}
			if count == 0 {
				fmt.Println("No cached review files found to delete.")
			} else {
				fmt.Printf("✓ Successfully deleted %d cached review file(s).\n", count)
			}
			return nil
		}

		// Delete reviews for a specific PR
		if prNumber > 0 {
			if targetProject == nil {
				if len(appConfig.Projects) > 0 {
					targetProject = &appConfig.Projects[0]
				} else {
					targetProject = &config.ProjectConfig{ID: "default", Provider: "github"}
				}
			}

			reviews := reviewEngine.ListReviewsForPR(*targetProject, prNumber)
			if len(reviews) == 0 {
				fmt.Printf("No cached reviews found for %s PR #%d.\n", targetProject.ID, prNumber)
				return nil
			}

			if !cleanForce {
				promptMsg := fmt.Sprintf("Delete %d review file(s) for %s PR #%d?", len(reviews), targetProject.ID, prNumber)
				if !confirmPrompt(promptMsg) {
					fmt.Println("Aborted.")
					return nil
				}
			}

			count, err := reviewEngine.DeleteReviewsForPR(*targetProject, prNumber)
			if err != nil {
				return fmt.Errorf("failed to delete some reviews for PR #%d: %w", prNumber, err)
			}
			fmt.Printf("✓ Successfully deleted %d review file(s) for %s PR #%d.\n", count, targetProject.ID, prNumber)
			return nil
		}

		return cmd.Help()
	},
}

func confirmPrompt(prompt string) bool {
	fmt.Printf("%s [y/N]: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

func init() {
	cleanCmd.Flags().BoolVarP(&cleanAll, "all", "a", false, "Delete all cached reviews (or all for the specified project)")
	cleanCmd.Flags().BoolVarP(&cleanForce, "force", "f", false, "Do not prompt for confirmation")
	cleanCmd.Flags().BoolVarP(&cleanForce, "yes", "y", false, "Alias for --force")
	cleanCmd.Flags().IntVar(&cleanPR, "pr", 0, "Pull request number to delete reviews for")
	cleanCmd.Flags().StringVarP(&cleanProject, "project", "p", "", "Project ID or repo name")
	cleanCmd.Flags().StringVar(&cleanFile, "file", "", "Specific review file path to delete")

	rootCmd.AddCommand(cleanCmd)
}
