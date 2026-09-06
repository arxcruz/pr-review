package review

import (
	"fmt"
	"io"
	"strings"

	"github.com/arxcruz/pr-review/pkg/gitprovider"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	successStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#04B575"))
	warnStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFA500"))
	metaStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#5A56E0")).Padding(0, 1)
	cellStyle    = lipgloss.NewStyle().Padding(0, 1)
	numStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00ADD8"))
	authorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#E06C75"))
)

// RenderMarkdown renders markdown content using Glamour with fallback to raw text
func RenderMarkdown(content string) (string, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		return content, err
	}

	out, err := renderer.Render(content)
	if err != nil {
		return content, err
	}
	return out, nil
}

// PrintPullRequestsTable prints a list of pull requests in a prettytable-style box table
func PrintPullRequestsTable(w io.Writer, prs []*gitprovider.PullRequest) {
	if len(prs) == 0 {
		fmt.Fprintln(w, warnStyle.Render("No pull/merge requests found matching criteria."))
		return
	}

	var rows [][]string
	for _, pr := range prs {
		labels := strings.Join(pr.Labels, ", ")
		if labels == "" {
			labels = "-"
		}

		project := pr.ProjectID
		if project == "" {
			project = pr.ProviderType
		}

		rows = append(rows, []string{
			project,
			fmt.Sprintf("#%d", pr.Number),
			truncate(pr.Title, 55),
			"@" + pr.Author,
			truncate(labels, 30),
		})
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))).
		Headers("PROJECT", "PR #", "TITLE", "AUTHOR", "LABELS").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}

			base := cellStyle
			switch col {
			case 1:
				return base.Inherit(numStyle)
			case 3:
				return base.Inherit(authorStyle)
			default:
				return base
			}
		})

	fmt.Fprintln(w, t.Render())
	fmt.Fprintf(w, "%s Total: %d pull/merge request(s)\n", metaStyle.Render("•"), len(prs))
}

// PrintReviewOutcome prints the formatted AI review outcome
func PrintReviewOutcome(outcome *ReviewOutcome, raw bool) {
	header := fmt.Sprintf("\n%s #%d: %s\n%s\n",
		titleStyle.Render(fmt.Sprintf("[%s]", outcome.Project.ID)),
		outcome.PR.Number,
		outcome.PR.Title,
		metaStyle.Render(fmt.Sprintf("Author: @%s | Source: %s -> %s | URL: %s",
			outcome.PR.Author, outcome.PR.SourceBranch, outcome.PR.TargetBranch, outcome.PR.URL)),
	)
	fmt.Print(header)

	var aiBanner string
	if outcome.LoadedFile != "" {
		aiBanner = fmt.Sprintf("%s %s (AI generation skipped)",
			metaStyle.Render("📂 Loaded existing review:"),
			warnStyle.Render(outcome.LoadedFile),
		)
	} else {
		aiBanner = fmt.Sprintf("%s Provider: %s | Model: %s",
			metaStyle.Render("🤖 Review by:"),
			successStyle.Render(outcome.Result.Provider),
			outcome.Result.Model,
		)
		if outcome.Result.TokensUsed > 0 {
			aiBanner += fmt.Sprintf(" | Tokens: %d", outcome.Result.TokensUsed)
		}
	}
	fmt.Println(aiBanner)
	fmt.Println(strings.Repeat("─", 80))

	if raw {
		fmt.Println(outcome.Result.Content)
	} else {
		rendered, err := RenderMarkdown(outcome.Result.Content)
		if err != nil {
			fmt.Println(outcome.Result.Content)
		} else {
			fmt.Print(rendered)
		}
	}

	fmt.Println(strings.Repeat("─", 80))
	if outcome.SavedFile != "" {
		fmt.Println(metaStyle.Render(fmt.Sprintf("💾 Saved review to %s", outcome.SavedFile)))
	}
	if outcome.Commented {
		fmt.Println(successStyle.Render(fmt.Sprintf("✓ Successfully posted review comment to %s PR #%d", outcome.Project.ID, outcome.PR.Number)))
	} else {
		fmt.Println(metaStyle.Render("Tip: Use --post to submit this review as a comment to GitHub / GitLab."))
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
