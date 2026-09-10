package session_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/arxcruz/pr-review/pkg/jira"
	"github.com/arxcruz/pr-review/pkg/session"
)

func TestFileStore_SaveAndLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := session.NewFileStore(dir)

	answeredAt := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	snapshot := &session.Snapshot{
		Key:    "STRAT-100",
		Status: "in-progress",
		Ticket: jira.Ticket{
			Key:         "STRAT-100",
			Summary:     "Core Platform Modularization",
			Description: "Decompose monolithic core into services",
			Status:      "To Do",
			IssueType:   "Epic",
			Comments: []jira.Comment{
				{
					ID:      "c1",
					Author:  "alice",
					Body:    "Initial requirements discussed.",
					Created: answeredAt,
				},
			},
			IssueLinks: []jira.IssueLink{
				{
					Relationship: "blocks",
					Key:          "DELIV-10",
					Summary:      "Child task",
					Status:       "Open",
				},
			},
		},
		Rounds: []session.Round{
			{
				Number:     1,
				AnsweredAt: &answeredAt,
				Questions: []session.Question{
					{
						ID:             "q1",
						Title:          "Which database to use?",
						Explanation:    "Need persistence choice",
						Options:        []string{"PostgreSQL", "SQLite"},
						Recommendation: "PostgreSQL",
						Answer:         "PostgreSQL",
					},
				},
			},
		},
		CurrentFrontier: []session.Question{
			{
				ID:             "q2",
				Title:          "Queue service?",
				Explanation:    "Need message queue choice",
				Options:        []string{"Kafka", "RabbitMQ"},
				Recommendation: "Kafka",
			},
		},
	}

	if err := store.Save(snapshot); err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	loaded, err := store.Load("STRAT-100")
	if err != nil {
		t.Fatalf("failed to load snapshot: %v", err)
	}

	if loaded.Key != snapshot.Key {
		t.Errorf("expected Key %q, got %q", snapshot.Key, loaded.Key)
	}
	if loaded.Status != "in-progress" {
		t.Errorf("expected Status %q, got %q", "in-progress", loaded.Status)
	}
	if loaded.Ticket.Key != "STRAT-100" || loaded.Ticket.Summary != "Core Platform Modularization" {
		t.Errorf("ticket details mismatch: %+v", loaded.Ticket)
	}
	if len(loaded.Ticket.Comments) != 1 || loaded.Ticket.Comments[0].Body != "Initial requirements discussed." {
		t.Errorf("ticket comments mismatch: %+v", loaded.Ticket.Comments)
	}
	if len(loaded.Rounds) != 1 || len(loaded.Rounds[0].Questions) != 1 {
		t.Fatalf("expected 1 round with 1 question, got %+v", loaded.Rounds)
	}
	if loaded.Rounds[0].Questions[0].Answer != "PostgreSQL" {
		t.Errorf("expected answered round question answer %q, got %q", "PostgreSQL", loaded.Rounds[0].Questions[0].Answer)
	}
	if len(loaded.CurrentFrontier) != 1 {
		t.Fatalf("expected 1 current frontier question, got %+v", loaded.CurrentFrontier)
	}
	if loaded.CurrentFrontier[0].ID != "q2" || loaded.CurrentFrontier[0].Recommendation != "Kafka" {
		t.Errorf("current frontier mismatch: %+v", loaded.CurrentFrontier[0])
	}
	if loaded.CreatedAt.IsZero() || loaded.UpdatedAt.IsZero() {
		t.Errorf("expected timestamps to be set, got CreatedAt=%v, UpdatedAt=%v", loaded.CreatedAt, loaded.UpdatedAt)
	}
}

func TestFileStore_DirectoryCreation(t *testing.T) {
	baseDir := t.TempDir()
	nestedDir := filepath.Join(baseDir, "nested", "level", "sessions")

	store := session.NewFileStore(nestedDir)

	snapshot := &session.Snapshot{
		Key:    "NESTED-1",
		Status: "new",
	}

	if err := store.Save(snapshot); err != nil {
		t.Fatalf("expected Save to automatically create directories, got: %v", err)
	}

	loaded, err := store.Load("NESTED-1")
	if err != nil {
		t.Fatalf("failed to load from newly created directory: %v", err)
	}
	if loaded.Key != "NESTED-1" {
		t.Errorf("expected Key NESTED-1, got %s", loaded.Key)
	}
}

