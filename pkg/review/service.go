package review

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/arxcruz/pr-review/pkg/ai"
	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/arxcruz/pr-review/pkg/gitprovider"
	"gopkg.in/yaml.v3"
)

type Service struct {
	cfg        *config.Config
	gitManager *gitprovider.Manager
	aiFactory  *ai.Factory
}

func NewService(cfg *config.Config) *Service {
	return &Service{
		cfg:        cfg,
		gitManager: gitprovider.NewManager(cfg),
		aiFactory:  ai.NewFactory(cfg),
	}
}

func (s *Service) GitManager() *gitprovider.Manager {
	return s.gitManager
}

func (s *Service) AIFactory() *ai.Factory {
	return s.aiFactory
}

type ReviewOptions struct {
	AIProvider     string
	Model          string
	PostComment    bool
	Guidelines     string
	GuidelinesFile string
	OutputFile     string
	Force          bool
}

type ReviewOutcome struct {
	Project    config.ProjectConfig
	PR         *gitprovider.PullRequest
	Result     *ai.ReviewResult
	Diff       string
	Commented  bool
	SavedFile  string
	LoadedFile string
}

// GetRepoIdentifier determines the repository name to use for review files
func GetRepoIdentifier(project config.ProjectConfig) string {
	if project.Repo != "" {
		return strings.ReplaceAll(project.Repo, "/", "-")
	}
	if project.ProjectPath != "" {
		parts := strings.Split(strings.Trim(project.ProjectPath, "/"), "/")
		return parts[len(parts)-1]
	}
	if project.ID != "" {
		return strings.ReplaceAll(project.ID, "/", "-")
	}
	return "repo"
}

// ReviewMetadata holds information about which AI generated the review
type ReviewMetadata struct {
	Provider  string    `yaml:"provider"`
	Model     string    `yaml:"model"`
	CreatedAt time.Time `yaml:"created_at"`
	Project   string    `yaml:"project,omitempty"`
	PR        int       `yaml:"pr,omitempty"`
}

// SavedReview represents an existing review on disk
type SavedReview struct {
	FilePath   string
	FileName   string
	Metadata   ReviewMetadata
	Provider   string
	Model      string
	CreatedAt  time.Time
	Content    string // Review body without frontmatter
	RawContent string // Raw file content including frontmatter
}

// SanitizeForFilename cleans model/provider strings for use in filenames
func SanitizeForFilename(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else if r == ':' || r == '/' || r == '.' || r == '_' {
			b.WriteRune('-')
		}
	}
	res := b.String()
	for strings.Contains(res, "--") {
		res = strings.ReplaceAll(res, "--", "-")
	}
	return strings.Trim(res, "-")
}

// FormatReviewMarkdown prepends YAML frontmatter containing provider and model metadata
func FormatReviewMarkdown(meta ReviewMetadata, content string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("provider: %q\n", meta.Provider))
	b.WriteString(fmt.Sprintf("model: %q\n", meta.Model))
	b.WriteString(fmt.Sprintf("created_at: %q\n", meta.CreatedAt.UTC().Format(time.RFC3339)))
	if meta.Project != "" {
		b.WriteString(fmt.Sprintf("project: %q\n", meta.Project))
	}
	if meta.PR > 0 {
		b.WriteString(fmt.Sprintf("pr: %d\n", meta.PR))
	}
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimSpace(content))
	b.WriteString("\n")
	return b.String()
}

// ParseReviewMarkdown extracts YAML frontmatter (if present) from review content
func ParseReviewMarkdown(raw string) (ReviewMetadata, string) {
	meta := ReviewMetadata{}
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "---") {
		parts := strings.SplitN(trimmed[3:], "---", 2)
		if len(parts) == 2 {
			frontmatter := parts[0]
			body := strings.TrimSpace(parts[1])
			if err := yaml.Unmarshal([]byte(frontmatter), &meta); err == nil && (meta.Provider != "" || meta.Model != "") {
				return meta, body
			}
		}
	}
	return meta, raw
}

func inferProviderFromFilename(baseName, prefix string) string {
	rest := strings.TrimPrefix(baseName, prefix)
	rest = strings.TrimPrefix(rest, "-")
	if rest == "" {
		return ""
	}
	parts := strings.Split(rest, "-")
	return parts[0]
}

func inferModelFromFilename(baseName, prefix string) string {
	rest := strings.TrimPrefix(baseName, prefix)
	rest = strings.TrimPrefix(rest, "-")
	if rest == "" {
		return ""
	}
	parts := strings.Split(rest, "-")
	if len(parts) > 1 {
		return strings.Join(parts[1:], "-")
	}
	return ""
}

// ReviewFileName returns the standard filename for a PR review (repository-pr-number.md)
func ReviewFileName(project config.ProjectConfig, prNumber int) string {
	repo := GetRepoIdentifier(project)
	return fmt.Sprintf("%s-%d.md", repo, prNumber)
}

