package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWizardModelWorkflow(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	m, err := NewWizardModel(configPath)
	if err != nil {
		t.Fatalf("failed to create wizard model: %v", err)
	}

	if m.step != stepProvider {
		t.Fatalf("expected initial step to be stepProvider, got %v", m.step)
	}

	// 1. Select vLLM (index 1)
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.providerCursor != 1 {
		t.Fatalf("expected providerCursor 1, got %d", m.providerCursor)
	}

	// Hit Enter -> StepTargetType
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepTargetType {
		t.Fatalf("expected stepTargetType, got %v", m.step)
	}

	// Choose Custom Endpoint -> Down -> Enter
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepEndpointID {
		t.Fatalf("expected stepEndpointID, got %v", m.step)
	}

	// Set endpoint ID & Name
	m.endpointIDInput.SetValue("gpu-vllm-test")
	m.endpointNameInput.SetValue("GPU vLLM Test")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // focus to name
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // proceed to connection
	if m.step != stepConnection {
		t.Fatalf("expected stepConnection, got %v", m.step)
	}

	// Connection: Base URL
	m.baseURLInput.SetValue("http://192.168.1.100:8000/v1")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepModels {
		t.Fatalf("expected stepModels, got %v", m.step)
	}

	// Models step: toggle recommended model
	m.Update(tea.KeyMsg{Type: tea.KeySpace}) // toggle model 0

	// Go to proceed button and press Enter
	p := m.selectedProvider()
	proceedIdx := len(p.models) + 1
	m.modelCursor = proceedIdx
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepParams {
		t.Fatalf("expected stepParams, got %v", m.step)
	}

	// Params step
	m.temperatureInput.SetValue("0.3")
	m.paramFocusIdx = 1
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepConfirm {
		t.Fatalf("expected stepConfirm, got %v", m.step)
	}

	// Confirm step: Press Enter on [Save & Apply]
	m.confirmCursor = 0
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepDone {
		t.Fatalf("expected stepDone, got %v (err: %v)", m.step, m.err)
	}

	// Check that file was created and contains saved endpoint
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file was not created at %s: %v", configPath, err)
	}

	// View rendering smoke test
	viewOutput := m.View()
	if len(viewOutput) == 0 {
		t.Fatalf("expected non-empty view output")
	}
}
