// Package config
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DefaultAIProvider string            `yaml:"default_ai_provider"`
	ReviewFile        string            `yaml:"review_file,omitempty"`
	ReviewsDir        string            `yaml:"reviews_dir,omitempty"`
	ReviewGuidelines  string            `yaml:"-"`
	AI                AIConfig          `yaml:"ai"`
	Git               GitConfig         `yaml:"git"`
	Keybindings       KeybindingsConfig `yaml:"keybindings"`
	Projects          []ProjectConfig   `yaml:"projects"`
	Jira              JiraConfig        `yaml:"jira,omitempty"`
}

type AIEndpointConfig struct {
	ID          string   `yaml:"id,omitempty"`          // Unique identifier for this endpoint/instance
	Name        string   `yaml:"name,omitempty"`        // Friendly display name (e.g. "Work GPU vLLM")
	Provider    string   `yaml:"provider"`              // "ollama", "vllm", "llamacpp", "anthropic", "gemini", "openai"
	BaseURL     string   `yaml:"base_url,omitempty"`    // Custom endpoint URL
	APIKey      string   `yaml:"api_key,omitempty"`     // Optional API key
	Model       string   `yaml:"model,omitempty"`       // Single model name
	Models      []string `yaml:"models,omitempty"`      // Multiple models available on this endpoint
	Temperature float64  `yaml:"temperature,omitempty"` // Temperature override
	MaxTokens   int      `yaml:"max_tokens,omitempty"`  // Max tokens override
}

type AIConfig struct {
	Endpoints []AIEndpointConfig `yaml:"endpoints,omitempty"`
}

// OpenAICompatibleConfig is used by providers that speak the OpenAI chat-completions wire format:
// ollama, vllm, llamacpp, openai, and any other compatible server.
type OpenAICompatibleConfig struct {
	BaseURL     string  `yaml:"base_url"`
	APIKey      string  `yaml:"api_key"`
	Model       string  `yaml:"model"`
	Temperature float64 `yaml:"temperature"`
}

type AnthropicConfig struct {
	BaseURL     string   `yaml:"base_url"`
	APIKey      string   `yaml:"api_key"`
	Model       string   `yaml:"model"`
	Models      []string `yaml:"models,omitempty"`
	Temperature float64  `yaml:"temperature"`
	MaxTokens   int      `yaml:"max_tokens"`
}

type GeminiConfig struct {
	BaseURL     string   `yaml:"base_url"`
	APIKey      string   `yaml:"api_key"`
	Model       string   `yaml:"model"`
	Models      []string `yaml:"models,omitempty"`
	Temperature float64  `yaml:"temperature"`
}

type GitConfig struct {
	GitHub GitHubConfig `yaml:"github"`
	GitLab GitLabConfig `yaml:"gitlab"`
	Gerrit GerritConfig `yaml:"gerrit"`
}

type GitHubConfig struct {
	Token   string `yaml:"token"`
	BaseURL string `yaml:"base_url"`
}

type GitLabConfig struct {
	Token   string `yaml:"token"`
	BaseURL string `yaml:"base_url"`
}

