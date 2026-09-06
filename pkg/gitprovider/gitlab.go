package gitprovider

import (
	"context"
	"fmt"
	"strings"

	"github.com/arxcruz/pr-review/pkg/config"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type GitLabProvider struct {
	cfg config.GitLabConfig
}

func NewGitLabProvider(cfg config.GitLabConfig) *GitLabProvider {
	return &GitLabProvider{cfg: cfg}
}

func (g *GitLabProvider) Name() string {
	return "gitlab"
}

func (g *GitLabProvider) getClient() (*gitlab.Client, error) {
	opts := []gitlab.ClientOptionFunc{}
	if g.cfg.BaseURL != "" {
		opts = append(opts, gitlab.WithBaseURL(g.cfg.BaseURL))
	}

	client, err := gitlab.NewClient(g.cfg.Token, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitlab client: %w", err)
	}

	return client, nil
}

func (g *GitLabProvider) ListPullRequests(ctx context.Context, project config.ProjectConfig, filter FilterOptions) ([]*PullRequest, error) {
	client, err := g.getClient()
	if err != nil {
		return nil, err
	}

	state := "opened"
	if filter.State != "" {
		if filter.State == "open" {
			state = "opened"
		} else {
			state = filter.State
		}
	}

	opts := &gitlab.ListProjectMergeRequestsOptions{
		State:       gitlab.Ptr(state),
		ListOptions: gitlab.ListOptions{PerPage: 50},
	}

	mrs, _, err := client.MergeRequests.ListProjectMergeRequests(project.ProjectPath, opts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("gitlab API error listing MRs for %s: %w", project.ProjectPath, err)
	}

	var results []*PullRequest
	for _, mr := range mrs {
		author := ""
		if mr.Author != nil {
			author = mr.Author.Username
		}

		labels := mr.Labels

		var createdAt, updatedAt = *mr.CreatedAt, *mr.CreatedAt
		if mr.UpdatedAt != nil {
			updatedAt = *mr.UpdatedAt
		}

		item := &PullRequest{
			ID:           int64(mr.ID),
			Number:       int(mr.IID),
			Title:        mr.Title,
			Description:  mr.Description,
			Author:       author,
			Labels:       labels,
			SourceBranch: mr.SourceBranch,
			TargetBranch: mr.TargetBranch,
			State:        mr.State,
			URL:          mr.WebURL,
			Draft:        mr.Draft,
			CreatedAt:    createdAt,
			UpdatedAt:    updatedAt,
			ProjectID:    project.ID,
			ProviderType: "gitlab",
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

func (g *GitLabProvider) GetPullRequest(ctx context.Context, project config.ProjectConfig, prNumber int) (*PullRequest, error) {
	client, err := g.getClient()
	if err != nil {
		return nil, err
	}

	mr, _, err := client.MergeRequests.GetMergeRequest(project.ProjectPath, int64(prNumber), nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch MR #%d from %s: %w", prNumber, project.ProjectPath, err)
	}

	author := ""
	if mr.Author != nil {
		author = mr.Author.Username
	}

	var createdAt, updatedAt = *mr.CreatedAt, *mr.CreatedAt
	if mr.UpdatedAt != nil {
		updatedAt = *mr.UpdatedAt
	}

	return &PullRequest{
		ID:           int64(mr.ID),
		Number:       int(mr.IID),
		Title:        mr.Title,
		Description:  mr.Description,
		Author:       author,
		Labels:       mr.Labels,
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		State:        mr.State,
		URL:          mr.WebURL,
		Draft:        mr.Draft,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
		ProjectID:    project.ID,
		ProviderType: "gitlab",
	}, nil
}

func (g *GitLabProvider) GetDiff(ctx context.Context, project config.ProjectConfig, prNumber int) (string, error) {
	client, err := g.getClient()
	if err != nil {
		return "", err
	}

	rawDiff, _, err := client.MergeRequests.ShowMergeRequestRawDiffs(project.ProjectPath, int64(prNumber), nil, gitlab.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("failed to get diff for MR #%d in %s: %w", prNumber, project.ProjectPath, err)
	}

	return string(rawDiff), nil
}

func (g *GitLabProvider) PostComment(ctx context.Context, project config.ProjectConfig, prNumber int, comment string) error {
	client, err := g.getClient()
	if err != nil {
		return err
	}

	if strings.TrimSpace(comment) == "" {
		return fmt.Errorf("comment body cannot be empty")
	}

	opts := &gitlab.CreateMergeRequestNoteOptions{
		Body: gitlab.Ptr(comment),
	}

	_, _, err = client.Notes.CreateMergeRequestNote(project.ProjectPath, int64(prNumber), opts, gitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to post note on MR #%d: %w", prNumber, err)
	}

	return nil
}
