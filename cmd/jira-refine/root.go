package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/arxcruz/pr-review/pkg/jira"
	"github.com/spf13/cobra"
)

type rootOptions struct {
	configFile string
	dump       bool
}

func newRootCmd() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:   "jira-refine [KEY]",
		Short: "Interactive refinement of Jira Strategic Tickets into delivery items",
		Long: `jira-refine is a tool to interactively refine Jira Strategic Tickets
into structured, actionable delivery items across team projects.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hasArg := len(args) > 0 && strings.TrimSpace(args[0]) != ""
			if opts.dump {
				if !hasArg {
					return fmt.Errorf("ticket key is required when using --dump")
				}
				key := strings.TrimSpace(args[0])

				_, client, err := initJiraClient(opts.configFile)
				if err != nil {
					return err
				}

				ticket, err := client.GetTicket(cmd.Context(), key)
				if err != nil {
					return err
				}

				summary := jira.FormatTicketSummary(ticket)
				fmt.Fprint(cmd.OutOrStdout(), summary)
				return nil
			}

			if !hasArg {
				cfg, client, err := initJiraClient(opts.configFile)
				if err != nil {
					return err
				}

				tickets, err := client.GetRecentTickets(cmd.Context(), cfg.Jira.OriginProject)
				if err != nil {
					return err
				}

				table := jira.FormatRecentTicketsTable(tickets)
				fmt.Fprint(cmd.OutOrStdout(), table)
				return nil
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.configFile, "config", "c", "", "Path to YAML configuration file")
	cmd.Flags().BoolVar(&opts.dump, "dump", false, "Dump parsed ticket summary to terminal")

	return cmd
}

func initJiraClient(configFile string) (*config.Config, jira.Client, error) {
	cfg, _, err := config.LoadConfig(configFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	if err := cfg.Jira.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid jira configuration: %w", err)
	}

	client, err := jira.NewClient(cfg.Jira)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize jira client: %w", err)
	}

	return cfg, client, nil
}

func Execute() {
	cmd := newRootCmd()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