type GerritConfig struct {
	BaseURL  string `yaml:"base_url"` // e.g. "https://review.opendev.org"
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type ProjectConfig struct {
	ID             string         `yaml:"id"`              // Unique identifier/alias for the project
	Provider       string         `yaml:"provider"`        // "github", "gitlab", or "gerrit"
	Owner          string         `yaml:"owner"`           // For github (e.g. "arxcruz")
	Repo           string         `yaml:"repo"`            // For github (e.g. "pr-review") or gerrit project name
	ProjectPath    string         `yaml:"project_path"`    // For gitlab (e.g. "group/subgroup/project" or "123456")
	GerritURL      string         `yaml:"gerrit_url"`      // Optional per-project Gerrit URL override
	Description    string         `yaml:"description,omitempty"` // Description / architecture context for AI reviews
	DefaultFilters FilterCriteria `yaml:"default_filters"` // Default filter settings
}

type FilterCriteria struct {
	Labels  []string `yaml:"labels"`
	Authors []string `yaml:"authors"`
}

// KeyList represents a list of key names, unmarshalable from either a single string or slice of strings
type KeyList []string

// UnmarshalYAML supports parsing either a single string or a list of strings
func (k *KeyList) UnmarshalYAML(value *yaml.Node) error {
	var single string
	if err := value.Decode(&single); err == nil {
		*k = []string{single}
		return nil
	}
	var multi []string
	if err := value.Decode(&multi); err == nil {
		*k = multi
		return nil
	}
	return fmt.Errorf("failed to parse keybinding: expected string or list of strings")
}

// KeybindingsConfig allows complete customization of all TUI keyboard shortcuts
type KeybindingsConfig struct {
	Up           KeyList `yaml:"up"`
	Down         KeyList `yaml:"down"`
	PageUp       KeyList `yaml:"page_up"`
	PageDown     KeyList `yaml:"page_down"`
	HalfPageUp   KeyList `yaml:"half_page_up"`
	HalfPageDown KeyList `yaml:"half_page_down"`
	Top          KeyList `yaml:"top"`
	Bottom       KeyList `yaml:"bottom"`
	SwitchPane   KeyList `yaml:"switch_pane"`
	TabOverview  KeyList `yaml:"tab_overview"`
	TabDiff      KeyList `yaml:"tab_diff"`
	TabReview    KeyList `yaml:"tab_review"`
	RunReview    KeyList `yaml:"run_review"`
	PostReview   KeyList `yaml:"post_review"`
	NextProject  KeyList `yaml:"next_project"`
	Refresh      KeyList `yaml:"refresh"`
	Filter       KeyList `yaml:"filter"`
	SelectAI         KeyList `yaml:"select_ai"`
	SelectReview     KeyList `yaml:"select_review"`
	PrevReview       KeyList `yaml:"prev_review"`
	NextReview       KeyList `yaml:"next_review"`
	CompareReviews   KeyList `yaml:"compare_reviews"`
	DeleteReview     KeyList `yaml:"delete_review"`
	DeleteAllReviews KeyList `yaml:"delete_all_reviews"`
	Help             KeyList `yaml:"help"`
	Quit             KeyList `yaml:"quit"`
}

// DefaultKeybindings returns sensible vim-inspired defaults
func DefaultKeybindings() KeybindingsConfig {
	return KeybindingsConfig{
		Up:               KeyList{"k", "up"},
		Down:             KeyList{"j", "down"},
		PageUp:           KeyList{"ctrl+b", "pgup"},
		PageDown:         KeyList{"ctrl+f", "pgdown"},
		HalfPageUp:       KeyList{"ctrl+u"},
		HalfPageDown:     KeyList{"ctrl+d"},
		Top:              KeyList{"g", "home"},
		Bottom:           KeyList{"G", "end"},
		SwitchPane:       KeyList{"tab", "shift+tab"},
		TabOverview:      KeyList{"1"},
		TabDiff:          KeyList{"2", "d"},
		TabReview:        KeyList{"3"},
		RunReview:        KeyList{"r"},
		PostReview:       KeyList{"P"},
		NextProject:      KeyList{"p"},
		Refresh:          KeyList{"R"},
		Filter:           KeyList{"/"},
		SelectAI:         KeyList{"a"},
		SelectReview:     KeyList{"v"},
		PrevReview:       KeyList{"[", "alt+left"},
		NextReview:       KeyList{"]", "alt+right"},
		CompareReviews:   KeyList{"c"},
		DeleteReview:     KeyList{"x", "delete"},
		DeleteAllReviews: KeyList{"X"},
		Help:             KeyList{"?"},
		Quit:             KeyList{"q", "ctrl+c"},
	}
}

func mergeKeyList(target *KeyList, fallback KeyList) {
	if len(*target) == 0 {
		*target = fallback
	}
}

func mergeKeybindings(custom *KeybindingsConfig, defaults KeybindingsConfig) {
	mergeKeyList(&custom.Up, defaults.Up)
	mergeKeyList(&custom.Down, defaults.Down)
	mergeKeyList(&custom.PageUp, defaults.PageUp)
	mergeKeyList(&custom.PageDown, defaults.PageDown)
	mergeKeyList(&custom.HalfPageUp, defaults.HalfPageUp)
	mergeKeyList(&custom.HalfPageDown, defaults.HalfPageDown)
	mergeKeyList(&custom.Top, defaults.Top)
	mergeKeyList(&custom.Bottom, defaults.Bottom)
	mergeKeyList(&custom.SwitchPane, defaults.SwitchPane)
	mergeKeyList(&custom.TabOverview, defaults.TabOverview)
	mergeKeyList(&custom.TabDiff, defaults.TabDiff)
	mergeKeyList(&custom.TabReview, defaults.TabReview)
	mergeKeyList(&custom.RunReview, defaults.RunReview)
	mergeKeyList(&custom.PostReview, defaults.PostReview)
	mergeKeyList(&custom.NextProject, defaults.NextProject)
	mergeKeyList(&custom.Refresh, defaults.Refresh)
	mergeKeyList(&custom.Filter, defaults.Filter)
	mergeKeyList(&custom.SelectAI, defaults.SelectAI)
	mergeKeyList(&custom.SelectReview, defaults.SelectReview)
	mergeKeyList(&custom.PrevReview, defaults.PrevReview)
	mergeKeyList(&custom.NextReview, defaults.NextReview)
	mergeKeyList(&custom.CompareReviews, defaults.CompareReviews)
	mergeKeyList(&custom.DeleteReview, defaults.DeleteReview)
	mergeKeyList(&custom.DeleteAllReviews, defaults.DeleteAllReviews)
	mergeKeyList(&custom.Help, defaults.Help)
	mergeKeyList(&custom.Quit, defaults.Quit)
}

// DefaultReviewGuidelinesText is the fallback review prompt instructions
const DefaultReviewGuidelinesText = `You are an expert senior software engineer and code reviewer.
Conduct a detailed, actionable, and line-specific code review of the pull request diff.

Structure your review into the following sections:

### 1. Summary of Changes
- Concise 2-3 sentence overview of what this PR introduces, modifies, or fixes.

### 2. Line-by-Line & File-Specific Code Suggestions
For every notable issue, bug, optimization, or style improvement found in the diff, format it like:
- **` + "`path/to/file.ext` (around line X / function `FunctionName`)" + `**:
  - **Issue / Rationale**: Explain clearly what the bug, edge case, security issue, or inefficiency is.
  - **Suggested Change**: Provide a clear diff or updated code block showing old vs new code.

Categorize findings by severity:
- 🔴 **Critical / High**: Bugs, security vulnerabilities, memory leaks, concurrency issues, broken tests.
- 🟡 **Medium**: Edge cases, performance bottlenecks, unhandled errors, missing validations.
- 🟢 **Low / Nitpick**: Idiomatic code style, naming conventions, docstrings, slight cleanups.

### 3. Positive Highlights
- Mention good practices, clean abstractions, or well-tested parts in this PR.

### 4. Verdict & Recommendation
- **Verdict**: [APPROVE | REQUEST CHANGES | COMMENT] with a brief closing summary.`

// DefaultReviewGuidelines returns the default review prompt instructions
func DefaultReviewGuidelines() string {
	return DefaultReviewGuidelinesText
}

// DefaultConfig returns a sane default configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultAIProvider: "ollama",
		ReviewsDir:        "reviews",
		ReviewGuidelines:  DefaultReviewGuidelines(),
		Keybindings:       DefaultKeybindings(),
		AI: AIConfig{
			Endpoints: []AIEndpointConfig{
				{
					ID:       "ollama",
					Name:     "Ollama (Local AI)",
					Provider: "ollama",
				},
			},
		},
		Git: GitConfig{
			GitHub: GitHubConfig{
				Token: os.Getenv("GITHUB_TOKEN"),
			},
			GitLab: GitLabConfig{
				Token: os.Getenv("GITLAB_TOKEN"),
			},
		},
		Projects: []ProjectConfig{},
	}
}

