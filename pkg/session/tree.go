package session

import (
	"fmt"
	"strings"
)

// DecompositionTreeJSONSchema defines the standard JSON schema for DecompositionTree.
const DecompositionTreeJSONSchema = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "DecompositionTree",
  "description": "Hierarchical breakdown of a strategic ticket into epics and delivery tasks with dependency edges",
  "type": "object",
  "required": ["epics"],
  "properties": {
    "epics": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["id", "title"],
        "properties": {
          "id": {
            "type": "string",
            "description": "Unique identifier for the epic (e.g. EPIC-1)"
          },
          "key": {
            "type": "string",
            "description": "Remote Jira issue key if synchronized"
          },
          "title": {
            "type": "string",
            "description": "Clear, concise epic title"
          },
          "description": {
            "type": "string",
            "description": "Detailed epic description and business value"
          },
          "delivery_project": {
            "type": "string",
            "description": "Assigned team delivery project key"
          },
          "tasks": {
            "type": "array",
            "items": {
              "type": "object",
              "required": ["id", "title"],
              "properties": {
                "id": {
                  "type": "string",
                  "description": "Unique identifier for the task (e.g. TASK-1)"
                },
                "key": {
                  "type": "string",
                  "description": "Remote Jira issue key if synchronized"
                },
                "title": {
                  "type": "string",
                  "description": "Actionable task or story title"
                },
                "description": {
                  "type": "string",
                  "description": "Technical implementation details"
                },
                "type": {
                  "type": "string",
                  "description": "Issue type, e.g. Story, Task, Spike, Bug"
                },
                "acceptance_criteria": {
                  "type": "array",
                  "items": {
                    "type": "string"
                  },
                  "description": "List of verifiable acceptance criteria"
                },
                "delivery_project": {
                  "type": "string",
                  "description": "Target team delivery project"
                },
                "depends_on": {
                  "type": "array",
                  "items": {
                    "type": "string"
                  },
                  "description": "IDs of tasks or epics this task depends on"
                }
              }
            }
          }
        }
      }
    }
  }
}`

// Validate checks the semantic integrity of the DecompositionTree.
func (t *DecompositionTree) Validate() error {
	if t == nil {
		return fmt.Errorf("decomposition tree is nil")
	}

	if len(t.Epics) == 0 {
		return fmt.Errorf("at least one epic is required in decomposition tree")
	}

	allIDs := make(map[string]struct{})
	taskDependencies := make(map[string][]string)

	for i, epic := range t.Epics {
		epicID := strings.TrimSpace(epic.ID)
		if epicID == "" {
			return fmt.Errorf("epic at index %d: epic id is required", i)
		}
		if strings.TrimSpace(epic.Title) == "" {
			return fmt.Errorf("epic %q: epic title is required", epicID)
		}

		if _, exists := allIDs[epicID]; exists {
			return fmt.Errorf("duplicate id found: %q", epicID)
		}
		allIDs[epicID] = struct{}{}

		for j, task := range epic.Tasks {
			taskID := strings.TrimSpace(task.ID)
			if taskID == "" {
				return fmt.Errorf("epic %q, task at index %d: task id is required", epicID, j)
			}
			if strings.TrimSpace(task.Title) == "" {
				return fmt.Errorf("epic %q, task %q: task title is required", epicID, taskID)
			}

			if _, exists := allIDs[taskID]; exists {
				return fmt.Errorf("duplicate id found: %q", taskID)
			}
			allIDs[taskID] = struct{}{}

			var cleanDeps []string
			for _, dep := range task.DependsOn {
				depTrimmed := strings.TrimSpace(dep)
				if depTrimmed != "" {
					if depTrimmed == taskID {
						return fmt.Errorf("task %q cannot depend on itself", taskID)
					}
					cleanDeps = append(cleanDeps, depTrimmed)
				}
			}
			taskDependencies[taskID] = cleanDeps
		}
	}

	// Validate dependency references
	for taskID, deps := range taskDependencies {
		for _, dep := range deps {
			if _, exists := allIDs[dep]; !exists {
				return fmt.Errorf("task %q has unknown dependency %q", taskID, dep)
			}
		}
	}

	// Detect cycles in task dependency graph
	type visitState int
	const (
		unvisited visitState = iota
		visiting
		visited
	)

	nodeStates := make(map[string]visitState)
	var hasCycle func(node string) bool
	hasCycle = func(node string) bool {
		nodeStates[node] = visiting
		for _, dep := range taskDependencies[node] {
			if nodeStates[dep] == visiting {
				return true
			}
			if nodeStates[dep] == unvisited {
				if hasCycle(dep) {
					return true
				}
			}
		}
		nodeStates[node] = visited
		return false
	}

	for taskID := range taskDependencies {
		if nodeStates[taskID] == unvisited {
			if hasCycle(taskID) {
				return fmt.Errorf("dependency cycle detected involving task %q", taskID)
			}
		}
	}

	return nil
}
