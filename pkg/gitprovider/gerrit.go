package gitprovider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arxcruz/pr-review/pkg/config"
)

type GerritProvider struct {
	cfg config.GerritConfig
}

func NewGerritProvider(cfg config.GerritConfig) *GerritProvider {
	return &GerritProvider{cfg: cfg}
}

func (g *GerritProvider) Name() string {
	return "gerrit"
}

func (g *GerritProvider) getBaseURL(project config.ProjectConfig) string {
	if project.GerritURL != "" {
		return strings.TrimRight(project.GerritURL, "/")
	}
	if g.cfg.BaseURL != "" {
		return strings.TrimRight(g.cfg.BaseURL, "/")
	}
	return "https://review.opendev.org"
}

func (g *GerritProvider) getGerritProjectName(project config.ProjectConfig) string {
	if project.Repo != "" {
		return project.Repo
	}
	if project.ProjectPath != "" {
		return project.ProjectPath
	}
	return project.ID
}

func (g *GerritProvider) newRequest(ctx context.Context, method, endpoint string, body io.Reader, authRequired bool) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}

	if g.cfg.Username != "" && g.cfg.Password != "" {
		req.SetBasicAuth(g.cfg.Username, g.cfg.Password)
	} else if authRequired {
		return nil, fmt.Errorf("gerrit username and password/token required for this operation. Configure in config.yaml or GERRIT_USERNAME / GERRIT_PASSWORD env vars")
	}

	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

func stripGerritMagic(data []byte) []byte {
	// Gerrit REST API prefixes all JSON with )]}' to prevent cross-site scripting
	trimmed := bytes.TrimPrefix(data, []byte(")]}'\n"))
	trimmed = bytes.TrimPrefix(trimmed, []byte(")]}'\r\n"))
	trimmed = bytes.TrimPrefix(trimmed, []byte(")]}'"))
	return trimmed
}

type gerritAccount struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type gerritChange struct {
	ID              string                   `json:"id"`
	Project         string                   `json:"project"`
	Branch          string                   `json:"branch"`
	ChangeNumber    int                      `json:"_number"`
	Subject         string                   `json:"subject"`
	Status          string                   `json:"status"` // "NEW", "MERGED", "ABANDONED"
	Owner           gerritAccount            `json:"owner"`
	Created         string                   `json:"created"`
	Updated         string                   `json:"updated"`
	WorkInProgress  bool                     `json:"work_in_progress"`
	CurrentRevision string                   `json:"current_revision"`
	Labels          map[string]gerritLabel   `json:"labels"`
}

type gerritLabel struct {
	Approved *gerritAccount `json:"approved,omitempty"`
	Rejected *gerritAccount `json:"rejected,omitempty"`
	All      []struct {
		Value int `json:"value"`
	} `json:"all,omitempty"`
}

