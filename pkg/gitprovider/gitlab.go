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

func (g *GitLabProvider) getProjectPath(project config.ProjectConfig) (string, error) {
	if strings.TrimSpace(project.ProjectPath) != "" {
		return strings.TrimSpace(project.ProjectPath), nil
	}
	if strings.TrimSpace(project.Owner) != "" && strings.TrimSpace(project.Repo) != "" {
		return fmt.Sprintf("%s/%s", strings.TrimSpace(project.Owner), strings.TrimSpace(project.Repo)), nil
	}
	if strings.TrimSpace(project.Repo) != "" {
		return strings.TrimSpace(project.Repo), nil
	}
	if strings.Contains(project.ID, "/") {
		return strings.TrimSpace(project.ID), nil
	}
	return "", fmt.Errorf("project '%s' is missing project_path (or owner/repo) in configuration", project.ID)
}

func normalizeBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return ""
	}
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}
	return strings.TrimRight(baseURL, "/")
}

func (g *GitLabProvider) getClient() (*gitlab.Client, error) {
	opts := []gitlab.ClientOptionFunc{}
	baseURL := normalizeBaseURL(g.cfg.BaseURL)
	if baseURL != "" {
		opts = append(opts, gitlab.WithBaseURL(baseURL))
	}

	client, err := gitlab.NewClient(g.cfg.Token, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitlab client: %w", err)
	}

	return client, nil
}

func (g *GitLabProvider) ListPullRequests(ctx context.Context, project config.ProjectConfig, filter FilterOptions) ([]*PullRequest, error) {
	projectPath, err := g.getProjectPath(project)
	if err != nil {
		return nil, err
	}

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

	mrs, _, err := client.MergeRequests.ListProjectMergeRequests(projectPath, opts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("gitlab API error listing MRs for %s: %w", projectPath, err)
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
	projectPath, err := g.getProjectPath(project)
	if err != nil {
		return nil, err
	}

	client, err := g.getClient()
	if err != nil {
		return nil, err
	}

	mr, _, err := client.MergeRequests.GetMergeRequest(projectPath, int64(prNumber), nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch MR #%d from %s: %w", prNumber, projectPath, err)
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
	projectPath, err := g.getProjectPath(project)
	if err != nil {
		return "", err
	}

	client, err := g.getClient()
	if err != nil {
		return "", err
	}

	rawDiff, _, err := client.MergeRequests.ShowMergeRequestRawDiffs(projectPath, int64(prNumber), nil, gitlab.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("failed to get diff for MR #%d in %s: %w", prNumber, projectPath, err)
	}

	return string(rawDiff), nil
}

func (g *GitLabProvider) PostComment(ctx context.Context, project config.ProjectConfig, prNumber int, comment string) error {
	projectPath, err := g.getProjectPath(project)
	if err != nil {
		return err
	}

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

	_, _, err = client.Notes.CreateMergeRequestNote(projectPath, int64(prNumber), opts, gitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to post note on MR #%d: %w", prNumber, err)
	}

	return nil
}
