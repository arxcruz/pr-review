package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/arxcruz/pr-review/pkg/ai"
	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/arxcruz/pr-review/pkg/docscan"
	"github.com/arxcruz/pr-review/pkg/jira"
	"github.com/arxcruz/pr-review/pkg/refine"
	"github.com/arxcruz/pr-review/pkg/session"
	"github.com/spf13/cobra"
)

type rootOptions struct {
	configFile string
	dump       bool
	plan       bool
	planFile   string
	sync       bool
	yes        bool
	provider   string
	model      string
	sessionDir string
}

func newRootCmd() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:   "jira-refine [KEY]",
		Short: "Interactive refinement of Jira Strategic Tickets into delivery items",
		Long: `jira-refine is a tool to interactively refine Jira Strategic Tickets
into structured, actionable delivery items across team projects.`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
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

			if opts.plan {
				if !hasArg {
					return fmt.Errorf("ticket key is required when using --plan")
				}
				key := strings.TrimSpace(args[0])
				cfg, _, err := initJiraClient(opts.configFile)
				if err != nil {
					return err
				}

				store := session.NewFileStore(opts.sessionDir)
				snap, err := store.Load(key)
				if err != nil {
					return fmt.Errorf("failed to load session snapshot: %w", err)
				}
				if snap.Tree == nil || len(snap.Tree.Epics) == 0 {
					return fmt.Errorf("no decomposition tree found in session snapshot for %s; refine ticket first", key)
				}

				router := refine.NewRouter(&cfg.Jira)
				if err := router.Route(cmd.Context(), snap); err != nil {
					return fmt.Errorf("failed to route decomposition tree: %w", err)
				}
				if err := store.Save(snap); err != nil {
					return fmt.Errorf("failed to save routed snapshot: %w", err)
				}

				planMd := refine.FormatPlanMarkdown(snap, &cfg.Jira)
				fmt.Fprint(cmd.OutOrStdout(), planMd)

				if opts.planFile != "" {
					if err := refine.WritePlanFile(opts.planFile, planMd); err != nil {
						return fmt.Errorf("failed to write plan file: %w", err)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "\nPlan successfully written to %s\n", opts.planFile)
				}
				return nil
			}

			if opts.sync {
				if !hasArg {
					return fmt.Errorf("ticket key is required when using --sync")
				}
				key := strings.TrimSpace(args[0])
				cfg, client, err := initJiraClient(opts.configFile)
				if err != nil {
					return err
				}

				store := session.NewFileStore(opts.sessionDir)
				snap, err := store.Load(key)
				if err != nil {
					return fmt.Errorf("failed to load session snapshot: %w", err)
				}

				syncer := refine.NewSyncer(client, store, &cfg.Jira)
				result, err := syncer.Sync(cmd.Context(), snap, refine.SyncOptions{
					In:          cmd.InOrStdin(),
					Out:         cmd.OutOrStdout(),
					AutoConfirm: opts.yes,
				})
				if err != nil {
					return fmt.Errorf("failed to synchronize with jira: %w", err)
				}

				if result != nil && !result.Aborted {
					summaryTable := refine.FormatSyncSummaryTable(result)
					fmt.Fprint(cmd.OutOrStdout(), summaryTable)
				}
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

			key := strings.TrimSpace(args[0])
			cfg, client, err := initJiraClient(opts.configFile)
			if err != nil {
				return err
			}

			store := session.NewFileStore(opts.sessionDir)

			var snap *session.Snapshot
			existing, err := store.Load(key)
			if err == nil {
				snap = existing
			} else if errors.Is(err, session.ErrNotFound) {
				ticket, err := client.GetTicket(cmd.Context(), key)
				if err != nil {
					return fmt.Errorf("failed to fetch ticket %s: %w", key, err)
				}
				snap = &session.Snapshot{
					Key:    key,
					Status: session.StatusNew,
					Ticket: *ticket,
				}
				if err := store.Save(snap); err != nil {
					return fmt.Errorf("failed to initialize session snapshot: %w", err)
				}
			} else {
				return fmt.Errorf("failed to load session snapshot: %w", err)
			}

			aiFactory := ai.NewFactory(cfg)
			aiEngine, err := aiFactory.GetEngine(opts.provider)
			if err != nil {
				return fmt.Errorf("failed to initialize AI engine: %w", err)
			}
			refineEngine := refine.NewEngine(aiEngine)

			var docContext string
			if len(cfg.Jira.DocPaths) > 0 {
				scanner := docscan.NewScanner(docscan.DefaultConfig())
				scanRes, scanErr := scanner.Scan(cfg.Jira.DocPaths)
				if scanErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed scanning doc paths: %v\n", scanErr)
				} else if len(scanRes.Documents) > 0 {
					docContext = scanner.FormatPromptContext(scanRes.Documents)
				}
			}

			router := refine.NewRouter(&cfg.Jira,
				refine.WithAIEngine(aiEngine),
				refine.WithModel(opts.model),
				refine.WithInteractive(cmd.InOrStdin(), cmd.OutOrStdout()),
			)

			loop := refine.NewSessionLoop(refine.LoopConfig{
				Engine:     refineEngine,
				Store:      store,
				Snapshot:   snap,
				Router:     router,
				JiraConfig: &cfg.Jira,
				PlanFile:   opts.planFile,
				Opts: refine.FrontierOptions{
					DocContext: docContext,
					Model:      opts.model,
				},
				In:  cmd.InOrStdin(),
				Out: cmd.OutOrStdout(),
			})

			return loop.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&opts.configFile, "config", "c", "", "Path to YAML configuration file")
	cmd.Flags().BoolVar(&opts.dump, "dump", false, "Dump parsed ticket summary to terminal")
	cmd.Flags().BoolVar(&opts.plan, "plan", false, "Output formatted markdown plan summary for ticket session")
	cmd.Flags().StringVar(&opts.planFile, "plan-file", "", "Path to write formatted plan markdown file")
	cmd.Flags().BoolVar(&opts.sync, "sync", false, "Synchronize decomposition tree with remote Jira instance")
	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Skip confirmation prompt when executing Jira synchronization")
	cmd.Flags().StringVar(&opts.provider, "provider", "", "AI provider to use (ollama, openai, anthropic, gemini, etc.)")
	cmd.Flags().StringVar(&opts.model, "model", "", "AI model override")
	cmd.Flags().StringVar(&opts.sessionDir, "session-dir", "", "Path to directory for persisting session snapshots")

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
