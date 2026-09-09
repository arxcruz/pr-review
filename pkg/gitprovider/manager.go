package gitprovider

import (
	"fmt"
	"strings"

	"github.com/arxcruz/pr-review/pkg/config"
)

type Manager struct {
	cfg      *config.Config
	githubPr *GitHubProvider
	gitlabPr *GitLabProvider
	gerritPr *GerritProvider
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg:      cfg,
		githubPr: NewGitHubProvider(cfg.Git.GitHub),
		gitlabPr: NewGitLabProvider(cfg.Git.GitLab),
		gerritPr: NewGerritProvider(cfg.Git.Gerrit),
	}
}

// GetProvider returns the appropriate provider for a given project configuration
func (m *Manager) GetProvider(project config.ProjectConfig) (Provider, error) {
	providerType := strings.ToLower(strings.TrimSpace(project.Provider))
	switch providerType {
	case "github":
		return m.githubPr, nil
	case "gitlab":
		return m.gitlabPr, nil
	case "gerrit":
		return m.gerritPr, nil
	default:
		return nil, fmt.Errorf("unsupported git provider '%s' for project '%s'", project.Provider, project.ID)
	}
}

// FindProject finds a project configuration by ID or owner/repo match
func (m *Manager) FindProject(identifier string) (*config.ProjectConfig, error) {
	cleanID := strings.TrimSpace(identifier)
	for _, p := range m.cfg.Projects {
		if strings.EqualFold(p.ID, cleanID) {
			return &p, nil
		}
		if p.Provider == "github" && strings.EqualFold(fmt.Sprintf("%s/%s", p.Owner, p.Repo), cleanID) {
			return &p, nil
		}
		if p.Provider == "gitlab" {
			if strings.EqualFold(p.ProjectPath, cleanID) {
				return &p, nil
			}
			if p.Owner != "" && p.Repo != "" && strings.EqualFold(fmt.Sprintf("%s/%s", p.Owner, p.Repo), cleanID) {
				return &p, nil
			}
		}
		if p.Provider == "gerrit" && (strings.EqualFold(p.Repo, cleanID) || strings.EqualFold(p.ProjectPath, cleanID)) {
			return &p, nil
		}
	}

	// Auto-infer if identifier is "owner/repo" or gitlab URL/path
	if strings.Contains(cleanID, "/") {
		parts := strings.Split(cleanID, "/")
		if len(parts) == 2 {
			// Infer default as GitHub
			return &config.ProjectConfig{
				ID:       cleanID,
				Provider: "github",
				Owner:    parts[0],
				Repo:     parts[1],
			}, nil
		}
	}

	return nil, fmt.Errorf("project '%s' not found in config. Please add it to your config file", identifier)
}