// ReviewFileNameWithAI returns a filename tagged with AI provider and model
func ReviewFileNameWithAI(project config.ProjectConfig, prNumber int, provider string, model string) string {
	repo := GetRepoIdentifier(project)
	if provider != "" {
		cleanProvider := SanitizeForFilename(provider)
		if model != "" {
			cleanModel := SanitizeForFilename(model)
			return fmt.Sprintf("%s-%d-%s-%s.md", repo, prNumber, cleanProvider, cleanModel)
		}
		return fmt.Sprintf("%s-%d-%s.md", repo, prNumber, cleanProvider)
	}
	return fmt.Sprintf("%s-%d.md", repo, prNumber)
}

// ReviewFilePath returns the path where the review file is stored based on configuration
func (s *Service) ReviewFilePath(project config.ProjectConfig, prNumber int) string {
	fileName := ReviewFileName(project, prNumber)
	if s.cfg != nil && s.cfg.ReviewsDir != "" {
		dir := config.ExpandPath(s.cfg.ReviewsDir)
		return filepath.Join(dir, fileName)
	}
	return fileName
}

// ReviewFilePathWithAI returns the path in reviews_dir with provider/model tagging
func (s *Service) ReviewFilePathWithAI(project config.ProjectConfig, prNumber int, provider string, model string) string {
	fileName := ReviewFileNameWithAI(project, prNumber, provider, model)
	if s.cfg != nil && s.cfg.ReviewsDir != "" {
		dir := config.ExpandPath(s.cfg.ReviewsDir)
		return filepath.Join(dir, fileName)
	}
	return fileName
}

// ListReviewsForPR finds all saved review files on disk for a given PR across providers
func (s *Service) ListReviewsForPR(project config.ProjectConfig, prNumber int) []SavedReview {
	repo := GetRepoIdentifier(project)
	prefix := fmt.Sprintf("%s-%d", repo, prNumber)

	dirsToScan := []string{}
	if s.cfg != nil && s.cfg.ReviewsDir != "" {
		dirsToScan = append(dirsToScan, config.ExpandPath(s.cfg.ReviewsDir))
	}
	dirsToScan = append(dirsToScan, ".")

	seenPaths := make(map[string]bool)
	var reviews []SavedReview

	for _, dir := range dirsToScan {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}

			baseName := strings.TrimSuffix(entry.Name(), ".md")
			if baseName != prefix && !strings.HasPrefix(baseName, prefix+"-") {
				continue
			}

			fullPath := filepath.Join(dir, entry.Name())
			absPath, err := filepath.Abs(fullPath)
			if err == nil {
				if seenPaths[absPath] {
					continue
				}
				seenPaths[absPath] = true
			}

			data, err := os.ReadFile(fullPath)
			if err != nil || len(strings.TrimSpace(string(data))) == 0 {
				continue
			}

			info, _ := entry.Info()
			modTime := time.Now()
			if info != nil {
				modTime = info.ModTime()
			}

			meta, body := ParseReviewMarkdown(string(data))
			if meta.CreatedAt.IsZero() {
				meta.CreatedAt = modTime
			}
			if meta.Provider == "" {
				meta.Provider = inferProviderFromFilename(baseName, prefix)
			}
			if meta.Model == "" {
				meta.Model = inferModelFromFilename(baseName, prefix)
			}

			reviews = append(reviews, SavedReview{
				FilePath:   fullPath,
				FileName:   entry.Name(),
				Metadata:   meta,
				Provider:   meta.Provider,
				Model:      meta.Model,
				CreatedAt:  meta.CreatedAt,
				Content:    body,
				RawContent: string(data),
			})
		}
	}

	sort.Slice(reviews, func(i, j int) bool {
		return reviews[i].CreatedAt.After(reviews[j].CreatedAt)
	})

	return reviews
}

// GetCachedReview searches for existing review markdown files in reviews_dir and fallback paths
func (s *Service) GetCachedReview(project config.ProjectConfig, prNumber int) (string, string, bool) {
	reviews := s.ListReviewsForPR(project, prNumber)
	if len(reviews) > 0 {
		return reviews[0].Content, reviews[0].FilePath, true
	}
	return "", s.ReviewFilePath(project, prNumber), false
}