func TestFileStore_AtomicWrite_CleansUpTemp(t *testing.T) {
	dir := t.TempDir()
	store := session.NewFileStore(dir)

	snapshot := &session.Snapshot{
		Key:    "ATOMIC-1",
		Status: "ready",
	}

	if err := store.Save(snapshot); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 file in dir, found %d", len(entries))
	}
	if entries[0].Name() != "ATOMIC-1.json" {
		t.Errorf("expected ATOMIC-1.json, got %s", entries[0].Name())
	}
}

func TestFileStore_CorruptFileDetection(t *testing.T) {
	dir := t.TempDir()
	store := session.NewFileStore(dir)

	// 1. Empty file
	emptyFile := filepath.Join(dir, "EMPTY-1.json")
	if err := os.WriteFile(emptyFile, []byte("   \n"), 0644); err != nil {
		t.Fatalf("failed to write empty file: %v", err)
	}

	_, err := store.Load("EMPTY-1")
	if err == nil || !errors.Is(err, session.ErrCorrupt) {
		t.Errorf("expected ErrCorrupt on empty file, got: %v", err)
	}

	// 2. Malformed JSON
	badJSONFile := filepath.Join(dir, "BAD-1.json")
	if err := os.WriteFile(badJSONFile, []byte("{not valid json"), 0644); err != nil {
		t.Fatalf("failed to write bad json: %v", err)
	}

	_, err = store.Load("BAD-1")
	if err == nil || !errors.Is(err, session.ErrCorrupt) {
		t.Errorf("expected ErrCorrupt on malformed json, got: %v", err)
	}

	// 3. Missing key
	missingKeyFile := filepath.Join(dir, "NOKEY-1.json")
	if err := os.WriteFile(missingKeyFile, []byte(`{"status":"in-progress"}`), 0644); err != nil {
		t.Fatalf("failed to write missing key json: %v", err)
	}

	_, err = store.Load("NOKEY-1")
	if err == nil || !errors.Is(err, session.ErrCorrupt) {
		t.Errorf("expected ErrCorrupt on missing key, got: %v", err)
	}

	// 4. Mismatched key
	mismatchedKeyFile := filepath.Join(dir, "MISMATCH-1.json")
	if err := os.WriteFile(mismatchedKeyFile, []byte(`{"key":"OTHER-2","status":"in-progress"}`), 0644); err != nil {
		t.Fatalf("failed to write mismatched key json: %v", err)
	}

	_, err = store.Load("MISMATCH-1")
	if err == nil || !errors.Is(err, session.ErrCorrupt) {
		t.Errorf("expected ErrCorrupt on mismatched key, got: %v", err)
	}
}