// DefaultConfigPaths returns standard config file paths in order of preference
func DefaultConfigPaths() []string {
	paths := []string{
		".pr-review.yaml",
		".pr-review.yml",
		"pr-review.yaml",
		"pr-review.yml",
	}

	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".config", "pr-review", "config.yaml"),
			filepath.Join(home, ".config", "pr-review", "config.yml"),
			filepath.Join(home, ".pr-review.yaml"),
		)
	}

	return paths
}

// ExpandEnvVars replaces ${VAR} or $VAR in text with environment variable values
func ExpandEnvVars(content []byte) []byte {
	str := string(content)
	re := regexp.MustCompile(`\$\{?([A-Za-z0-9_]+)\}?`)
	expanded := re.ReplaceAllStringFunc(str, func(match string) string {
		varName := strings.TrimPrefix(match, "$")
		varName = strings.TrimPrefix(varName, "{")
		varName = strings.TrimSuffix(varName, "}")
		if val, ok := os.LookupEnv(varName); ok {
			return val
		}
		return match
	})
	return []byte(expanded)
}

// ExpandPath expands leading ~ to the user's home directory and expands environment variables
func ExpandPath(path string) string {
	if path == "" {
		return ""
	}
	path = string(ExpandEnvVars([]byte(path)))
	if strings.HasPrefix(path, "~/") || path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// DefaultReviewPaths returns standard paths to search for review guidelines markdown files
func DefaultReviewPaths(configDir string) []string {
	paths := []string{
		"review.md",
		".review.md",
		".pr-review.md",
	}

	if configDir != "" {
		paths = append(paths,
			filepath.Join(configDir, "review.md"),
			filepath.Join(configDir, ".review.md"),
			filepath.Join(configDir, ".pr-review.md"),
		)
	}

	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".config", "pr-review", "review.md"),
			filepath.Join(home, ".review.md"),
		)
	}

	return paths
}