func (g *GerritProvider) ListPullRequests(ctx context.Context, project config.ProjectConfig, filter FilterOptions) ([]*PullRequest, error) {
	baseURL := g.getBaseURL(project)
	projectName := g.getGerritProjectName(project)

	state := "status:open"
	if filter.State != "" {
		switch strings.ToLower(filter.State) {
		case "open", "opened", "new":
			state = "status:open"
		case "closed", "merged":
			state = "status:closed"
		case "all":
			state = ""
		}
	}

	queryParts := []string{fmt.Sprintf("project:%s", projectName)}
	if state != "" {
		queryParts = append(queryParts, state)
	}

	query := strings.Join(queryParts, "+")
	authPrefix := ""
	if g.cfg.Username != "" && g.cfg.Password != "" {
		authPrefix = "/a"
	}

	reqURL := fmt.Sprintf("%s%s/changes/?q=%s&o=CURRENT_REVISION&o=CURRENT_COMMIT&o=LABELS&o=DETAILED_ACCOUNTS",
		baseURL, authPrefix, query)

	req, err := g.newRequest(ctx, http.MethodGet, reqURL, nil, false)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query gerrit changes from %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read gerrit response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gerrit API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	cleanedData := stripGerritMagic(bodyBytes)
	var changes []gerritChange
	if err := json.Unmarshal(cleanedData, &changes); err != nil {
		return nil, fmt.Errorf("failed to parse gerrit JSON: %w (raw response: %s)", err, string(cleanedData))
	}

	var results []*PullRequest
	for _, c := range changes {
		author := c.Owner.Username
		if author == "" {
			author = c.Owner.Name
		}
		if author == "" {
			author = c.Owner.Email
		}

		var labels []string
		for labelName := range c.Labels {
			labels = append(labels, labelName)
		}

		createdTime, _ := time.Parse("2006-01-02 15:04:05.000000000", c.Created)
		updatedTime, _ := time.Parse("2006-01-02 15:04:05.000000000", c.Updated)

		changeURL := fmt.Sprintf("%s/c/%s/+/%d", baseURL, url.PathEscape(c.Project), c.ChangeNumber)

		item := &PullRequest{
			ID:           int64(c.ChangeNumber),
			Number:       c.ChangeNumber,
			Title:        c.Subject,
			Author:       author,
			Labels:       labels,
			SourceBranch: "patchset",
			TargetBranch: c.Branch,
			State:        strings.ToLower(c.Status),
			URL:          changeURL,
			Draft:        c.WorkInProgress,
			CreatedAt:    createdTime,
			UpdatedAt:    updatedTime,
			ProjectID:    project.ID,
			ProviderType: "gerrit",
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

func (g *GerritProvider) GetPullRequest(ctx context.Context, project config.ProjectConfig, prNumber int) (*PullRequest, error) {
	baseURL := g.getBaseURL(project)
	authPrefix := ""
	if g.cfg.Username != "" && g.cfg.Password != "" {
		authPrefix = "/a"
	}

	reqURL := fmt.Sprintf("%s%s/changes/%d/detail?o=CURRENT_REVISION&o=CURRENT_COMMIT", baseURL, authPrefix, prNumber)
	req, err := g.newRequest(ctx, http.MethodGet, reqURL, nil, false)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get gerrit change #%d: %w", prNumber, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read gerrit change details: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gerrit API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	cleanedData := stripGerritMagic(bodyBytes)
	var c gerritChange
	if err := json.Unmarshal(cleanedData, &c); err != nil {
		return nil, fmt.Errorf("failed to parse gerrit change JSON: %w", err)
	}

	author := c.Owner.Username
	if author == "" {
		author = c.Owner.Name
	}
	if author == "" {
		author = c.Owner.Email
	}

	var labels []string
	for labelName := range c.Labels {
		labels = append(labels, labelName)
	}

	createdTime, _ := time.Parse("2006-01-02 15:04:05.000000000", c.Created)
	updatedTime, _ := time.Parse("2006-01-02 15:04:05.000000000", c.Updated)
	changeURL := fmt.Sprintf("%s/c/%s/+/%d", baseURL, url.PathEscape(c.Project), c.ChangeNumber)

	return &PullRequest{
		ID:           int64(c.ChangeNumber),
		Number:       c.ChangeNumber,
		Title:        c.Subject,
		Author:       author,
		Labels:       labels,
		SourceBranch: "patchset",
		TargetBranch: c.Branch,
		State:        strings.ToLower(c.Status),
		URL:          changeURL,
		Draft:        c.WorkInProgress,
		CreatedAt:    createdTime,
		UpdatedAt:    updatedTime,
		ProjectID:    project.ID,
		ProviderType: "gerrit",
	}, nil
}

func (g *GerritProvider) GetDiff(ctx context.Context, project config.ProjectConfig, prNumber int) (string, error) {
	baseURL := g.getBaseURL(project)
	authPrefix := ""
	if g.cfg.Username != "" && g.cfg.Password != "" {
		authPrefix = "/a"
	}

	// Fetch current revision patch from Gerrit (returns base64-encoded git unified patch)
	reqURL := fmt.Sprintf("%s%s/changes/%d/revisions/current/patch", baseURL, authPrefix, prNumber)
	req, err := g.newRequest(ctx, http.MethodGet, reqURL, nil, false)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch patch for change #%d: %w", prNumber, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read patch body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gerrit API returned status %d when fetching patch: %s", resp.StatusCode, string(bodyBytes))
	}

	// Decode base64
	decodedBytes, err := base64.StdEncoding.DecodeString(string(bodyBytes))
	if err != nil {
		// Sometimes raw text is returned if not base64 encoded
		return string(bodyBytes), nil
	}

	return string(decodedBytes), nil
}

func (g *GerritProvider) PostComment(ctx context.Context, project config.ProjectConfig, prNumber int, comment string) error {
	baseURL := g.getBaseURL(project)
	reqURL := fmt.Sprintf("%s/a/changes/%d/revisions/current/review", baseURL, prNumber)

	reviewPayload := map[string]interface{}{
		"message": comment,
	}

	payloadBytes, err := json.Marshal(reviewPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal gerrit review payload: %w", err)
	}

	req, err := g.newRequest(ctx, http.MethodPost, reqURL, bytes.NewReader(payloadBytes), true)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post review to gerrit change #%d: %w", prNumber, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read post review response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("gerrit API error posting review (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
