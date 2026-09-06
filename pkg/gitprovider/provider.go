package gitprovider

import (
	"context"
	"strings"
	"time"

	"github.com/arxcruz/pr-review/pkg/config"
)

// PullRequest represents a unified pull request (GitHub) or merge request (GitLab)
type PullRequest struct {
	ID           int64     `json:"id"`
	Number       int       `json:"number"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Author       string    `json:"author"`
	Labels       []string  `json:"labels"`
	SourceBranch string    `json:"source_branch"`
	TargetBranch string    `json:"target_branch"`
	State        string    `json:"state"`
	URL          string    `json:"url"`
	Draft        bool      `json:"draft"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	ProjectID    string    `json:"project_id"`
	ProviderType string    `json:"provider_type"` // "github" or "gitlab"
}

// FilterOptions defines filters when querying PRs/MRs
type FilterOptions struct {
	Labels  []string
	Authors []string
	State   string // "open", "closed", "all"
	Limit   int
}

// Matches returns true if the PR satisfies the filter criteria
func (pr *PullRequest) Matches(opts FilterOptions) bool {
	// Check Author filter (case-insensitive)
	if len(opts.Authors) > 0 {
		matchedAuthor := false
		for _, a := range opts.Authors {
			if strings.EqualFold(pr.Author, strings.TrimSpace(a)) {
				matchedAuthor = true
				break
			}
		}
		if !matchedAuthor {
			return false
		}
	}

	// Check Label filter (case-insensitive - must contain each requested label or at least one depending on use)
	if len(opts.Labels) > 0 {
		for _, requiredLabel := range opts.Labels {
			req := strings.TrimSpace(requiredLabel)
			matchedLabel := false
			for _, prLabel := range pr.Labels {
				if strings.EqualFold(prLabel, req) {
					matchedLabel = true
					break
				}
			}
			if !matchedLabel {
				return false
			}
		}
	}

	return true
}

// Provider is the interface implemented by GitHub and GitLab clients
type Provider interface {
	Name() string
	ListPullRequests(ctx context.Context, project config.ProjectConfig, filter FilterOptions) ([]*PullRequest, error)
	GetPullRequest(ctx context.Context, project config.ProjectConfig, prNumber int) (*PullRequest, error)
	GetDiff(ctx context.Context, project config.ProjectConfig, prNumber int) (string, error)
	PostComment(ctx context.Context, project config.ProjectConfig, prNumber int, comment string) error
}