// LoadReviewGuidelines attempts to load review guidelines from a custom path, default paths, or fallback
func LoadReviewGuidelines(customPath string, configDir string) (string, string) {
	if customPath != "" {
		candidates := []string{customPath}
		if !filepath.IsAbs(customPath) && configDir != "" {
			candidates = append(candidates, filepath.Join(configDir, customPath))
		}
		for _, p := range candidates {
			if data, err := os.ReadFile(p); err == nil {
				return string(data), p
			}
		}
	}

	for _, p := range DefaultReviewPaths(configDir) {
		if data, err := os.ReadFile(p); err == nil {
			return string(data), p
		}
	}

	return DefaultReviewGuidelines(), ""
}

// FindConfigFile searches for an existing config file path or returns preferred default path
func FindConfigFile(path string) string {
	if path != "" {
		return path
	}
	for _, p := range DefaultConfigPaths() {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "pr-review", "config.yaml")
	}
	return ".pr-review.yaml"
}

// LoadRawConfig loads configuration without expanding environment variables (preserving ${VAR} references for saving)
func LoadRawConfig(path string) (*Config, string, error) {
	cfg := DefaultConfig()
	targetPath := path
	if targetPath == "" {
		for _, p := range DefaultConfigPaths() {
			if _, err := os.Stat(p); err == nil {
				targetPath = p
				break
			}
		}
	}

	if targetPath == "" {
		return cfg, "", nil
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, targetPath, fmt.Errorf("failed to read config file %s: %w", targetPath, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, targetPath, fmt.Errorf("failed to parse yaml config %s: %w", targetPath, err)
	}

	return cfg, targetPath, nil
}

// LoadConfig loads configuration from a specific path, or scans default paths if path is empty
func LoadConfig(path string) (*Config, string, error) {
	cfg := DefaultConfig()

	var targetPath string
	if path != "" {
		targetPath = path
		if _, err := os.Stat(targetPath); err != nil {
			return nil, "", fmt.Errorf("config file not found at %s: %w", targetPath, err)
		}
	} else {
		for _, p := range DefaultConfigPaths() {
			if _, err := os.Stat(p); err == nil {
				targetPath = p
				break
			}
		}
	}

	if targetPath == "" {
		// Attempt to load review guidelines from default paths even if no config file is found
		guidelines, _ := LoadReviewGuidelines("", "")
		cfg.ReviewGuidelines = guidelines
		return cfg, "", nil
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, targetPath, fmt.Errorf("failed to read config file %s: %w", targetPath, err)
	}

	expandedData := ExpandEnvVars(data)
	if err := yaml.Unmarshal(expandedData, cfg); err != nil {
		return nil, targetPath, fmt.Errorf("failed to parse yaml config %s: %w", targetPath, err)
	}

	// Detect deprecated nested provider config format and emit a hard error
	if err := detectDeprecatedAIConfig(expandedData); err != nil {
		return nil, targetPath, fmt.Errorf("config file %s uses a deprecated format: %w", targetPath, err)
	}

	if cfg.ReviewsDir != "" {
		cfg.ReviewsDir = ExpandPath(cfg.ReviewsDir)
	}

	configDir := filepath.Dir(targetPath)
	if cfg.ReviewFile != "" {
		reviewFilePath := cfg.ReviewFile
		if !filepath.IsAbs(reviewFilePath) {
			if _, err := os.Stat(reviewFilePath); err != nil {
				candidate := filepath.Join(configDir, reviewFilePath)
				if _, err2 := os.Stat(candidate); err2 == nil {
					reviewFilePath = candidate
				}
			}
		}
		guidelineData, err := os.ReadFile(reviewFilePath)
		if err != nil {
			return nil, targetPath, fmt.Errorf("failed to read review file %s: %w", cfg.ReviewFile, err)
		}
		cfg.ReviewGuidelines = string(guidelineData)
	} else {
		guidelines, _ := LoadReviewGuidelines("", configDir)
		cfg.ReviewGuidelines = guidelines
	}

	// Fallback to environment variables if tokens are still empty
	if cfg.Git.GitHub.Token == "" {
		cfg.Git.GitHub.Token = os.Getenv("GITHUB_TOKEN")
	}
	if cfg.Git.GitLab.Token == "" {
		cfg.Git.GitLab.Token = os.Getenv("GITLAB_TOKEN")
	}
	if cfg.Git.Gerrit.BaseURL == "" {
		cfg.Git.Gerrit.BaseURL = os.Getenv("GERRIT_BASE_URL")
	}
	if cfg.Git.Gerrit.Username == "" {
		cfg.Git.Gerrit.Username = os.Getenv("GERRIT_USERNAME")
	}
	if cfg.Git.Gerrit.Password == "" {
		cfg.Git.Gerrit.Password = os.Getenv("GERRIT_PASSWORD")
	}

	// Jira configuration defaults and environment variable fallback
	cfg.Jira.ApplyEnvAndDefaults()

	// Inject API keys from environment into endpoints that lack them
	for i := range cfg.AI.Endpoints {
		if cfg.AI.Endpoints[i].APIKey != "" {
			continue
		}
		switch strings.ToLower(cfg.AI.Endpoints[i].Provider) {
		case "anthropic", "claude":
			cfg.AI.Endpoints[i].APIKey = os.Getenv("ANTHROPIC_API_KEY")
		case "gemini", "google":
			cfg.AI.Endpoints[i].APIKey = os.Getenv("GEMINI_API_KEY")
		case "openai":
			cfg.AI.Endpoints[i].APIKey = os.Getenv("OPENAI_API_KEY")
		}
	}

	mergeKeybindings(&cfg.Keybindings, DefaultKeybindings())

	return cfg, targetPath, nil
}

// SaveConfig writes the given configuration to a YAML file
func SaveConfig(cfg *Config, targetPath string) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(targetPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", targetPath, err)
	}

	return nil
}

