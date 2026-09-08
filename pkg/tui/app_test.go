package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/arxcruz/pr-review/pkg/gitprovider"
	"github.com/arxcruz/pr-review/pkg/review"
)

func TestNewModel(t *testing.T) {
	cfg := config.DefaultConfig()
	svc := review.NewService(cfg)

	model := NewModel(cfg, svc)
	if len(model.projects) == 0 {
		t.Fatal("expected at least 1 project in model")
	}

	if model.activePane != panePRList {
		t.Errorf("expected active pane to be PRList (%d), got %d", panePRList, model.activePane)
	}

	if model.activeTab != tabOverview {
		t.Errorf("expected active tab to be Overview (%d), got %d", tabOverview, model.activeTab)
	}
}

func TestModelReviewCachePrecheck(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.ReviewsDir = tmpDir
	svc := review.NewService(cfg)

	model := NewModel(cfg, svc)
	proj := config.ProjectConfig{
		ID:    "test-proj",
		Owner: "myorg",
		Repo:  "repo1",
	}
	model.projects = []config.ProjectConfig{proj}

	// Target review path
	expectedPath := filepath.Join(tmpDir, "repo1-10.md")

	_, path, exists := svc.GetCachedReview(proj, 10)
	if exists {
		t.Fatalf("expected review to not exist yet")
	}
	if path != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, path)
	}
}

func TestKeybindingsNavigation(t *testing.T) {
	cfg := config.DefaultConfig()
	// Custom keybindings (e.g. Moonlander / customized layout)
	cfg.Keybindings.Down = config.KeyList{"n", "down"}
	cfg.Keybindings.Up = config.KeyList{"e", "up"}
	svc := review.NewService(cfg)

	model := NewModel(cfg, svc)

	if !model.matches("n", model.cfg.Keybindings.Down) {
		t.Errorf("expected 'n' to match Down keybinding")
	}
	if !model.matches("down", model.cfg.Keybindings.Down) {
		t.Errorf("expected 'down' to match Down keybinding")
	}
	if !model.matches("e", model.cfg.Keybindings.Up) {
		t.Errorf("expected 'e' to match Up keybinding")
	}
	if model.matches("j", model.cfg.Keybindings.Down) {
		t.Errorf("expected 'j' not to match Down keybinding when overridden")
	}
}

func TestAIProviderSelection(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DefaultAIProvider = "ollama"
	// Add endpoints for anthropic and vllm so the test can select them
	cfg.AI.Endpoints = append(cfg.AI.Endpoints,
		config.AIEndpointConfig{ID: "anthropic", Provider: "anthropic", APIKey: "test"},
		config.AIEndpointConfig{ID: "vllm", Provider: "vllm"},
	)
	svc := review.NewService(cfg)

	model := NewModel(cfg, svc)
	if model.selectedAIProvider != "ollama" {
		t.Errorf("expected initial AI provider to be ollama, got %s", model.selectedAIProvider)
	}

	choice := model.activeAIChoice()
	if choice.ID != "ollama" {
		t.Errorf("expected active choice ID to be ollama, got %s", choice.ID)
	}

	// Change selected provider to anthropic
	model.selectedAIProvider = "anthropic"
	choice = model.activeAIChoice()
	if choice.ID != "anthropic" {
		t.Errorf("expected active choice ID to be anthropic, got %s", choice.ID)
	}
	if choice.DisplayName != "anthropic" {
		t.Errorf("expected DisplayName 'anthropic', got %s", choice.DisplayName)
	}

	// Change selected provider to vllm
	model.selectedAIProvider = "vllm"
	choice = model.activeAIChoice()
	if choice.ID != "vllm" {
		t.Errorf("expected active choice ID to be vllm, got %s", choice.ID)
	}
	if choice.DisplayName != "vllm" {
		t.Errorf("expected DisplayName 'vllm', got %s", choice.DisplayName)
	}

	// Test custom endpoints in TUI model
	cfg2 := config.DefaultConfig()
	cfg2.AI.Endpoints = []config.AIEndpointConfig{
		{
			ID:       "remote-gpu",
			Name:     "GPU Server",
			Provider: "vllm",
			BaseURL:  "http://192.168.1.50:8000/v1",
			Models:   []string{"qwen-coder-32b", "deepseek-coder"},
		},
	}
	model2 := NewModel(cfg2, svc)
	if len(model2.aiProviders) == 0 {
		t.Fatal("expected aiProviders to be populated")
	}

	model2.selectedAIProvider = "remote-gpu:qwen-coder-32b"
	choice2 := model2.activeAIChoice()
	if choice2.Model != "qwen-coder-32b" {
		t.Errorf("expected model qwen-coder-32b, got %s", choice2.Model)
	}
	if choice2.DisplayName != "GPU Server" {
		t.Errorf("expected DisplayName 'GPU Server', got %s", choice2.DisplayName)
	}

	// Verify modal rendering layout
	modalView := model2.renderAISelectorModal("background")
	if !strings.Contains(modalView, "PROVIDER / ENDPOINT") || !strings.Contains(modalView, "MODEL NAME") || !strings.Contains(modalView, "STATUS") {
		t.Errorf("expected modal view to contain table headers, got:\n%s", modalView)
	}
	if !strings.Contains(modalView, "GPU Server") {
		t.Errorf("expected modal view to contain 'GPU Server', got:\n%s", modalView)
	}
}

