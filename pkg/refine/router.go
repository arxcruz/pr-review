package refine

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/arxcruz/pr-review/pkg/ai"
	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/arxcruz/pr-review/pkg/session"
)

// RouterOption configures a Router instance.
type RouterOption func(*Router)

// WithAIEngine configures an AI engine for recommendations.
func WithAIEngine(engine ai.Engine) RouterOption {
	return func(r *Router) {
		r.aiEngine = engine
	}
}

// WithModel configures the model name for AI recommendations.
func WithModel(model string) RouterOption {
	return func(r *Router) {
		r.model = model
	}
}

// WithInteractive enables user prompts when no mapping rule applies.
func WithInteractive(in io.Reader, out io.Writer) RouterOption {
	return func(r *Router) {
		r.interactive = true
		r.in = in
		r.out = out
	}
}

// Router inspects a DecompositionTree and assigns work items to Delivery Projects and issue types.
type Router struct {
	cfg         *config.JiraConfig
	aiEngine    ai.Engine
	model       string
	interactive bool
	in          io.Reader
	out         io.Writer
}

// NewRouter creates a new project routing engine.
func NewRouter(cfg *config.JiraConfig, opts ...RouterOption) *Router {
	r := &Router{
		cfg: cfg,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Route inspects the snapshot's DecompositionTree and assigns Epics and Tasks
// to appropriate Delivery Projects and issue types, saving changes to the snapshot.
func (r *Router) Route(ctx context.Context, snap *session.Snapshot) error {
	if snap == nil {
		return fmt.Errorf("session snapshot is required")
	}
	if snap.Tree == nil {
		return fmt.Errorf("decomposition tree is required")
	}

	defaultOrigin := r.defaultOriginProject()

	for i := range snap.Tree.Epics {
		epic := &snap.Tree.Epics[i]

		var matchedTeam *config.JiraTeamConfig
		// 1. Check existing delivery project
		if epic.DeliveryProject != "" {
			_, matchedTeam = r.findTeamByDeliveryProject(epic.DeliveryProject)
		}

		// 2. Rule matching
		if matchedTeam == nil && epic.DeliveryProject == "" {
			_, matchedTeam = r.matchTeamByRules(epic.Title, epic.Description)
			if matchedTeam != nil {
				epic.DeliveryProject = matchedTeam.DeliveryProject
			}
		}

		// 3. AI recommendation
		if matchedTeam == nil && epic.DeliveryProject == "" && r.aiEngine != nil {
			aiProj := r.recommendViaAI(ctx, epic.ID, epic.Title, epic.Description)
			if aiProj != "" {
				epic.DeliveryProject = aiProj
				_, matchedTeam = r.findTeamByDeliveryProject(aiProj)
			}
		}

		// 4. Interactive prompt or fallback
		if epic.DeliveryProject == "" {
			if r.interactive && r.in != nil {
				chosen := r.promptUserForDeliveryProject("Epic", epic.ID, epic.Title)
				epic.DeliveryProject = chosen
				_, matchedTeam = r.findTeamByDeliveryProject(chosen)
			} else {
				epic.DeliveryProject = defaultOrigin
			}
		}

		// Issue type assignment for Epic
		if matchedTeam != nil && matchedTeam.IssueTypes.Epic != "" {
			epic.Type = matchedTeam.IssueTypes.Epic
		} else if epic.Type == "" {
			epic.Type = "Epic"
		}

		// Route tasks under this epic
		for j := range epic.Tasks {
			task := &epic.Tasks[j]
			var taskTeam *config.JiraTeamConfig

			// 1. Check existing
			if task.DeliveryProject != "" {
				_, taskTeam = r.findTeamByDeliveryProject(task.DeliveryProject)
			}

			// 2. Rule matching on task
			if taskTeam == nil && task.DeliveryProject == "" {
				_, taskTeam = r.matchTeamByRules(task.Title, task.Description)
				if taskTeam != nil {
					task.DeliveryProject = taskTeam.DeliveryProject
				}
			}

			// 3. AI recommendation for task if still unmapped
			if taskTeam == nil && task.DeliveryProject == "" && r.aiEngine != nil {
				aiProj := r.recommendViaAI(ctx, task.ID, task.Title, task.Description)
				if aiProj != "" {
					task.DeliveryProject = aiProj
					_, taskTeam = r.findTeamByDeliveryProject(aiProj)
				}
			}

			// 4. Interactive prompt fallback
			if task.DeliveryProject == "" && r.interactive && r.in != nil {
				chosen := r.promptUserForDeliveryProject("Task", task.ID, task.Title)
				task.DeliveryProject = chosen
				_, taskTeam = r.findTeamByDeliveryProject(chosen)
			}

			// 5. Inherit from parent epic or fallback to default origin
			if task.DeliveryProject == "" {
				if matchedTeam != nil {
					taskTeam = matchedTeam
					task.DeliveryProject = matchedTeam.DeliveryProject
				} else if epic.DeliveryProject != "" {
					task.DeliveryProject = epic.DeliveryProject
					_, taskTeam = r.findTeamByDeliveryProject(epic.DeliveryProject)
				} else {
					task.DeliveryProject = defaultOrigin
				}
			}

			// Map Task issue type: map Story and Task according to team config, preserve custom types (e.g. Spike, Bug)
			isStory := strings.EqualFold(task.Type, "Story") || strings.EqualFold(task.Type, "User Story")
			isTaskOrEmpty := task.Type == "" || strings.EqualFold(task.Type, "Task")
			if taskTeam != nil {
				if isStory && taskTeam.IssueTypes.Story != "" {
					task.Type = taskTeam.IssueTypes.Story
				} else if isTaskOrEmpty && taskTeam.IssueTypes.Task != "" {
					task.Type = taskTeam.IssueTypes.Task
				}
			} else {
				if task.Type == "" {
					task.Type = "Task"
				}
			}
		}
	}

	snap.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *Router) findTeamByDeliveryProject(proj string) (string, *config.JiraTeamConfig) {
	if r.cfg == nil || r.cfg.Teams == nil || proj == "" {
		return "", nil
	}
	for name, t := range r.cfg.Teams {
		if strings.EqualFold(t.DeliveryProject, proj) || strings.EqualFold(name, proj) {
			teamCopy := t
			return name, &teamCopy
		}
	}
	return "", nil
}

func (r *Router) matchTeamByRules(title, description string) (string, *config.JiraTeamConfig) {
	if r.cfg == nil || len(r.cfg.Teams) == 0 {
		return "", nil
	}

	type teamScore struct {
		name  string
		team  config.JiraTeamConfig
		score int
	}

	var scores []teamScore
	for name, team := range r.cfg.Teams {
		s := scoreTeam(name, team, title, description)
		if s > 0 {
			scores = append(scores, teamScore{name: name, team: team, score: s})
		}
	}

	if len(scores) == 0 {
		return "", nil
	}

	sort.Slice(scores, func(i, j int) bool {
		if scores[i].score != scores[j].score {
			return scores[i].score > scores[j].score
		}
		return scores[i].name < scores[j].name
	})

	return scores[0].name, &scores[0].team
}

func scoreTeam(teamName string, team config.JiraTeamConfig, title, description string) int {
	score := 0
	titleLower := strings.ToLower(title)
	descLower := strings.ToLower(description)

	// Check team name
	tNameLower := strings.ToLower(teamName)
	if strings.Contains(titleLower, tNameLower) {
		score += 3
	} else if strings.Contains(descLower, tNameLower) {
		score += 1
	}

	// Check delivery project key
	if team.DeliveryProject != "" {
		dpLower := strings.ToLower(team.DeliveryProject)
		if strings.Contains(titleLower, dpLower) {
			score += 3
		} else if strings.Contains(descLower, dpLower) {
			score += 1
		}
	}

	// Check keywords
	for _, kw := range team.Keywords {
		kwTrimmed := strings.TrimSpace(strings.ToLower(kw))
		if kwTrimmed == "" {
			continue
		}
		if strings.Contains(titleLower, kwTrimmed) {
			score += 2
		} else if strings.Contains(descLower, kwTrimmed) {
			score += 1
		}
	}

	return score
}

func (r *Router) recommendViaAI(ctx context.Context, itemID, title, description string) string {
	if r.aiEngine == nil || r.cfg == nil || len(r.cfg.Teams) == 0 {
		return ""
	}

	var teamNames []string
	for name := range r.cfg.Teams {
		teamNames = append(teamNames, name)
	}
	sort.Strings(teamNames)

	var teamList []string
	for _, name := range teamNames {
		team := r.cfg.Teams[name]
		teamList = append(teamList, fmt.Sprintf("- %s (Delivery Project: %s, Keywords: %s)", name, team.DeliveryProject, strings.Join(team.Keywords, ", ")))
	}

	sysPrompt := "You are an expert technical project router. Recommend the best matching team delivery project for the given work item. Respond ONLY with a JSON object mapping the item ID to the chosen delivery project key, e.g. {\"" + itemID + "\": \"PROJECT_KEY\"}."
	userPrompt := fmt.Sprintf("Item ID: %s\nTitle: %s\nDescription: %s\n\nAvailable Teams:\n%s\n\nWhich Delivery Project should handle this item?", itemID, title, description, strings.Join(teamList, "\n"))

	res, err := r.aiEngine.Generate(ctx, ai.PromptRequest{
		SystemPrompt: sysPrompt,
		UserPrompt:   userPrompt,
		Model:        r.model,
		Temperature:  0.1,
	})
	if err != nil {
		return ""
	}

	cleaned := strings.TrimSpace(res.Content)
	if strings.HasPrefix(cleaned, "```") {
		lines := strings.Split(cleaned, "\n")
		if len(lines) >= 2 {
			cleaned = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var mapping map[string]string
	if err := json.Unmarshal([]byte(cleaned), &mapping); err == nil {
		if val, ok := mapping[itemID]; ok && strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val)
		}
	}

	for _, name := range teamNames {
		team := r.cfg.Teams[name]
		if strings.Contains(strings.ToUpper(res.Content), strings.ToUpper(team.DeliveryProject)) {
			return team.DeliveryProject
		}
	}

	return ""
}

func (r *Router) promptUserForDeliveryProject(itemType, itemID, itemTitle string) string {
	if r.in == nil || r.out == nil {
		return r.defaultOriginProject()
	}

	type teamChoice struct {
		name            string
		deliveryProject string
	}
	var choices []teamChoice
	if r.cfg != nil {
		for name, t := range r.cfg.Teams {
			choices = append(choices, teamChoice{name: name, deliveryProject: t.DeliveryProject})
		}
		sort.Slice(choices, func(i, j int) bool {
			return choices[i].name < choices[j].name
		})
	}

	origin := r.defaultOriginProject()

	fmt.Fprintf(r.out, "\nNo team delivery project could be automatically determined for %s [%s] %q\n", itemType, itemID, itemTitle)
	fmt.Fprintf(r.out, "Available Delivery Projects:\n")
	for i, c := range choices {
		fmt.Fprintf(r.out, "  [%d] %s (Team: %s)\n", i+1, c.deliveryProject, c.name)
	}
	fmt.Fprintf(r.out, "  [%d] %s (Origin Project fallback)\n", len(choices)+1, origin)
	fmt.Fprintf(r.out, "Select delivery project [1-%d, or enter project key, or Enter for default %s]: ", len(choices)+1, origin)

	reader := bufio.NewReader(r.in)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return origin
	}
	ans := strings.TrimSpace(line)
	if ans == "" {
		return origin
	}

	if idx, err := strconv.Atoi(ans); err == nil {
		if idx >= 1 && idx <= len(choices) {
			return choices[idx-1].deliveryProject
		}
		if idx == len(choices)+1 {
			return origin
		}
	}

	return strings.ToUpper(ans)
}

func (r *Router) defaultOriginProject() string {
	if r.cfg != nil && strings.TrimSpace(r.cfg.OriginProject) != "" {
		return strings.TrimSpace(r.cfg.OriginProject)
	}
	return "ORIGIN"
}