func TestFileStore_NotFoundAndExists(t *testing.T) {
	dir := t.TempDir()
	store := session.NewFileStore(dir)

	exists, err := store.Exists("NONEXISTENT-1")
	if err != nil {
		t.Fatalf("unexpected error on Exists: %v", err)
	}
	if exists {
		t.Errorf("expected Exists to return false")
	}

	_, err = store.Load("NONEXISTENT-1")
	if err == nil || !errors.Is(err, session.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestFileStore_List_And_Delete(t *testing.T) {
	dir := t.TempDir()
	store := session.NewFileStore(dir)

	// Non-existent directory handling in List
	emptyStore := session.NewFileStore(filepath.Join(dir, "does-not-exist"))
	keys, err := emptyStore.List()
	if err != nil || len(keys) != 0 {
		t.Errorf("expected empty list for non-existent dir, got %v, err %v", keys, err)
	}

	// Save two snapshots
	_ = store.Save(&session.Snapshot{Key: "PROJ-1"})
	_ = store.Save(&session.Snapshot{Key: "PROJ-2"})

	// Create random non-json file and hidden file
	_ = os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("hi"), 0644)
	_ = os.WriteFile(filepath.Join(dir, ".PROJ-1.tmp"), []byte("tmp"), 0644)

	list, err := store.List()
	if err != nil {
		t.Fatalf("failed to list keys: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 keys, got %d (%v)", len(list), list)
	}

	// Delete
	if err := store.Delete("PROJ-1"); err != nil {
		t.Fatalf("failed to delete PROJ-1: %v", err)
	}

	exists, err := store.Exists("PROJ-1")
	if err != nil || exists {
		t.Errorf("expected PROJ-1 to not exist after delete")
	}

	// Deleting again should not fail
	if err := store.Delete("PROJ-1"); err != nil {
		t.Errorf("expected delete of non-existent key to succeed, got: %v", err)
	}
}

func TestFileStore_InvalidKeys(t *testing.T) {
	dir := t.TempDir()
	store := session.NewFileStore(dir)

	invalidKeys := []string{"", "   ", "../escape", "key/with/slash", "key\\backslash", "bad key"}

	for _, k := range invalidKeys {
		if err := store.Save(&session.Snapshot{Key: k}); !errors.Is(err, session.ErrInvalidKey) {
			t.Errorf("Save(%q): expected ErrInvalidKey, got: %v", k, err)
		}
		if _, err := store.Load(k); !errors.Is(err, session.ErrInvalidKey) {
			t.Errorf("Load(%q): expected ErrInvalidKey, got: %v", k, err)
		}
		if _, err := store.Exists(k); !errors.Is(err, session.ErrInvalidKey) {
			t.Errorf("Exists(%q): expected ErrInvalidKey, got: %v", k, err)
		}
		if err := store.Delete(k); !errors.Is(err, session.ErrInvalidKey) {
			t.Errorf("Delete(%q): expected ErrInvalidKey, got: %v", k, err)
		}
	}

	if err := store.Save(nil); !errors.Is(err, session.ErrInvalidKey) {
		t.Errorf("expected ErrInvalidKey on nil snapshot, got: %v", err)
	}
}

func TestFileStore_DecompositionTree_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := session.NewFileStore(dir)

	snapshot := &session.Snapshot{
		Key:    "TREE-1",
		Status: "decomposed",
		Tree: &session.DecompositionTree{
			Epics: []session.DecompositionEpic{
				{
					ID:              "epic-1",
					Key:             "DELIV-101",
					Title:           "Authentication Redesign",
					Description:     "Migrate auth to OAuth2",
					DeliveryProject: "AUTH",
					Tasks: []session.DecompositionTask{
						{
							ID:                 "task-1",
							Key:                "DELIV-102",
							Title:              "Implement JWT validation",
							Description:        "Add middleware for JWT tokens",
							AcceptanceCriteria: []string{"Validates exp", "Checks issuer"},
							DeliveryProject:    "AUTH",
							DependsOn:          []string{},
						},
					},
				},
			},
		},
	}

	if err := store.Save(snapshot); err != nil {
		t.Fatalf("failed to save snapshot with tree: %v", err)
	}

	loaded, err := store.Load("TREE-1")
	if err != nil {
		t.Fatalf("failed to load snapshot with tree: %v", err)
	}

	if loaded.Tree == nil || len(loaded.Tree.Epics) != 1 {
		t.Fatalf("expected 1 epic in tree, got %+v", loaded.Tree)
	}
	epic := loaded.Tree.Epics[0]
	if epic.Title != "Authentication Redesign" || len(epic.Tasks) != 1 {
		t.Errorf("epic mismatch: %+v", epic)
	}
	task := epic.Tasks[0]
	if task.Title != "Implement JWT validation" || len(task.AcceptanceCriteria) != 2 {
		t.Errorf("task mismatch: %+v", task)
	}
}

func TestFileStore_DefaultDirectory(t *testing.T) {
	dir := session.DefaultSessionDir()
	if dir == "" {
		t.Errorf("expected non-empty default session dir")
	}

	store := session.NewFileStore("")
	if store.Dir() != dir {
		t.Errorf("expected store dir %s, got %s", dir, store.Dir())
	}
}