// AITarget represents a resolved, selectable AI endpoint & model combination
type AITarget struct {
	ID          string  `yaml:"id"`          // Unique target ID (e.g. "ollama:qwen2.5-coder:latest" or "remote-vllm:qwen-32b")
	EndpointID  string  `yaml:"endpoint_id"` // Optional parent endpoint ID
	Name        string  `yaml:"name"`        // Friendly display name
	Provider    string  `yaml:"provider"`    // "ollama", "vllm", "llamacpp", "anthropic", "gemini", "openai"
	BaseURL     string  `yaml:"base_url"`
	APIKey      string  `yaml:"api_key"`
	Model       string  `yaml:"model"`
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
	Configured  bool    `yaml:"configured"`
}

// GetAllAITargets resolves all configured AI endpoints and models into a slice of selectable AITargets
func GetAllAITargets(cfg *Config) []AITarget {
	var targets []AITarget
	seen := make(map[string]bool)

	addTarget := func(t AITarget) {
		key := fmt.Sprintf("%s|%s|%s", t.Provider, t.BaseURL, t.Model)
		if seen[key] {
			return
		}
		seen[key] = true
		targets = append(targets, t)
	}

	for _, ep := range cfg.AI.Endpoints {
		prov := strings.ToLower(strings.TrimSpace(ep.Provider))
		if prov == "" {
			prov = "openai"
		}

		baseURL := ep.BaseURL
		if baseURL == "" {
			baseURL = DefaultBaseURLForProvider(prov)
		}

		temp := ep.Temperature
		if temp <= 0 {
			temp = 0.2
		}

		models := collectModels(ep.Model, ep.Models, defaultModelForProvider(prov))
		name := ep.Name
		if name == "" {
			if ep.ID != "" {
				name = ep.ID
			} else {
				name = defaultProviderName(prov)
			}
		}

		for _, m := range models {
			targetID := ep.ID
			if targetID == "" {
				targetID = fmt.Sprintf("%s:%s", prov, m)
			} else if len(models) > 1 {
				targetID = fmt.Sprintf("%s:%s", ep.ID, m)
			}

			isConfigured := checkTargetConfigured(prov, baseURL, ep.APIKey)
			addTarget(AITarget{
				ID:          targetID,
				EndpointID:  ep.ID,
				Name:        name,
				Provider:    prov,
				BaseURL:     baseURL,
				APIKey:      ep.APIKey,
				Model:       m,
				Temperature: temp,
				MaxTokens:   ep.MaxTokens,
				Configured:  isConfigured,
			})
		}
	}

	return targets
}

