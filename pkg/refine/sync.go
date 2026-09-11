package refine

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/arxcruz/pr-review/pkg/config"
	"github.com/arxcruz/pr-review/pkg/jira"
	"github.com/arxcruz/pr-review/pkg/session"
)

// SyncOptions configures synchronization execution.
type SyncOptions struct {
	In          io.Reader
	Out         io.Writer
	AutoConfirm bool
}

// SyncedItem tracks a created or updated issue during synchronization.
type SyncedItem struct {
	ID              string
	Key             string
	Type            string
	Title           string
	DeliveryProject string
	ParentKey       string
	DependsOn       []string
	Status          string // "Created" or "Existing"
}

// SyncResult captures the full outcome of a plan synchronization run.
type SyncResult struct {
	StrategicKey string
	EpicsCreated []SyncedItem
	TasksCreated []SyncedItem
	Aborted      bool
}

// Syncer handles validating and synchronizing a decomposition tree with Jira.
type Syncer struct {
	client jira.Client
	store  session.Store
	cfg    *config.JiraConfig
}

// NewSyncer creates a new Syncer instance.
func NewSyncer(client jira.Client, store session.Store, cfg *config.JiraConfig) *Syncer {
	return &Syncer{
		client: client,
		store:  store,
		cfg:    cfg,
	}
}

// Verify checks that the session snapshot has a valid, complete, and routed decomposition tree.
func (s *Syncer) Verify(snap *session.Snapshot) error {
	if snap == nil {
		return errors.New("session snapshot is required")
	}

	if snap.Tree == nil || len(snap.Tree.Epics) == 0 {
		return fmt.Errorf("no decomposition tree found in session snapshot for %s; refine ticket first", snap.Key)
	}

	if err := snap.Tree.Validate(); err != nil {
		return fmt.Errorf("decomposition tree validation failed: %w", err)
	}

	for _, epic := range snap.Tree.Epics {
		if strings.TrimSpace(epic.DeliveryProject) == "" {
			return fmt.Errorf("epic %q (%s) is not routed to a delivery project; run --plan or refine first", epic.ID, epic.Title)
		}
		if len(epic.Tasks) == 0 {
			return fmt.Errorf("epic %q (%s) has no delivery tasks; tree is incomplete", epic.ID, epic.Title)
		}
		for _, task := range epic.Tasks {
			if strings.TrimSpace(task.DeliveryProject) == "" {
				return fmt.Errorf("task %q (%s) is not routed to a delivery project; run --plan or refine first", task.ID, task.Title)
			}
		}
	}

	return nil
}

type taskRef struct {
	Task    *session.DecompositionTask
	EpicID  string
	EpicKey string
}

// sortTasksByDependency produces a topologically sorted list of tasks respecting task dependencies.
func sortTasksByDependency(epics []session.DecompositionEpic) ([]*taskRef, error) {
	var allTasks []*taskRef
	taskMap := make(map[string]*taskRef)

	for i := range epics {
		epic := &epics[i]
		for j := range epic.Tasks {
			t := &epic.Tasks[j]
			ref := &taskRef{
				Task:    t,
				EpicID:  epic.ID,
				EpicKey: epic.Key,
			}
			allTasks = append(allTasks, ref)
			taskMap[t.ID] = ref
		}
	}

	inDegree := make(map[string]int)
	dependents := make(map[string][]string)

	for _, ref := range allTasks {
		inDegree[ref.Task.ID] = 0
	}

	for _, ref := range allTasks {
		for _, dep := range ref.Task.DependsOn {
			if _, exists := taskMap[dep]; exists {
				inDegree[ref.Task.ID]++
				dependents[dep] = append(dependents[dep], ref.Task.ID)
			}
		}
	}

	var queue []*taskRef
	for _, ref := range allTasks {
		if inDegree[ref.Task.ID] == 0 {
			queue = append(queue, ref)
		}
	}

	var sorted []*taskRef
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		sorted = append(sorted, curr)

		for _, depID := range dependents[curr.Task.ID] {
			inDegree[depID]--
			if inDegree[depID] == 0 {
				queue = append(queue, taskMap[depID])
			}
		}
	}

	if len(sorted) != len(allTasks) {
		return nil, fmt.Errorf("dependency cycle detected among tasks in decomposition tree")
	}

	return sorted, nil
}