func TestReviewVersionSelector(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.ReviewsDir = tmpDir
	svc := review.NewService(cfg)

	model := NewModel(cfg, svc)
	model.width = 100
	model.height = 30

	proj := config.ProjectConfig{
		ID:    "test-proj",
		Owner: "myorg",
		Repo:  "repo1",
	}
	model.projects = []config.ProjectConfig{proj}

	// 1. Initially no reviews available -> 'v' gives status message
	model.availableReviews = nil
	updatedModel, _ := model.Update(mockKeyMsg("v"))
	m := updatedModel.(Model)
	if m.reviewSelectorModal {
		t.Errorf("expected reviewSelectorModal to be false when no reviews exist")
	}

	// 2. Add multiple reviews
	m.availableReviews = []review.SavedReview{
		{
			FileName: "repo1-10-ollama-qwen.md",
			Provider: "ollama",
			Model:    "qwen2.5-coder",
			Content:  "Review from Ollama",
		},
		{
			FileName: "repo1-10-anthropic-claude.md",
			Provider: "anthropic",
			Model:    "claude-3-7-sonnet",
			Content:  "Review from Claude",
		},
	}

	// Press 'v' to open selector
	updatedModel, _ = m.Update(mockKeyMsg("v"))
	m = updatedModel.(Model)
	if !m.reviewSelectorModal {
		t.Errorf("expected reviewSelectorModal to be true")
	}
	if m.reviewSelectorCursor != 0 {
		t.Errorf("expected reviewSelectorCursor to be 0, got %d", m.reviewSelectorCursor)
	}

	// Press 'j' or 'down' to move down
	updatedModel, _ = m.Update(mockKeyMsg("j"))
	m = updatedModel.(Model)
	if m.reviewSelectorCursor != 1 {
		t.Errorf("expected cursor to be 1 after moving down, got %d", m.reviewSelectorCursor)
	}

	// Press Enter to select the second review
	updatedModel, _ = m.Update(mockKeyMsg("enter"))
	m = updatedModel.(Model)
	if m.reviewSelectorModal {
		t.Errorf("expected modal to close after selecting")
	}
	if m.selectedReviewIdx != 1 {
		t.Errorf("expected selectedReviewIdx to be 1, got %d", m.selectedReviewIdx)
	}
	if m.reviewContent != "Review from Claude" {
		t.Errorf("expected reviewContent to be %q, got %q", "Review from Claude", m.reviewContent)
	}
}

func TestReviewDeletionTUI(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.ReviewsDir = tmpDir
	svc := review.NewService(cfg)

	proj := config.ProjectConfig{
		ID:    "test-proj",
		Owner: "myorg",
		Repo:  "repo1",
	}

	f1 := filepath.Join(tmpDir, "repo1-10-ollama-model.md")
	f2 := filepath.Join(tmpDir, "repo1-10-anthropic-model.md")
	_ = os.WriteFile(f1, []byte("Ollama review"), 0644)
	_ = os.WriteFile(f2, []byte("Claude review"), 0644)

	model := NewModel(cfg, svc)
	model.width = 100
	model.height = 30
	model.projects = []config.ProjectConfig{proj}

	// Trigger PR selection for PR 10
	model.filteredPRs = []*gitprovider.PullRequest{
		{Number: 10, Title: "Fix bug"},
	}
	model.onPRSelected(0)

	if len(model.availableReviews) != 2 {
		t.Fatalf("expected 2 available reviews, got %d", len(model.availableReviews))
	}

	// 1. Press 'x' to trigger single review deletion modal
	updatedModel, _ := model.Update(mockKeyMsg("x"))
	m := updatedModel.(Model)
	if !m.confirmDeleteModal {
		t.Fatalf("expected confirmDeleteModal to be true")
	}
	if m.deleteTargetAll {
		t.Fatalf("expected deleteTargetAll to be false for 'x'")
	}

	// Confirm with 'y'
	updatedModel, _ = m.Update(mockKeyMsg("y"))
	m = updatedModel.(Model)
	if m.confirmDeleteModal {
		t.Fatalf("expected confirmDeleteModal to be closed after 'y'")
	}
	if len(m.availableReviews) != 1 {
		t.Errorf("expected 1 remaining review after deleting one, got %d", len(m.availableReviews))
	}

	// 2. Press 'X' to trigger delete all reviews for PR modal
	updatedModel, _ = m.Update(mockKeyMsg("X"))
	m = updatedModel.(Model)
	if !m.confirmDeleteModal {
		t.Fatalf("expected confirmDeleteModal to be true for 'X'")
	}
	if !m.deleteTargetAll {
		t.Fatalf("expected deleteTargetAll to be true for 'X'")
	}

	// Confirm with 'y'
	updatedModel, _ = m.Update(mockKeyMsg("y"))
	m = updatedModel.(Model)
	if len(m.availableReviews) != 0 {
		t.Errorf("expected 0 reviews after deleting all, got %d", len(m.availableReviews))
	}
	if m.reviewContent != "" {
		t.Errorf("expected reviewContent to be empty, got %q", m.reviewContent)
	}
}