// detectDeprecatedAIConfig checks raw YAML data for deprecated nested provider keys
// (ai.ollama, ai.vllm, ai.llamacpp, ai.openai, ai.anthropic, ai.gemini) and returns
// a hard error telling the user to migrate to the ai.endpoints[] format.
func detectDeprecatedAIConfig(data []byte) error {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil // let the main unmarshal report the error
	}

	aiRaw, ok := raw["ai"]
	if !ok {
		return nil
	}

	aiMap, ok := aiRaw.(map[string]interface{})
	if !ok {
		return nil
	}

	deprecatedKeys := []string{"ollama", "vllm", "llamacpp", "openai", "anthropic", "gemini"}
	var found []string
	for _, key := range deprecatedKeys {
		if _, exists := aiMap[key]; exists {
			found = append(found, "ai."+key)
		}
	}

	if len(found) == 0 {
		return nil
	}

	return fmt.Errorf(
		"found deprecated nested AI provider keys: %s. "+
			"These are no longer supported. Please migrate to the ai.endpoints[] format. "+
			"Example:\n"+
			"  ai:\n"+
			"    endpoints:\n"+
			"      - id: my-ollama\n"+
			"        provider: ollama\n"+
			"        model: qwen2.5-coder:latest\n"+
			"      - id: my-anthropic\n"+
			"        provider: anthropic\n"+
			"        api_key: ${ANTHROPIC_API_KEY}\n"+
			"        model: claude-3-7-sonnet-20250219",
		strings.Join(found, ", "),
	)
}

func collectModels(single string, list []string, fallback string) []string {
	var result []string
	for _, m := range list {
		if m = strings.TrimSpace(m); m != "" {
			result = append(result, m)
		}
	}
	if single = strings.TrimSpace(single); single != "" {
		found := false
		for _, m := range result {
			if m == single {
				found = true
				break
			}
		}
		if !found {
			result = append([]string{single}, result...)
		}
	}
	if len(result) == 0 && fallback != "" {
		result = []string{fallback}
	}
	return result
}

func defaultModelForProvider(prov string) string {
	switch strings.ToLower(prov) {
	case "ollama":
		return "qwen2.5-coder:latest"
	case "anthropic", "claude":
		return "claude-3-7-sonnet-20250219"
	case "gemini", "google":
		return "gemini-2.5-flash"
	case "openai":
		return "gpt-4o"
	case "vllm", "llamacpp", "llama":
		return "default"
	default:
		return "default"
	}
}

// DefaultBaseURLForProvider returns standard default URL for known providers
func DefaultBaseURLForProvider(prov string) string {
	switch strings.ToLower(prov) {
	case "ollama":
		return "http://localhost:11434/v1"
	case "vllm":
		return "http://localhost:8000/v1"
	case "llamacpp", "llama":
		return "http://localhost:8080/v1"
	case "anthropic", "claude":
		return "https://api.anthropic.com"
	case "gemini", "google":
		return "https://generativelanguage.googleapis.com"
	case "openai":
		return "https://api.openai.com/v1"
	default:
		return "http://localhost:8000/v1"
	}
}

func defaultProviderName(prov string) string {
	switch strings.ToLower(prov) {
	case "ollama":
		return "Ollama"
	case "vllm":
		return "vLLM"
	case "llamacpp", "llama":
		return "llama.cpp"
	case "anthropic", "claude":
		return "Anthropic Claude"
	case "gemini", "google":
		return "Google Gemini"
	case "openai":
		return "OpenAI"
	default:
		return strings.ToUpper(prov)
	}
}

func checkTargetConfigured(prov, baseURL, apiKey string) bool {
	switch strings.ToLower(prov) {
	case "anthropic", "claude", "gemini", "google":
		return apiKey != ""
	case "openai":
		return apiKey != "" || strings.Contains(baseURL, "localhost") || strings.Contains(baseURL, "127.0.0.1")
	default:
		return true
	}
}