// ReviewPR runs the AI review pipeline for a given project and PR number
func (s *Service) ReviewPR(ctx context.Context, project config.ProjectConfig, prNumber int, opts ReviewOptions) (*ReviewOutcome, error) {
	provider, err := s.gitManager.GetProvider(project)
	if err != nil {
		return nil, fmt.Errorf("failed to get git provider: %w", err)
	}

	pr, err := provider.GetPullRequest(ctx, project, prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR details: %w", err)
	}

	outputFile := opts.OutputFile
	if outputFile == "" {
		outputFile = s.ReviewFilePath(project, prNumber)
	}

	outcome := &ReviewOutcome{
		Project: project,
		PR:      pr,
	}

	var result *ai.ReviewResult

	// When --post is used without --force, check if review file exists before using AI
	if opts.PostComment && !opts.Force {
		if content, path, exists := s.GetCachedReview(project, prNumber); exists {
			outcome.LoadedFile = path
			result = &ai.ReviewResult{
				Content:  content,
				Provider: "local-file",
				Model:    path,
			}
		}
	}

	// If no existing review file was loaded, run AI review
	if result == nil {
		diff, err := provider.GetDiff(ctx, project, prNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to get PR diff: %w", err)
		}

		if strings.TrimSpace(diff) == "" {
			return nil, fmt.Errorf("PR #%d has an empty diff", prNumber)
		}
		outcome.Diff = diff

		engine, err := s.aiFactory.GetEngine(opts.AIProvider)
		if err != nil {
			return nil, fmt.Errorf("failed to get AI engine: %w", err)
		}

		guidelines := s.cfg.ReviewGuidelines
		if opts.Guidelines != "" {
			guidelines = opts.Guidelines
		} else if opts.GuidelinesFile != "" {
			data, err := os.ReadFile(opts.GuidelinesFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read review guidelines file %s: %w", opts.GuidelinesFile, err)
			}
			guidelines = string(data)
		}

		req := ai.ReviewRequest{
			PR:                 pr,
			Diff:               diff,
			Guidelines:         guidelines,
			ProjectID:          project.ID,
			ProjectDescription: project.Description,
			Model:              opts.Model,
		}

		aiResult, err := engine.Review(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("AI review failed (%s): %w", engine.Name(), err)
		}
		result = aiResult

		// If outputFile was not explicitly overridden by flag, tag it with provider & model
		if opts.OutputFile == "" {
			outputFile = s.ReviewFilePathWithAI(project, prNumber, result.Provider, result.Model)
		}

		// Save the generated review with YAML frontmatter metadata
		meta := ReviewMetadata{
			Provider:  result.Provider,
			Model:     result.Model,
			CreatedAt: time.Now(),
			Project:   project.ID,
			PR:        prNumber,
		}
		formattedContent := FormatReviewMarkdown(meta, result.Content)

		dir := filepath.Dir(outputFile)
		if dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0755)
		}
		if err := os.WriteFile(outputFile, []byte(formattedContent), 0644); err == nil {
			outcome.SavedFile = outputFile
		}
	}

	outcome.Result = result

	if opts.PostComment {
		var commentBody string
		if outcome.LoadedFile != "" {
			commentBody = fmt.Sprintf("🤖 **AI Code Review** (Model: `%s` / `%s`)\n\n%s", result.Provider, result.Model, result.Content)
		} else {
			commentBody = fmt.Sprintf("🤖 **AI Code Review** (Generated by `%s` / `%s`)\n\n%s", result.Provider, result.Model, result.Content)
		}
		if err := provider.PostComment(ctx, project, prNumber, commentBody); err != nil {
			return outcome, fmt.Errorf("review ready, but failed to post comment to %s: %w", provider.Name(), err)
		}
		outcome.Commented = true
	}

	return outcome, nil
}

// DeleteReviewFile removes a specific review file from disk
func (s *Service) DeleteReviewFile(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("empty file path")
	}
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete review file %s: %w", filePath, err)
	}
	return nil
}

// DeleteReviewsForPR removes all saved review files for a given PR
func (s *Service) DeleteReviewsForPR(project config.ProjectConfig, prNumber int) (int, error) {
	reviews := s.ListReviewsForPR(project, prNumber)
	deleted := 0
	var firstErr error
	for _, rev := range reviews {
		if err := s.DeleteReviewFile(rev.FilePath); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		} else {
			deleted++
		}
	}
	return deleted, firstErr
}

// DeleteAllReviews removes all saved reviews. If project is non-nil, only reviews for that project are removed.
func (s *Service) DeleteAllReviews(project *config.ProjectConfig) (int, error) {
	dirsToScan := []string{}
	if s.cfg != nil && s.cfg.ReviewsDir != "" {
		dirsToScan = append(dirsToScan, config.ExpandPath(s.cfg.ReviewsDir))
	}
	dirsToScan = append(dirsToScan, ".")

	var projectPrefix string
	if project != nil {
		projectPrefix = GetRepoIdentifier(*project) + "-"
	}

	seenPaths := make(map[string]bool)
	deleted := 0
	var firstErr error

	for _, dir := range dirsToScan {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}

			// If scanning current dir '.', only remove files matching review patterns
			if dir == "." {
				if projectPrefix != "" {
					if !strings.HasPrefix(entry.Name(), projectPrefix) {
						continue
					}
				} else {
					// Check if entry matches any known project prefix or has frontmatter
					isReviewFile := false
					if s.cfg != nil {
						for _, p := range s.cfg.Projects {
							if strings.HasPrefix(entry.Name(), GetRepoIdentifier(p)+"-") {
								isReviewFile = true
								break
							}
						}
					}
					if !isReviewFile {
						continue
					}
				}
			} else {
				// In dedicated reviews_dir
				if projectPrefix != "" && !strings.HasPrefix(entry.Name(), projectPrefix) {
					continue
				}
			}

			fullPath := filepath.Join(dir, entry.Name())
			absPath, err := filepath.Abs(fullPath)
			if err == nil {
				if seenPaths[absPath] {
					continue
				}
				seenPaths[absPath] = true
			}

			if err := os.Remove(fullPath); err != nil {
				if firstErr == nil {
					firstErr = err
				}
			} else {
				deleted++
			}
		}
	}

	return deleted, firstErr
}

