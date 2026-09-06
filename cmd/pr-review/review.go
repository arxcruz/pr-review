package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/arxcruz/pr-review/pkg/review"
	"github.com/spf13/cobra"
)

var (
	postComment bool
	rawOutput   bool
	reviewFile  string
	outputFile  string
	forceAI     bool
)

var reviewCmd = &cobra.Command{
	Use:   "review <project/repo> <pr-number>",
	Short: "Perform an AI code review on a pull request or merge request",
	Long: `Perform an AI code review on a pull request (GitHub) or merge request (GitLab).
The review diff is sent to the selected AI provider (Ollama, Anthropic Claude, Gemini, OpenAI)
and analyzed against review guidelines (loaded from review.md by default).

The generated review is saved automatically to <repository>-<pr-number>.md.
When running with --post, pr-review checks if <repository>-<pr-number>.md exists first
and uses its content, skipping AI generation unless --force is specified.

Examples:
  pr-review review myorg/backend 42
  pr-review review backend 42 --post
  pr-review review backend 42 --post --force
  pr-review review gitlab.com/group/frontend 15 --provider gemini --model gemini-2.5-flash
  pr-review review myorg/backend 42 --provider anthropic --post
  pr-review review myorg/backend 42 --review-file custom-review.md
  pr-review review myorg/backend 42 --provider ollama --model qwen2.5-coder:latest`,
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

		opts := review.ReviewOptions{
			AIProvider:     aiProvider,
			Model:          aiModel,
			PostComment:    postComment,
			GuidelinesFile: reviewFile,
			OutputFile:     outputFile,
			Force:          forceAI,
		}

		targetFile := outputFile
		if targetFile == "" {
			targetFile = reviewEngine.ReviewFilePath(*proj, prNumber)
		}

		if postComment && !forceAI {
			fmt.Printf("🔍 Preparing review for %s #%d (checking %s)...\n", proj.ID, prNumber, targetFile)
		} else {
			fmt.Printf("🔍 Fetching diff and generating review for %s #%d...\n", proj.ID, prNumber)
		}

		outcome, err := reviewEngine.ReviewPR(ctx, *proj, prNumber, opts)
		if err != nil {
			return err
		}

		review.PrintReviewOutcome(outcome, rawOutput)
		return nil
	},
}

func init() {
	reviewCmd.Flags().BoolVarP(&postComment, "post", "P", false, "Post the review as a comment to GitHub/GitLab (uses existing file if present)")
	reviewCmd.Flags().BoolVar(&rawOutput, "raw", false, "Print raw markdown output without terminal formatting")
	reviewCmd.Flags().StringVarP(&reviewFile, "review-file", "r", "", "Path to custom review guidelines markdown file (default: review.md)")
	reviewCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Path to save/load review markdown file (default: <repository>-<pr-number>.md)")
	reviewCmd.Flags().BoolVarP(&forceAI, "force", "f", false, "Force AI review generation even if review markdown file already exists")

	rootCmd.AddCommand(reviewCmd)
}
