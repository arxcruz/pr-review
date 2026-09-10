package session

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/arxcruz/pr-review/pkg/config"
)

var (
	// ErrNotFound is returned when a session snapshot does not exist.
	ErrNotFound = errors.New("session snapshot not found")
	// ErrCorrupt is returned when a session snapshot file is empty, malformed, or has mismatched/invalid data.
	ErrCorrupt = errors.New("session snapshot file is corrupt")
	// ErrInvalidKey is returned when a session key is invalid or contains unsafe characters.
	ErrInvalidKey = errors.New("invalid session key")
)

var validKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Store defines persistence operations for session snapshots.
type Store interface {
	Save(snapshot *Snapshot) error
	Load(key string) (*Snapshot, error)
	Exists(key string) (bool, error)
	Delete(key string) error
	List() ([]string, error)
}

// FileStore implements Store using JSON files on the local filesystem.
type FileStore struct {
	dir string
}

// DefaultSessionDir returns the canonical session snapshot storage directory: ~/.config/jira-refine/sessions
func DefaultSessionDir() string {
	return config.ExpandPath("~/.config/jira-refine/sessions")
}

// NewFileStore creates a new FileStore persisting to the given directory.
// If dir is empty, DefaultSessionDir is used.
func NewFileStore(dir string) *FileStore {
	if dir == "" {
		dir = DefaultSessionDir()
	}
	return &FileStore{dir: dir}
}

// Dir returns the underlying storage directory.
func (s *FileStore) Dir() string {
	return s.dir
}

func validateKey(key string) error {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" || !validKeyPattern.MatchString(trimmed) {
		return ErrInvalidKey
	}
	return nil
}

func (s *FileStore) filePath(key string) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	return filepath.Join(s.dir, key+".json"), nil
}

// Save atomically writes the snapshot to a JSON file.
func (s *FileStore) Save(snapshot *Snapshot) error {
	if snapshot == nil {
		return ErrInvalidKey
	}
	if err := validateKey(snapshot.Key); err != nil {
		return err
	}

	targetPath, err := s.filePath(snapshot.Key)
	if err != nil {
		return err
	}

	// Make a shallow copy to prevent unexpected caller mutations
	snapCopy := *snapshot
	if snapCopy.Version == 0 {
		snapCopy.Version = 1
	}
	now := time.Now().UTC()
	if snapCopy.CreatedAt.IsZero() {
		snapCopy.CreatedAt = now
	}
	snapCopy.UpdatedAt = now

	// Update caller fields to reflect persisted state
	snapshot.Version = snapCopy.Version
	snapshot.CreatedAt = snapCopy.CreatedAt
	snapshot.UpdatedAt = snapCopy.UpdatedAt

	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	data, err := json.MarshalIndent(&snapCopy, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	tmpFile, err := os.CreateTemp(s.dir, fmt.Sprintf(".%s-*.tmp", snapCopy.Key))
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write snapshot data: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to sync snapshot file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close snapshot file: %w", err)
	}

	if err := os.Rename(tmpName, targetPath); err != nil {
		return fmt.Errorf("failed to atomically replace snapshot file: %w", err)
	}

	return nil
}

// Load reads and parses the session snapshot for the given ticket key.
func (s *FileStore) Load(key string) (*Snapshot, error) {
	path, err := s.filePath(key)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, key)
		}
		return nil, fmt.Errorf("failed to read snapshot file: %w", err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("%w: file is empty", ErrCorrupt)
	}

	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("%w: invalid json: %v", ErrCorrupt, err)
	}

	if strings.TrimSpace(snapshot.Key) == "" || snapshot.Key != key {
		return nil, fmt.Errorf("%w: key mismatch or missing in snapshot content", ErrCorrupt)
	}

	return &snapshot, nil
}

// Exists checks whether a snapshot exists for the given key.
func (s *FileStore) Exists(key string) (bool, error) {
	path, err := s.filePath(key)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Delete removes the snapshot for the given key.
func (s *FileStore) Delete(key string) error {
	path, err := s.filePath(key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// List returns all stored snapshot ticket keys.
func (s *FileStore) List() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	keys := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".json") {
			continue
		}
		key := strings.TrimSuffix(name, ".json")
		if validateKey(key) == nil {
			keys = append(keys, key)
		}
	}
	return keys, nil
}
