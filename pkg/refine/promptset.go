package refine

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed prompts/jira-refine.md
var defaultPromptSetMD string

// PromptSet holds the raw prompt templates and shared guidelines loaded from
// a jira-refine.md file. The two prompt templates may reference
// {{.Guidelines}}, which is filled in with Guidelines at render time.
type PromptSet struct {
	Frontier      string
	Decomposition string
	Guidelines    string
}

// DefaultPromptSet returns the prompt set embedded in the binary.
func DefaultPromptSet() (*PromptSet, error) {
	return parsePromptSet(defaultPromptSetMD)
}

// LoadPromptSet resolves the PromptSet to use. If explicitPath is non-empty
// it is read directly (a missing or unreadable file is an error). Otherwise
// the file at the user's config directory (~/.config/jira-refine/jira-refine.md)
// is used; if it doesn't exist yet, the embedded default is written there and
// a message describing that is printed to msgOut.
func LoadPromptSet(explicitPath string, msgOut io.Writer) (*PromptSet, error) {
	if explicitPath != "" {
		data, err := os.ReadFile(explicitPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read prompt file %s: %w", explicitPath, err)
		}
		return parsePromptSet(string(data))
	}

	configPath, err := defaultPromptFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if errors.Is(err, os.ErrNotExist) {
		if mkErr := os.MkdirAll(filepath.Dir(configPath), 0o755); mkErr != nil {
			return nil, fmt.Errorf("failed to create config directory for %s: %w", configPath, mkErr)
		}
		if writeErr := os.WriteFile(configPath, []byte(defaultPromptSetMD), 0o644); writeErr != nil {
			return nil, fmt.Errorf("failed to write default prompt file to %s: %w", configPath, writeErr)
		}
		if msgOut != nil {
			fmt.Fprintf(msgOut, "no custom prompt found — using built-in default, writing it to %s for future editing\n", configPath)
		}
		return parsePromptSet(defaultPromptSetMD)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read prompt file %s: %w", configPath, err)
	}

	return parsePromptSet(string(data))
}

func defaultPromptFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "jira-refine", "jira-refine.md"), nil
}

// parsePromptSet splits a jira-refine.md document into its named "## " sections.
func parsePromptSet(md string) (*PromptSet, error) {
	sections := make(map[string]string)
	var current string
	var buf strings.Builder

	flush := func() {
		if current != "" {
			sections[current] = strings.TrimSpace(buf.String())
		}
		buf.Reset()
	}

	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "## ") {
			flush()
			current = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			continue
		}
		if current != "" {
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}
	flush()

	ps := &PromptSet{
		Frontier:      sections["Frontier Prompt"],
		Decomposition: sections["Decomposition Prompt"],
		Guidelines:    sections["Guidelines"],
	}
	if ps.Frontier == "" {
		return nil, fmt.Errorf("prompt file is missing the required '## Frontier Prompt' section")
	}
	if ps.Decomposition == "" {
		return nil, fmt.Errorf("prompt file is missing the required '## Decomposition Prompt' section")
	}
	return ps, nil
}

type promptTemplateData struct {
	Guidelines string
}

func renderPromptTemplate(name, tmplStr, guidelines string) (string, error) {
	tmpl, err := template.New(name).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse %s section of prompt file: %w", name, err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, promptTemplateData{Guidelines: guidelines}); err != nil {
		return "", fmt.Errorf("failed to render %s section of prompt file: %w", name, err)
	}
	return buf.String(), nil
}
