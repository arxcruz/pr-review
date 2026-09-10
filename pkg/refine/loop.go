package refine

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/arxcruz/pr-review/pkg/session"
)

// LoopConfig configures the interactive CLI refinement loop.
type LoopConfig struct {
	Engine     *Engine
	Store      session.Store
	Snapshot   *session.Snapshot
	Opts       FrontierOptions
	Router     *Router
	JiraConfig *config.JiraConfig
	PlanFile   string
	In         io.Reader
	Out        io.Writer
}

// SessionLoop manages the interactive refinement REPL.
type SessionLoop struct {
	engine     *Engine
	store      session.Store
	snapshot   *session.Snapshot
	opts       FrontierOptions
	router     *Router
	jiraConfig *config.JiraConfig
	planFile   string
	in         io.Reader
	out        io.Writer
}

// NewSessionLoop creates a new interactive refinement loop instance.
func NewSessionLoop(cfg LoopConfig) *SessionLoop {
	in := cfg.In
	if in == nil {
		in = os.Stdin
	}
	out := cfg.Out
	if out == nil {
		out = os.Stdout
	}
	return &SessionLoop{
		engine:     cfg.Engine,
		store:      cfg.Store,
		snapshot:   cfg.Snapshot,
		opts:       cfg.Opts,
		router:     cfg.Router,
		jiraConfig: cfg.JiraConfig,
		planFile:   cfg.PlanFile,
		in:         in,
		out:        out,
	}
}

