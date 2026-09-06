package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/arxcruz/pr-review/pkg/gitprovider"
	"github.com/arxcruz/pr-review/pkg/review"
	"github.com/spf13/cobra"
)

var (
	filterAuthors []string
	filterLabels  []string
	filterProject string
	filterState   string
	filterLimit   int
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List pull requests and merge requests across configured projects",
	Long: `List pull requests (GitHub) and merge requests (GitLab) across configured projects.
Supports filtering by author, labels, and project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var targetProjects []config.ProjectConfig
		if filterProject != "" {
			proj, err := reviewEngine.GitManager().FindProject(filterProject)
			if err != nil {
				return err
			}
			targetProjects = append(targetProjects, *proj)
		} else if len(appConfig.Projects) > 0 {
			targetProjects = appConfig.Projects
		} else {
			return fmt.Errorf("no projects configured in config file. Use --project <owner/repo> or configure projects in your config file")
		}

		// Flatten comma-separated values in flags
		authors := parseCommaSeparated(filterAuthors)
		labels := parseCommaSeparated(filterLabels)

		var allPRs []*gitprovider.PullRequest

		for _, proj := range targetProjects {
			provider, err := reviewEngine.GitManager().GetProvider(proj)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
				continue
			}

			// Merge CLI filters with default filters if CLI flags were not specified
			projAuthors := authors
			if len(projAuthors) == 0 && len(proj.DefaultFilters.Authors) > 0 {
				projAuthors = proj.DefaultFilters.Authors
			}

			projLabels := labels
			if len(projLabels) == 0 && len(proj.DefaultFilters.Labels) > 0 {
				projLabels = proj.DefaultFilters.Labels
			}

			filter := gitprovider.FilterOptions{
				Authors: projAuthors,
				Labels:  projLabels,
				State:   filterState,
				Limit:   filterLimit,
			}

			prs, err := provider.ListPullRequests(ctx, proj, filter)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error querying %s: %v\n", proj.ID, err)
				continue
			}
			allPRs = append(allPRs, prs...)
		}

		review.PrintPullRequestsTable(os.Stdout, allPRs)
		return nil
	},
}

func parseCommaSeparated(inputs []string) []string {
	var results []string
	for _, item := range inputs {
		for _, part := range strings.Split(item, ",") {
			cleaned := strings.TrimSpace(part)
			if cleaned != "" {
				results = append(results, cleaned)
			}
		}
	}
	return results
}

func init() {
	listCmd.Flags().StringSliceVarP(&filterAuthors, "author", "a", nil, "Filter by author (comma-separated or multiple flags)")
	listCmd.Flags().StringSliceVarP(&filterLabels, "label", "l", nil, "Filter by label (comma-separated or multiple flags)")
	listCmd.Flags().StringVarP(&filterProject, "project", "p", "", "Filter to a specific project ID or owner/repo")
	listCmd.Flags().StringVarP(&filterProject, "repo", "r", "", "Alias for --project")
	listCmd.Flags().StringVarP(&filterState, "state", "s", "open", "Filter by PR state (open, closed, all)")
	listCmd.Flags().IntVar(&filterLimit, "limit", 50, "Limit number of PRs returned per project")

	rootCmd.AddCommand(listCmd)
}