func TestReviewCycleAndCompareMode(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.ReviewsDir = tmpDir
	svc := review.NewService(cfg)

	proj := config.ProjectConfig{
		ID:    "test-proj",
		Owner: "myorg",
		Repo:  "repo1",
	}

	f1 := filepath.Join(tmpDir, "repo1-42-gemini-flash.md")
	f2 := filepath.Join(tmpDir, "repo1-42-ollama-qwen.md")
	f3 := filepath.Join(tmpDir, "repo1-42-anthropic-claude.md")
	_ = os.WriteFile(f1, []byte("Gemini review for PR 42"), 0644)
	_ = os.WriteFile(f2, []byte("Ollama review for PR 42"), 0644)
	_ = os.WriteFile(f3, []byte("Claude review for PR 42"), 0644)

	model := NewModel(cfg, svc)
	model.width = 120
	model.height = 35
	model.projects = []config.ProjectConfig{proj}

	// Select PR 42
	model.filteredPRs = []*gitprovider.PullRequest{
		{Number: 42, Title: "Add awesome feature"},
	}
	model.onPRSelected(0)

	if len(model.availableReviews) != 3 {
		t.Fatalf("expected 3 available reviews, got %d", len(model.availableReviews))
	}
	if model.selectedReviewIdx != 0 {
		t.Errorf("expected initial selectedReviewIdx to be 0, got %d", model.selectedReviewIdx)
	}

	// 1. Test NextReview key ']' to cycle to review 1 (Ollama)
	updatedModel, _ := model.Update(mockKeyMsg("]"))
	m := updatedModel.(Model)
	if m.selectedReviewIdx != 1 {
		t.Errorf("expected selectedReviewIdx 1 after ']', got %d", m.selectedReviewIdx)
	}

	// 2. Test NextReview key ']' again to cycle to review 2 (Claude)
	updatedModel, _ = m.Update(mockKeyMsg("]"))
	m = updatedModel.(Model)
	if m.selectedReviewIdx != 2 {
		t.Errorf("expected selectedReviewIdx 2 after ']', got %d", m.selectedReviewIdx)
	}

	// 3. Test PrevReview key '[' to cycle back to review 1
	updatedModel, _ = m.Update(mockKeyMsg("["))
	m = updatedModel.(Model)
	if m.selectedReviewIdx != 1 {
		t.Errorf("expected selectedReviewIdx 1 after '[', got %d", m.selectedReviewIdx)
	}

	// 4. Test CompareReviews key 'c' to enter side-by-side comparison mode
	updatedModel, _ = m.Update(mockKeyMsg("c"))
	m = updatedModel.(Model)
	if !m.compareMode {
		t.Fatalf("expected compareMode to be true after pressing 'c'")
	}
	if m.compareActiveCol != 0 {
		t.Errorf("expected compareActiveCol to be 0 initially, got %d", m.compareActiveCol)
	}

	// Switch column with 'tab'
	m.activePane = paneDetail
	m.activeTab = tabReview
	updatedModel, _ = m.Update(mockKeyMsg("tab"))
	m = updatedModel.(Model)
	if m.compareActiveCol != 1 {
		t.Errorf("expected compareActiveCol to be 1 after tab, got %d", m.compareActiveCol)
	}

	// Exit compare mode with 'c'
	updatedModel, _ = m.Update(mockKeyMsg("c"))
	m = updatedModel.(Model)
	if m.compareMode {
		t.Errorf("expected compareMode to be false after pressing 'c' again")
	}
}

func mockKeyMsg(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}
