package config

import (
	"fmt"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"
)

// JiraConfig defines Jira server connection, origin project, team delivery routing, and doc paths.
type JiraConfig struct {
	URL           string                    `yaml:"url,omitempty"`
	User          string                    `yaml:"user,omitempty"`
	Email         string                    `yaml:"email,omitempty"`
	Token         string                    `yaml:"token,omitempty"`
	PAT           string                    `yaml:"pat,omitempty"`
	OriginProject string                    `yaml:"origin_project,omitempty"`
	Teams         map[string]JiraTeamConfig `yaml:"teams,omitempty"`
	DocPaths      []string                  `yaml:"doc_paths,omitempty"`
}

// JiraTeamConfig defines delivery project routing and issue types for a team.
type JiraTeamConfig struct {
	DeliveryProject string               `yaml:"delivery_project"`
	IssueTypes      JiraIssueTypesConfig `yaml:"issue_types,omitempty"`
	Keywords        []string             `yaml:"keywords,omitempty"`
}

// JiraIssueTypesConfig defines the issue types for Epics, Tasks, and Stories.
type JiraIssueTypesConfig struct {
	Epic  string `yaml:"epic,omitempty"`
	Task  string `yaml:"task,omitempty"`
	Story string `yaml:"story,omitempty"`
}

// DefaultJiraIssueTypes returns standard issue type names.
func DefaultJiraIssueTypes() JiraIssueTypesConfig {
	return JiraIssueTypesConfig{
		Epic:  "Epic",
		Task:  "Task",
		Story: "Story",
	}
}

// GetAuthToken returns configured Token or PAT.
func (j *JiraConfig) GetAuthToken() string {
	if j.Token != "" {
		return j.Token
	}
	return j.PAT
}

// IsConfigured returns true if any Jira setting is defined.
func (j *JiraConfig) IsConfigured() bool {
	return j.URL != "" || j.GetAuthToken() != "" || j.OriginProject != "" || len(j.Teams) > 0 || len(j.DocPaths) > 0
}

// ApplyEnvAndDefaults populates empty fields from environment variables and sets defaults.
func (j *JiraConfig) ApplyEnvAndDefaults() {
	if j.URL == "" {
		j.URL = os.Getenv("JIRA_URL")
	}
	if j.User == "" {
		j.User = os.Getenv("JIRA_USER")
	}
	if j.Email == "" {
		j.Email = os.Getenv("JIRA_EMAIL")
	}
	if j.User == "" && j.Email != "" {
		j.User = j.Email
	}
	if j.Token == "" {
		j.Token = os.Getenv("JIRA_TOKEN")
	}
	if j.PAT == "" {
		j.PAT = os.Getenv("JIRA_PAT")
	}
	if j.OriginProject == "" {
		j.OriginProject = os.Getenv("JIRA_ORIGIN_PROJECT")
	}

	for i, p := range j.DocPaths {
		j.DocPaths[i] = ExpandPath(p)
	}

	for k, team := range j.Teams {
		if team.IssueTypes.Epic == "" {
			team.IssueTypes.Epic = "Epic"
		}
		if team.IssueTypes.Task == "" {
			team.IssueTypes.Task = "Task"
		}
		if team.IssueTypes.Story == "" {
			team.IssueTypes.Story = "Story"
		}
		j.Teams[k] = team
	}
}

// Validate checks that required Jira fields are present and well-formed.
func (j *JiraConfig) Validate() error {
	if j.URL == "" {
		return fmt.Errorf("jira url is required")
	}
	parsedURL, err := url.ParseRequestURI(j.URL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return fmt.Errorf("jira url must be a valid http or https url")
	}
	if j.GetAuthToken() == "" {
		return fmt.Errorf("jira auth credentials (token or pat) are required")
	}
	if j.OriginProject == "" {
		return fmt.Errorf("jira origin project is required")
	}
	if len(j.Teams) == 0 {
		return fmt.Errorf("at least one team delivery project mapping is required")
	}
	for name, team := range j.Teams {
		if team.DeliveryProject == "" {
			return fmt.Errorf("jira team %q: delivery_project is required", name)
		}
	}
	if len(j.DocPaths) == 0 {
		return fmt.Errorf("at least one multi-repo documentation path is required")
	}
	return nil
}

// Custom Unmarshaler for JiraTeamConfig to apply default issue types if omitted
var _ yaml.Unmarshaler = (*JiraTeamConfig)(nil)

func (t *JiraTeamConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawTeam JiraTeamConfig
	var raw rawTeam
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*t = JiraTeamConfig(raw)
	if t.IssueTypes.Epic == "" {
		t.IssueTypes.Epic = "Epic"
	}
	if t.IssueTypes.Task == "" {
		t.IssueTypes.Task = "Task"
	}
	if t.IssueTypes.Story == "" {
		t.IssueTypes.Story = "Story"
	}
	return nil
}
