package docscan

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arxcruz/pr-review/pkg/config"
)

// Scanner scans directories for CONTEXT.md, ADRs, and architectural docs.
type Scanner struct {
	cfg Config
}

// NewScanner creates a new Scanner with the specified configuration.
func NewScanner(cfg Config) *Scanner {
	if cfg.MaxTotalBytes <= 0 {
		cfg.MaxTotalBytes = DefaultMaxTotalBytes
	}
	if cfg.MaxDocBytes <= 0 {
		cfg.MaxDocBytes = DefaultMaxDocBytes
	}
	return &Scanner{cfg: cfg}
}

func (s *Scanner) warn(result *ScanResult, msg string) {
	result.Warnings = append(result.Warnings, msg)
	if s.cfg.WarnFunc != nil {
		s.cfg.WarnFunc(msg)
	}
}

// Scan processes a list of directory paths and collects architectural docs.
func (s *Scanner) Scan(paths []string) (*ScanResult, error) {
	result := &ScanResult{
		Documents: make([]Document, 0),
		Warnings:  make([]string, 0),
	}

	for _, rawPath := range paths {
		expanded := config.ExpandPath(strings.TrimSpace(rawPath))
		if expanded == "" {
			continue
		}

		fi, err := os.Stat(expanded)
		if err != nil {
			s.warn(result, fmt.Sprintf("documentation path does not exist or cannot be accessed: %s (%v)", expanded, err))
			continue
		}
		if !fi.IsDir() {
			s.warn(result, fmt.Sprintf("documentation path is not a directory: %s", expanded))
			continue
		}

		s.scanRepoDir(expanded, result)
	}

	return result, nil
}

func (s *Scanner) probeAndAdd(repoDir, relPath string, result *ScanResult) bool {
	target := filepath.Join(repoDir, relPath)
	fi, err := os.Stat(target)
	if err != nil {
		if !os.IsNotExist(err) {
			s.warn(result, fmt.Sprintf("failed to access %s: %v", target, err))
		}
		return false
	}
	if fi.IsDir() {
		return false
	}

	doc, err := s.readDocument(repoDir, relPath)
	if err != nil {
		s.warn(result, fmt.Sprintf("failed to read %s: %v", target, err))
		return false
	}

	result.Documents = append(result.Documents, *doc)
	return true
}

func (s *Scanner) scanRepoDir(repoDir string, result *ScanResult) {
	// 1. Root CONTEXT-MAP.md (for multi-context repos)
	s.probeAndAdd(repoDir, "CONTEXT-MAP.md", result)

	// 2. Root CONTEXT.md
	s.probeAndAdd(repoDir, "CONTEXT.md", result)

	// 3. Root architectural docs: docs/*.md (excluding subdirectories)
	s.scanDocsDir(repoDir, "docs", result)

	// 4. docs/adr/*.md
	s.scanADRDir(repoDir, filepath.Join("docs", "adr"), result)

	// 5. Sub-contexts under src/ (e.g. src/<context>/CONTEXT.md, src/<context>/docs/adr/*.md)
	srcDir := filepath.Join(repoDir, "src")
	if fi, err := os.Stat(srcDir); err == nil && fi.IsDir() {
		entries, err := os.ReadDir(srcDir)
		if err != nil {
			s.warn(result, fmt.Sprintf("failed to read src directory %s: %v", srcDir, err))
		} else {
			var subDirs []string
			for _, entry := range entries {
				if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
					subDirs = append(subDirs, entry.Name())
				}
			}
			sort.Strings(subDirs)

			for _, sub := range subDirs {
				s.probeAndAdd(repoDir, filepath.Join("src", sub, "CONTEXT.md"), result)
				s.scanADRDir(repoDir, filepath.Join("src", sub, "docs", "adr"), result)
			}
		}
	} else if err != nil && !os.IsNotExist(err) {
		s.warn(result, fmt.Sprintf("failed to access src directory %s: %v", srcDir, err))
	}
}

func (s *Scanner) scanDocsDir(repoDir, relDir string, result *ScanResult) {
	dir := filepath.Join(repoDir, relDir)
	fi, err := os.Stat(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			s.warn(result, fmt.Sprintf("failed to access directory %s: %v", dir, err))
		}
		return
	}
	if !fi.IsDir() {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		s.warn(result, fmt.Sprintf("failed to read directory %s: %v", dir, err))
		return
	}

	var mdFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".md") {
			mdFiles = append(mdFiles, name)
		}
	}
	sort.Strings(mdFiles)

	for _, name := range mdFiles {
		relPath := filepath.Join(relDir, name)
		doc, err := s.readDocument(repoDir, relPath)
		if err != nil {
			s.warn(result, fmt.Sprintf("failed to read %s: %v", filepath.Join(dir, name), err))
			continue
		}
		result.Documents = append(result.Documents, *doc)
	}
}

func (s *Scanner) scanADRDir(repoDir, relDir string, result *ScanResult) {
	s.scanDocsDir(repoDir, relDir, result)
}

func (s *Scanner) readDocument(repoDir, relPath string) (*Document, error) {
	fullPath := filepath.Join(repoDir, relPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	content := string(data)
	title := extractTitle(content, filepath.Base(relPath))

	return &Document{
		RepoPath:     repoDir,
		RelativePath: relPath,
		Title:        title,
		Content:      content,
	}, nil
}

// extractTitle finds the first level-1 markdown header (# Header) or falls back to filename.
func extractTitle(content, fallback string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# ") {
			title := strings.TrimSpace(strings.TrimPrefix(line, "#"))
			if title != "" {
				return title
			}
		}
	}
	return fallback
}
