package gitprovider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/google/go-github/v69/github"
	"golang.org/x/oauth2"
)

type GitHubProvider struct {
	cfg config.GitHubConfig
}

func NewGitHubProvider(cfg config.GitHubConfig) *GitHubProvider {
	return &GitHubProvider{cfg: cfg}
}

func (g *GitHubProvider) Name() string {
	return "github"
}

func (g *GitHubProvider) getClient(ctx context.Context) (*github.Client, error) {
	var httpClient *http.Client
	if g.cfg.Token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: g.cfg.Token},
		)
		httpClient = oauth2.NewClient(ctx, ts)
	}

	var client *github.Client
	if g.cfg.BaseURL != "" {
		var err error
		client, err = github.NewClient(httpClient).WithEnterpriseURLs(g.cfg.BaseURL, g.cfg.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create enterprise github client: %w", err)
		}
	} else {
		client = github.NewClient(httpClient)
	}

	return client, nil
}

func (g *GitHubProvider) ListPullRequests(ctx context.Context, project config.ProjectConfig, filter FilterOptions) ([]*PullRequest, error) {
	client, err := g.getClient(ctx)
	if err != nil {
		return nil, err
	}

	state := "open"
	if filter.State != "" {
		state = filter.State
	}

	opts := &github.PullRequestListOptions{
		State:       state,
		ListOptions: github.ListOptions{PerPage: 50},
	}

	ghPRs, _, err := client.PullRequests.List(ctx, project.Owner, project.Repo, opts)
	if err != nil {
		return nil, fmt.Errorf("github API error listing PRs for %s/%s: %w", project.Owner, project.Repo, err)
	}

	var results []*PullRequest
	for _, pr := range ghPRs {
		labels := make([]string, 0, len(pr.Labels))
		for _, l := range pr.Labels {
			if l.Name != nil {
				labels = append(labels, *l.Name)
			}
		}

		author := ""
		if pr.User != nil && pr.User.Login != nil {
			author = *pr.User.Login
		}

		sourceBranch := ""
		if pr.Head != nil && pr.Head.Ref != nil {
			sourceBranch = *pr.Head.Ref
		}

		targetBranch := ""
		if pr.Base != nil && pr.Base.Ref != nil {
			targetBranch = *pr.Base.Ref
		}

		title := ""
		if pr.Title != nil {
			title = *pr.Title
		}

		description := ""
		if pr.Body != nil {
			description = *pr.Body
		}

		htmlURL := ""
		if pr.HTMLURL != nil {
			htmlURL = *pr.HTMLURL
		}

		prState := ""
		if pr.State != nil {
			prState = *pr.State
		}

		isDraft := false
		if pr.Draft != nil {
			isDraft = *pr.Draft
		}

		item := &PullRequest{
			ID:           pr.GetID(),
			Number:       pr.GetNumber(),
			Title:        title,
			Description:  description,
			Author:       author,
			Labels:       labels,
			SourceBranch: sourceBranch,
			TargetBranch: targetBranch,
			State:        prState,
			URL:          htmlURL,
			Draft:        isDraft,
			CreatedAt:    pr.GetCreatedAt().Time,
			UpdatedAt:    pr.GetUpdatedAt().Time,
			ProjectID:    project.ID,
			ProviderType: "github",
		}

		if item.Matches(filter) {
			results = append(results, item)
			if filter.Limit > 0 && len(results) >= filter.Limit {
				break
			}
		}
	}

	return results, nil
}

func (g *GitHubProvider) GetPullRequest(ctx context.Context, project config.ProjectConfig, prNumber int) (*PullRequest, error) {
	client, err := g.getClient(ctx)
	if err != nil {
		return nil, err
	}

	pr, _, err := client.PullRequests.Get(ctx, project.Owner, project.Repo, prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PR #%d from %s/%s: %w", prNumber, project.Owner, project.Repo, err)
	}

	labels := make([]string, 0, len(pr.Labels))
	for _, l := range pr.Labels {
		if l.Name != nil {
			labels = append(labels, *l.Name)
		}
	}

	author := ""
	if pr.User != nil && pr.User.Login != nil {
		author = *pr.User.Login
	}

	sourceBranch := ""
	if pr.Head != nil && pr.Head.Ref != nil {
		sourceBranch = *pr.Head.Ref
	}

	targetBranch := ""
	if pr.Base != nil && pr.Base.Ref != nil {
		targetBranch = *pr.Base.Ref
	}

	return &PullRequest{
		ID:           pr.GetID(),
		Number:       pr.GetNumber(),
		Title:        pr.GetTitle(),
		Description:  pr.GetBody(),
		Author:       author,
		Labels:       labels,
		SourceBranch: sourceBranch,
		TargetBranch: targetBranch,
		State:        pr.GetState(),
		URL:          pr.GetHTMLURL(),
		Draft:        pr.GetDraft(),
		CreatedAt:    pr.GetCreatedAt().Time,
		UpdatedAt:    pr.GetUpdatedAt().Time,
		ProjectID:    project.ID,
		ProviderType: "github",
	}, nil
}

func (g *GitHubProvider) GetDiff(ctx context.Context, project config.ProjectConfig, prNumber int) (string, error) {
	client, err := g.getClient(ctx)
	if err != nil {
		return "", err
	}

	rawDiff, _, err := client.PullRequests.GetRaw(ctx, project.Owner, project.Repo, prNumber, github.RawOptions{
		Type: github.Diff,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get diff for PR #%d in %s/%s: %w", prNumber, project.Owner, project.Repo, err)
	}

	return rawDiff, nil
}

func (g *GitHubProvider) PostComment(ctx context.Context, project config.ProjectConfig, prNumber int, comment string) error {
	client, err := g.getClient(ctx)
	if err != nil {
		return err
	}

	if strings.TrimSpace(comment) == "" {
		return fmt.Errorf("comment body cannot be empty")
	}

	issueComment := &github.IssueComment{
		Body: &comment,
	}

	_, _, err = client.Issues.CreateComment(ctx, project.Owner, project.Repo, prNumber, issueComment)
	if err != nil {
		return fmt.Errorf("failed to post comment on PR #%d: %w", prNumber, err)
	}

	return nil
}
