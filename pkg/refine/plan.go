package refine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/arxcruz/pr-review/pkg/session"
)

// FormatPlanMarkdown produces a structured, human-readable markdown summary of the routed plan.
func FormatPlanMarkdown(snap *session.Snapshot, cfg *config.JiraConfig) string {
	if snap == nil {
		return ""
	}

	var sb strings.Builder

	title := snap.Ticket.Summary
	if title == "" {
		title = "Strategic Ticket Refinement"
	}

	sb.WriteString(fmt.Sprintf("# Refinement Delivery Plan: %s - %s\n\n", snap.Key, title))

	originProj := "N/A"
	if cfg != nil && cfg.OriginProject != "" {
		originProj = cfg.OriginProject
	}

	totalEpics := 0
	totalTasks := 0
	if snap.Tree != nil {
		totalEpics = len(snap.Tree.Epics)
		for _, e := range snap.Tree.Epics {
			totalTasks += len(e.Tasks)
		}
	}

	sb.WriteString(fmt.Sprintf("- **Strategic Ticket**: %s\n", snap.Key))
	sb.WriteString(fmt.Sprintf("- **Origin Project**: %s\n", originProj))
	sb.WriteString(fmt.Sprintf("- **Session Status**: %s\n", snap.Status))
	sb.WriteString(fmt.Sprintf("- **Total Epics**: %d\n", totalEpics))
	sb.WriteString(fmt.Sprintf("- **Total Tasks/Stories**: %d\n\n", totalTasks))

	if snap.Tree == nil || len(snap.Tree.Epics) == 0 {
		sb.WriteString("*(No decomposition tree available in session)*\n")
		return sb.String()
	}

	// Project-to-Team mapping lookup
	projToTeam := make(map[string]string)
	if cfg != nil && cfg.Teams != nil {
		for teamName, t := range cfg.Teams {
			projToTeam[t.DeliveryProject] = teamName
		}
	}

	// Calculate counts per Delivery Project
	type projectStats struct {
		project string
		team    string
		epics   int
		tasks   int
	}

	statsMap := make(map[string]*projectStats)
	for _, epic := range snap.Tree.Epics {
		epicProj := epic.DeliveryProject
		if epicProj == "" {
			epicProj = originProj
		}
		if _, ok := statsMap[epicProj]; !ok {
			statsMap[epicProj] = &projectStats{project: epicProj, team: projToTeam[epicProj]}
		}
		statsMap[epicProj].epics++

		for _, task := range epic.Tasks {
			taskProj := task.DeliveryProject
			if taskProj == "" {
				taskProj = epicProj
			}
			if _, ok := statsMap[taskProj]; !ok {
				statsMap[taskProj] = &projectStats{project: taskProj, team: projToTeam[taskProj]}
			}
			statsMap[taskProj].tasks++
		}
	}

	var statsList []*projectStats
	for _, s := range statsMap {
		statsList = append(statsList, s)
	}
	sort.Slice(statsList, func(i, j int) bool {
		return statsList[i].project < statsList[j].project
	})

	sb.WriteString("## Delivery Project Routing Summary\n\n")
	sb.WriteString("| Delivery Project | Team | Epics | Tasks/Stories |\n")
	sb.WriteString("|------------------|------|-------|---------------|\n")
	for _, s := range statsList {
		teamLabel := s.team
		if teamLabel == "" {
			if s.project == originProj {
				teamLabel = "(Origin)"
			} else {
				teamLabel = "-"
			}
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %d | %d |\n", s.project, teamLabel, s.epics, s.tasks))
	}
	sb.WriteString("\n")

	// Cross-project dependencies
	taskProjMap := make(map[string]string)
	for _, epic := range snap.Tree.Epics {
		for _, task := range epic.Tasks {
			taskProj := task.DeliveryProject
			if taskProj == "" {
				taskProj = epic.DeliveryProject
			}
			taskProjMap[task.ID] = taskProj
		}
	}

	type crossDependency struct {
		fromTask    string
		fromProject string
		toTask      string
		toProject   string
	}
	var crossDeps []crossDependency
	for _, epic := range snap.Tree.Epics {
		for _, task := range epic.Tasks {
			taskProj := task.DeliveryProject
			if taskProj == "" {
				taskProj = epic.DeliveryProject
			}
			for _, dep := range task.DependsOn {
				depProj := taskProjMap[dep]
				if depProj != "" && depProj != taskProj {
					crossDeps = append(crossDeps, crossDependency{
						fromTask:    task.ID,
						fromProject: taskProj,
						toTask:      dep,
						toProject:   depProj,
					})
				}
			}
		}
	}

	if len(crossDeps) > 0 {
		sb.WriteString("## Cross-Project Dependencies\n\n")
		for _, dep := range crossDeps {
			sb.WriteString(fmt.Sprintf("- Task `%s` [`%s`] depends on `%s` [`%s`]\n", dep.fromTask, dep.fromProject, dep.toTask, dep.toProject))
		}
		sb.WriteString("\n")
	}

	// Decomposed Items Detail
	sb.WriteString("## Decomposed Delivery Items\n\n")
	for _, epic := range snap.Tree.Epics {
		epicType := epic.Type
		if epicType == "" {
			epicType = "Epic"
		}
		teamName := projToTeam[epic.DeliveryProject]
		teamSuffix := ""
		if teamName != "" {
			teamSuffix = fmt.Sprintf(" (Team: %s)", teamName)
		}

		sb.WriteString(fmt.Sprintf("### Epic: [%s] %s\n", epic.ID, epic.Title))
		sb.WriteString(fmt.Sprintf("- **Delivery Project**: `%s`%s\n", epic.DeliveryProject, teamSuffix))
		sb.WriteString(fmt.Sprintf("- **Issue Type**: %s\n", epicType))
		if strings.TrimSpace(epic.Description) != "" {
			sb.WriteString(fmt.Sprintf("- **Description**: %s\n", strings.TrimSpace(epic.Description)))
		}
		sb.WriteString("\n")

		if len(epic.Tasks) > 0 {
			sb.WriteString("#### Tasks & Stories\n\n")
			for _, task := range epic.Tasks {
				taskType := task.Type
				if taskType == "" {
					taskType = "Task"
				}
				taskProj := task.DeliveryProject
				if taskProj == "" {
					taskProj = epic.DeliveryProject
				}
				taskTeam := projToTeam[taskProj]
				tTeamSuffix := ""
				if taskTeam != "" {
					tTeamSuffix = fmt.Sprintf(" (Team: %s)", taskTeam)
				}

				sb.WriteString(fmt.Sprintf("- **[%s] %s**\n", task.ID, task.Title))
				sb.WriteString(fmt.Sprintf("  - **Delivery Project**: `%s`%s\n", taskProj, tTeamSuffix))
				sb.WriteString(fmt.Sprintf("  - **Issue Type**: %s\n", taskType))
				if strings.TrimSpace(task.Description) != "" {
					sb.WriteString(fmt.Sprintf("  - **Description**: %s\n", strings.TrimSpace(task.Description)))
				}
				if len(task.DependsOn) > 0 {
					sb.WriteString(fmt.Sprintf("  - **Depends On**: %s\n", strings.Join(task.DependsOn, ", ")))
				} else {
					sb.WriteString("  - **Depends On**: None\n")
				}
				if len(task.AcceptanceCriteria) > 0 {
					sb.WriteString("  - **Acceptance Criteria**:\n")
					for _, ac := range task.AcceptanceCriteria {
						sb.WriteString(fmt.Sprintf("    - %s\n", ac))
					}
				}
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

// WritePlanFile writes the plan markdown content to the given file path,
// creating any intermediate directories if required.
func WritePlanFile(filePath string, markdown string) error {
	if strings.TrimSpace(filePath) == "" {
		return fmt.Errorf("plan file path is required")
	}

	dir := filepath.Dir(filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed creating directory for plan file: %w", err)
		}
	}

	if err := os.WriteFile(filePath, []byte(markdown), 0644); err != nil {
		return fmt.Errorf("failed writing plan file: %w", err)
	}

	return nil
}
