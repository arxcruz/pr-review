package main

import (
	"os"

	"github.com/arxcruz/pr-review/pkg/review"
	"github.com/spf13/cobra"
)

var projectsVerbose bool

var projectsCmd = &cobra.Command{
	Use:     "projects",
	Aliases: []string{"proj"},
	Short:   "List projects configured in the config file",
	Long:    `List the projects defined under 'projects:' in your config file (ID, provider, and repo/project path).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		review.PrintProjectsTable(os.Stdout, appConfig.Projects, projectsVerbose)
		return nil
	},
}

func init() {
	projectsCmd.Flags().BoolVarP(&projectsVerbose, "verbose", "v", false, "Also show description and default filters for each project")

	rootCmd.AddCommand(projectsCmd)
}