// Run executes the interactive refinement loop until the frontier is resolved and finalized.
func (l *SessionLoop) Run(ctx context.Context) error {
	if l.snapshot == nil {
		return fmt.Errorf("session snapshot is required")
	}
	if l.engine == nil {
		return fmt.Errorf("refinement engine is required")
	}

	if l.snapshot.Status == session.StatusFinalized {
		fmt.Fprintf(l.out, "Refinement session for [%s] is already finalized.\n", l.snapshot.Key)
		return l.outputPlan()
	}

	reader := bufio.NewReader(l.in)

	// If resuming or starting fresh without active frontier questions, generate first round
	if len(l.snapshot.CurrentFrontier) == 0 {
		fmt.Fprintf(l.out, "Analyzing Strategic Ticket [%s] and generating frontier questions...\n", l.snapshot.Key)
		if _, err := l.engine.GenerateFrontier(ctx, l.snapshot, l.opts); err != nil {
			return fmt.Errorf("failed to generate initial frontier: %w", err)
		}
		if err := l.saveSnapshot(); err != nil {
			return err
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check if frontier is empty
		if len(l.snapshot.CurrentFrontier) == 0 {
			fmt.Fprintf(l.out, "\nAll frontier questions resolved. The refinement frontier is empty.\n")
			fmt.Fprintf(l.out, "[F]inalize refinement or [a]dd extra requirements? [F/a]: ")

			line, err := reader.ReadString('\n')
			if err != nil && err != io.EOF {
				return fmt.Errorf("failed reading user input: %w", err)
			}
			choice := strings.TrimSpace(line)

			if strings.EqualFold(choice, "a") || strings.EqualFold(choice, "add") {
				fmt.Fprintf(l.out, "Enter additional requirement or architectural guidance: ")
				reqLine, err := reader.ReadString('\n')
				if err != nil && err != io.EOF {
					return fmt.Errorf("failed reading requirement input: %w", err)
				}
				reqText := strings.TrimSpace(reqLine)
				if reqText != "" {
					l.snapshot.AddUserRequirement(reqText)
					if err := l.saveSnapshot(); err != nil {
						return err
					}
					fmt.Fprintf(l.out, "Requirement recorded. Re-evaluating frontier...\n")
					if _, err := l.engine.GenerateFrontier(ctx, l.snapshot, l.opts); err != nil {
						return fmt.Errorf("failed to re-evaluate frontier: %w", err)
					}
					if err := l.saveSnapshot(); err != nil {
						return err
					}
					continue
				}
			}

			// Default: finalize
			if l.snapshot.Tree == nil {
				fmt.Fprintf(l.out, "\nSynthesizing Decomposition Tree from settled refinement rounds...\n")
				tree, err := l.engine.GenerateDecompositionTree(ctx, l.snapshot, DecompositionOptions{
					DocContext:  l.opts.DocContext,
					Guidelines:  l.opts.Guidelines,
					Model:       l.opts.Model,
					Temperature: l.opts.Temperature,
				})
				if err != nil {
					return fmt.Errorf("failed to generate decomposition tree: %w", err)
				}
				fmt.Fprintf(l.out, "Decomposition Tree generated: %d epics created.\n", len(tree.Epics))
			}

			if l.router != nil && l.snapshot.Tree != nil {
				fmt.Fprintf(l.out, "Routing Decomposition Tree to Delivery Projects...\n")
				if err := l.router.Route(ctx, l.snapshot); err != nil {
					return fmt.Errorf("failed to route decomposition tree: %w", err)
				}
			}

			l.snapshot.Status = session.StatusFinalized
			if err := l.saveSnapshot(); err != nil {
				return fmt.Errorf("failed to save finalized snapshot: %w", err)
			}
			fmt.Fprintf(l.out, "Refinement session for [%s] finalized.\n", l.snapshot.Key)

			return l.outputPlan()
		}

		// Prompt user through each question in CurrentFrontier
		roundNum := len(l.snapshot.Rounds) + 1
		totalQ := len(l.snapshot.CurrentFrontier)
		fmt.Fprintf(l.out, "\n=== Refinement Round %d (%d Frontier Questions) ===\n", roundNum, totalQ)

		for i := range l.snapshot.CurrentFrontier {
			q := &l.snapshot.CurrentFrontier[i]

			fmt.Fprintf(l.out, "\n[%s] %s\n", q.ID, q.Title)
			if q.Explanation != "" {
				fmt.Fprintf(l.out, "  Why: %s\n", q.Explanation)
			}

			if len(q.Options) > 0 {
				fmt.Fprintf(l.out, "  Options:\n")
				for idx, opt := range q.Options {
					fmt.Fprintf(l.out, "    [%d] %s\n", idx+1, opt)
				}
			}

			if q.Recommendation != "" {
				fmt.Fprintf(l.out, "  Recommendation: %s\n", q.Recommendation)
			}

			promptMsg := "  Answer"
			if q.Recommendation != "" {
				if len(q.Options) > 0 {
					promptMsg += fmt.Sprintf(" [Enter to accept recommendation: %q, or 1-%d, or custom answer]", q.Recommendation, len(q.Options))
				} else {
					promptMsg += fmt.Sprintf(" [Enter to accept recommendation: %q, or custom answer]", q.Recommendation)
				}
			} else if len(q.Options) > 0 {
				promptMsg += fmt.Sprintf(" [1-%d, or custom answer]", len(q.Options))
			}
			promptMsg += ": "

			var chosenAnswer string
			for {
				fmt.Fprint(l.out, promptMsg)

				answerLine, err := reader.ReadString('\n')
				if err != nil && err != io.EOF {
					return fmt.Errorf("failed reading user answer: %w", err)
				}
				if err == io.EOF && len(strings.TrimSpace(answerLine)) == 0 && q.Recommendation == "" {
					return io.EOF
				}

				rawAnswer := strings.TrimSpace(answerLine)
				if rawAnswer == "" {
					if q.Recommendation != "" {
						chosenAnswer = q.Recommendation
						break
					}
					if len(q.Options) > 0 {
						fmt.Fprintf(l.out, "  (Please select an option 1-%d or enter a custom answer)\n", len(q.Options))
						continue
					}
					chosenAnswer = ""
					break
				}

				// Check if answer matches option index
				if optIdx, parseErr := strconv.Atoi(rawAnswer); parseErr == nil && optIdx >= 1 && optIdx <= len(q.Options) {
					chosenAnswer = q.Options[optIdx-1]
				} else {
					chosenAnswer = rawAnswer
				}
				break
			}

			q.Answer = chosenAnswer
			fmt.Fprintf(l.out, "  -> Answer recorded: %s\n", q.Answer)
		}

		// Advance round and persist snapshot
		l.snapshot.AdvanceRound()
		if err := l.saveSnapshot(); err != nil {
			return fmt.Errorf("failed to save snapshot after round: %w", err)
		}

		fmt.Fprintf(l.out, "\nRound %d answers saved. Computing next frontier round...\n", roundNum)
		if _, err := l.engine.GenerateFrontier(ctx, l.snapshot, l.opts); err != nil {
			return fmt.Errorf("failed to generate next frontier round: %w", err)
		}
		if err := l.saveSnapshot(); err != nil {
			return fmt.Errorf("failed to save snapshot with new frontier: %w", err)
		}
	}
}

func (l *SessionLoop) saveSnapshot() error {
	if l.store == nil || l.snapshot == nil {
		return nil
	}
	return l.store.Save(l.snapshot)
}

func (l *SessionLoop) outputPlan() error {
	if l.snapshot == nil || l.snapshot.Tree == nil {
		return nil
	}
	planMd := FormatPlanMarkdown(l.snapshot, l.jiraConfig)
	fmt.Fprintf(l.out, "\n%s\n", planMd)
	if l.planFile != "" {
		if err := WritePlanFile(l.planFile, planMd); err != nil {
			return fmt.Errorf("failed to write plan file: %w", err)
		}
		fmt.Fprintf(l.out, "Plan successfully written to %s\n", l.planFile)
	}
	return nil
}