// Sync synchronizes the decomposed items with Jira.
func (s *Syncer) Sync(ctx context.Context, snap *session.Snapshot, opts SyncOptions) (*SyncResult, error) {
	if err := s.Verify(snap); err != nil {
		return nil, err
	}

	if s.client == nil {
		return nil, errors.New("jira client is required for sync")
	}

	in := opts.In
	if in == nil {
		in = os.Stdin
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}

	totalTasks := 0
	for _, e := range snap.Tree.Epics {
		totalTasks += len(e.Tasks)
	}

	// Explicit user confirmation prompt
	if !opts.AutoConfirm {
		fmt.Fprintf(out, "Sync execution plan for %s:\n", snap.Key)
		fmt.Fprintf(out, "  - Strategic Ticket: %s\n", snap.Key)
		fmt.Fprintf(out, "  - Epics to sync: %d\n", len(snap.Tree.Epics))
		fmt.Fprintf(out, "  - Tasks/Stories to sync: %d\n", totalTasks)
		fmt.Fprint(out, "Are you sure you want to create these issues in Jira? [y/N]: ")

		reader := bufio.NewReader(in)
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read confirmation input: %w", err)
		}

		trimmed := strings.TrimSpace(strings.ToLower(line))
		if trimmed != "y" && trimmed != "yes" {
			fmt.Fprintln(out, "Sync aborted by user.")
			return &SyncResult{StrategicKey: snap.Key, Aborted: true}, nil
		}
	}

	result := &SyncResult{
		StrategicKey: snap.Key,
	}
	localIDToRemoteKey := make(map[string]string)

	linkType := ""
	if s.cfg != nil {
		linkType = s.cfg.LinkType
	}

	// 1. Create Epics first and link to Strategic Ticket
	for i := range snap.Tree.Epics {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		epic := &snap.Tree.Epics[i]
		epicType := epic.Type
		if epicType == "" {
			epicType = "Epic"
		}

		status := "Existing"
		if epic.Key == "" {
			created, err := s.client.CreateEpic(ctx, jira.CreateEpicRequest{
				Project:     epic.DeliveryProject,
				Summary:     epic.Title,
				Description: epic.Description,
				IssueType:   epicType,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create epic %s (%s): %w", epic.ID, epic.Title, err)
			}

			epic.Key = created.Key
			status = "Created"

			// Link epic back to strategic ticket
			_, err = s.client.LinkStrategicTicket(ctx, jira.StrategicLinkRequest{
				ChildKey:  epic.Key,
				OriginKey: snap.Key,
				LinkType:  linkType,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to link epic %s (%s) to strategic ticket %s: %w", epic.ID, epic.Key, snap.Key, err)
			}

			if s.store != nil {
				if err := s.store.Save(snap); err != nil {
					return nil, fmt.Errorf("failed to save intermediate snapshot after epic %s: %w", epic.ID, err)
				}
			}
		}

		localIDToRemoteKey[epic.ID] = epic.Key
		result.EpicsCreated = append(result.EpicsCreated, SyncedItem{
			ID:              epic.ID,
			Key:             epic.Key,
			Type:            epicType,
			Title:           epic.Title,
			DeliveryProject: epic.DeliveryProject,
			ParentKey:       snap.Key,
			Status:          status,
		})
	}

	// 2. Sort child Tasks/Stories in topological dependency order
	sortedTasks, err := sortTasksByDependency(snap.Tree.Epics)
	if err != nil {
		return nil, err
	}

	// 3. Create Tasks/Stories with parent link, then dependency links
	for _, ref := range sortedTasks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		task := ref.Task
		parentEpicKey := localIDToRemoteKey[ref.EpicID]

		taskType := task.Type
		if taskType == "" {
			taskType = "Task"
		}

		status := "Existing"
		isNewTask := task.Key == ""
		if isNewTask {
			created, err := s.client.CreateTask(ctx, jira.CreateTaskRequest{
				Project:            task.DeliveryProject,
				Summary:            task.Title,
				Description:        task.Description,
				IssueType:          taskType,
				ParentKey:          parentEpicKey,
				AcceptanceCriteria: task.AcceptanceCriteria,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create task %s (%s): %w", task.ID, task.Title, err)
			}

			task.Key = created.Key
			status = "Created"

			if s.store != nil {
				if err := s.store.Save(snap); err != nil {
					return nil, fmt.Errorf("failed to save intermediate snapshot after task %s: %w", task.ID, err)
				}
			}
		}

		localIDToRemoteKey[task.ID] = task.Key

		// Establish inter-task dependency links (for newly created tasks, avoiding duplicate links on resume)
		if isNewTask {
			for _, depID := range task.DependsOn {
				// Avoid creating Blocks link to parent epic
				if depID == ref.EpicID {
					continue
				}
				depKey := localIDToRemoteKey[depID]
				if depKey != "" && depKey != task.Key {
					err := s.client.CreateDependencyLink(ctx, task.Key, depKey)
					if err != nil {
						return nil, fmt.Errorf("failed to link dependency between %s (%s) and %s (%s): %w", task.ID, task.Key, depID, depKey, err)
					}
				}
			}
		}

		result.TasksCreated = append(result.TasksCreated, SyncedItem{
			ID:              task.ID,
			Key:             task.Key,
			Type:            taskType,
			Title:           task.Title,
			DeliveryProject: task.DeliveryProject,
			ParentKey:       parentEpicKey,
			DependsOn:       task.DependsOn,
			Status:          status,
		})
	}

	// 4. Mark snapshot synced and save final state
	snap.Status = session.StatusSynced
	if s.store != nil {
		if err := s.store.Save(snap); err != nil {
			return nil, fmt.Errorf("failed to save synced snapshot: %w", err)
		}
	}

	return result, nil
}

// FormatSyncSummaryTable renders an aligned summary table of created Jira items.
func FormatSyncSummaryTable(result *SyncResult) string {
	if result == nil || result.Aborted {
		return ""
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("\n=== Synchronized Jira Issues for %s ===\n\n", result.StrategicKey))

	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTYPE\tKEY\tPROJECT\tPARENT\tSTATUS\tSUMMARY")

	idToKey := make(map[string]string)
	renderItems := func(items []SyncedItem) {
		for _, item := range items {
			idToKey[item.ID] = item.Key
			summary := strings.ReplaceAll(item.Title, "\t", " ")
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				item.ID,
				item.Type,
				item.Key,
				item.DeliveryProject,
				item.ParentKey,
				item.Status,
				strings.TrimSpace(summary),
			)
		}
	}

	renderItems(result.EpicsCreated)
	renderItems(result.TasksCreated)
	_ = w.Flush()

	// Print dependency linkages if any
	var depLines []string
	for _, item := range result.TasksCreated {
		for _, depID := range item.DependsOn {
			depKey := idToKey[depID]
			if depKey == "" {
				depKey = depID
			}
			depLines = append(depLines, fmt.Sprintf("• %s [%s] is blocked by %s [%s]", item.Key, item.ID, depKey, depID))
		}
	}

	if len(depLines) > 0 {
		buf.WriteString("\n--- Dependency Links Created ---\n")
		for _, line := range depLines {
			buf.WriteString(line + "\n")
		}
	}

	return buf.String()
}
