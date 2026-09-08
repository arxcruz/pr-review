package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"

	"github.com/arxcruz/pr-review/pkg/config"
)

type wizardStep int

const (
	stepProvider wizardStep = iota
	stepTargetType
	stepEndpointID
	stepConnection
	stepModels
	stepParams
	stepConfirm
	stepDone
)

type providerItem struct {
	id          string
	label       string
	desc        string
	defaultURL  string
	requiresKey bool
	models      []string
}

var wizardProviders = []providerItem{
	{
		id:          "ollama",
		label:       "Ollama",
		desc:        "Local & self-hosted open models (Llama 3, Qwen 2.5 Coder, DeepSeek)",
		defaultURL:  "http://localhost:11434",
		requiresKey: false,
		models:      []string{"qwen2.5-coder:latest", "qwen2.5-coder:32b", "deepseek-coder-v2:16b", "llama3.3:70b", "codellama:latest"},
	},
	{
		id:          "vllm",
		label:       "vLLM",
		desc:        "High-throughput GPU inference engine with OpenAI-compatible API",
		defaultURL:  "http://localhost:8000/v1",
		requiresKey: false,
		models:      []string{"Qwen/Qwen2.5-Coder-32B-Instruct", "deepseek-ai/DeepSeek-Coder-V2-Instruct", "meta-llama/Llama-3.3-70B-Instruct"},
	},
	{
		id:          "llamacpp",
		label:       "llama.cpp (llama-server)",
		desc:        "Lightweight C/C++ local inference server with OpenAI-compatible API",
		defaultURL:  "http://localhost:8080/v1",
		requiresKey: false,
		models:      []string{"default", "qwen2.5-coder-7b", "llama-3-8b"},
	},
	{
		id:          "anthropic",
		label:       "Anthropic Claude",
		desc:        "State-of-the-art coding and reasoning cloud models",
		defaultURL:  "",
		requiresKey: true,
		models:      []string{"claude-3-7-sonnet-20250219", "claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022"},
	},
	{
		id:          "gemini",
		label:       "Google Gemini",
		desc:        "Fast, large context cloud AI models with high rate limits",
		defaultURL:  "",
		requiresKey: true,
		models:      []string{"gemini-2.5-flash", "gemini-2.5-pro", "gemini-1.5-flash", "gemini-1.5-pro"},
	},
	{
		id:          "openai",
		label:       "OpenAI",
		desc:        "Official OpenAI cloud models (GPT-4o, o3-mini, etc.)",
		defaultURL:  "https://api.openai.com/v1",
		requiresKey: true,
		models:      []string{"gpt-4o", "gpt-4o-mini", "o3-mini", "o1"},
	},
	{
		id:          "custom-openai",
		label:       "Custom OpenAI-Compatible Server",
		desc:        "LM Studio, LocalAI, remote GPU clusters, OpenRouter, etc.",
		defaultURL:  "http://localhost:1234/v1",
		requiresKey: false,
		models:      []string{"default"},
	},
}

// WizardModel is the Bubble Tea model for the AI configuration wizard
type WizardModel struct {
	configPath string
	rawConfig  *config.Config
	step       wizardStep
	width      int
	height     int
	err        error

	// Step 0: Provider selection
	providerCursor int

	// Step 1: Target Type (0 = standard provider, 1 = custom endpoint)
	isCustomEndpoint bool
	targetTypeCursor int

	// Step 2: Custom Endpoint identity
	endpointIDInput   textinput.Model
	endpointNameInput textinput.Model
	endpointFocusIdx  int

	// Step 3: Connection
	baseURLInput     textinput.Model
	apiKeyInput      textinput.Model
	connectionFocus  int

	// Step 4: Models
	modelSelections  map[string]bool
	modelCursor      int
	customModelInput textinput.Model
	selectedModels   []string

	// Step 5: Parameters & Settings
	temperatureInput textinput.Model
	isDefault        bool
	paramFocusIdx    int

	// Step 6: Confirmation
	confirmCursor int

	// Final result
	savedPath string
}

// NewWizardModel initializes the wizard Bubble Tea model
func NewWizardModel(configPath string) (*WizardModel, error) {
	targetPath := configPath
	if targetPath == "" {
		targetPath = config.FindConfigFile("")
	}

	rawCfg, loadedPath, err := config.LoadRawConfig(targetPath)
	if err != nil {
		rawCfg = config.DefaultConfig()
		if targetPath == "" {
			home, _ := os.UserHomeDir()
			targetPath = filepath.Join(home, ".config", "pr-review", "config.yaml")
		}
	} else if loadedPath != "" {
		targetPath = loadedPath
	}

	// Setup text inputs
	epID := textinput.New()
	epID.Placeholder = "e.g. gpu1-vllm, remote-ollama"
	epID.CharLimit = 50
	epID.Focus()

	epName := textinput.New()
	epName.Placeholder = "e.g. GPU 1 (DeepSeek Coder)"
	epName.CharLimit = 60

	baseURL := textinput.New()
	baseURL.Placeholder = "http://localhost:11434"
	baseURL.CharLimit = 150
	baseURL.Focus()

	apiKey := textinput.New()
	apiKey.Placeholder = "e.g. ${API_KEY} or token"
	apiKey.CharLimit = 150

	customModel := textinput.New()
	customModel.Placeholder = "Type custom model name and press Enter"
	customModel.CharLimit = 100

	tempInput := textinput.New()
	tempInput.Placeholder = "0.2"
	tempInput.SetValue("0.2")
	tempInput.CharLimit = 10
	tempInput.Focus()

	m := &WizardModel{
		configPath:        targetPath,
		rawConfig:         rawCfg,
		step:              stepProvider,
		modelSelections:   make(map[string]bool),
		endpointIDInput:   epID,
		endpointNameInput: epName,
		baseURLInput:      baseURL,
		apiKeyInput:       apiKey,
		customModelInput:  customModel,
		temperatureInput:  tempInput,
		isDefault:         true,
	}

	return m, nil
}

func (m *WizardModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.step == stepDone {
				return m, tea.Quit
			}
		}

		switch m.step {
		case stepProvider:
			return m.updateProvider(msg)
		case stepTargetType:
			return m.updateTargetType(msg)
		case stepEndpointID:
			return m.updateEndpointID(msg)
		case stepConnection:
			return m.updateConnection(msg)
		case stepModels:
			return m.updateModels(msg)
		case stepParams:
			return m.updateParams(msg)
		case stepConfirm:
			return m.updateConfirm(msg)
		case stepDone:
			if msg.String() == "enter" || msg.String() == "q" || msg.String() == "esc" {
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

func (m *WizardModel) selectedProvider() providerItem {
	if m.providerCursor >= 0 && m.providerCursor < len(wizardProviders) {
		return wizardProviders[m.providerCursor]
	}
	return wizardProviders[0]
}

func (m *WizardModel) updateProvider(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.providerCursor > 0 {
			m.providerCursor--
		}
	case "down", "j":
		if m.providerCursor < len(wizardProviders)-1 {
			m.providerCursor++
		}
	case "enter":
		p := m.selectedProvider()
		m.baseURLInput.SetValue(p.defaultURL)

		// Set default key placeholder
		switch p.id {
		case "anthropic":
			m.apiKeyInput.SetValue("${ANTHROPIC_API_KEY}")
		case "gemini":
			m.apiKeyInput.SetValue("${GEMINI_API_KEY}")
		case "openai":
			m.apiKeyInput.SetValue("${OPENAI_API_KEY}")
		case "vllm":
			m.apiKeyInput.SetValue("")
			m.apiKeyInput.Placeholder = "Optional (e.g. ${VLLM_API_KEY})"
		default:
			m.apiKeyInput.SetValue("")
			m.apiKeyInput.Placeholder = "Optional API key or ${ENV_VAR}"
		}

		// Pre-select recommended models
		m.modelSelections = make(map[string]bool)
		for i, mod := range p.models {
			if i == 0 {
				m.modelSelections[mod] = true
			} else {
				m.modelSelections[mod] = false
			}
		}

		// Custom OpenAI is always an endpoint
		if p.id == "custom-openai" {
			m.isCustomEndpoint = true
			m.step = stepEndpointID
			m.endpointIDInput.Focus()
			return m, nil
		}

		m.step = stepTargetType
		m.targetTypeCursor = 0
	}
	return m, nil
}

func (m *WizardModel) updateTargetType(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k", "down", "j":
		m.targetTypeCursor = 1 - m.targetTypeCursor
	case "esc", "backspace":
		m.step = stepProvider
	case "enter":
		p := m.selectedProvider()
		if m.targetTypeCursor == 1 {
			m.isCustomEndpoint = true
			m.step = stepEndpointID
			if m.endpointIDInput.Value() == "" {
				m.endpointIDInput.SetValue(fmt.Sprintf("%s-server", p.id))
			}
			if m.endpointNameInput.Value() == "" {
				m.endpointNameInput.SetValue(fmt.Sprintf("%s (Custom Endpoint)", p.label))
			}
			m.endpointFocusIdx = 0
			m.endpointIDInput.Focus()
			m.endpointNameInput.Blur()
		} else {
			m.isCustomEndpoint = false
			m.step = stepConnection
			m.connectionFocus = 0
			m.baseURLInput.Focus()
			m.apiKeyInput.Blur()
		}
	}
	return m, nil
}

func (m *WizardModel) updateEndpointID(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.selectedProvider().id == "custom-openai" {
			m.step = stepProvider
		} else {
			m.step = stepTargetType
		}
		return m, nil

	case "tab", "shift+tab", "down", "up":
		m.endpointFocusIdx = 1 - m.endpointFocusIdx
		if m.endpointFocusIdx == 0 {
			m.endpointIDInput.Focus()
			m.endpointNameInput.Blur()
		} else {
			m.endpointNameInput.Focus()
			m.endpointIDInput.Blur()
		}
		return m, nil

	case "enter":
		if m.endpointFocusIdx == 0 {
			m.endpointFocusIdx = 1
			m.endpointNameInput.Focus()
			m.endpointIDInput.Blur()
			return m, nil
		}
		// Validate
		idVal := strings.TrimSpace(m.endpointIDInput.Value())
		if idVal == "" {
			m.endpointIDInput.SetValue(fmt.Sprintf("%s-endpoint", m.selectedProvider().id))
		}
		m.step = stepConnection
		m.connectionFocus = 0
		m.baseURLInput.Focus()
		m.apiKeyInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	if m.endpointFocusIdx == 0 {
		m.endpointIDInput, cmd = m.endpointIDInput.Update(msg)
	} else {
		m.endpointNameInput, cmd = m.endpointNameInput.Update(msg)
	}
	return m, cmd
}

func (m *WizardModel) updateConnection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.isCustomEndpoint {
			m.step = stepEndpointID
			m.endpointFocusIdx = 0
			m.endpointIDInput.Focus()
			m.endpointNameInput.Blur()
		} else {
			m.step = stepTargetType
		}
		return m, nil

	case "tab", "shift+tab", "down", "up":
		m.connectionFocus = 1 - m.connectionFocus
		if m.connectionFocus == 0 {
			m.baseURLInput.Focus()
			m.apiKeyInput.Blur()
		} else {
			m.apiKeyInput.Focus()
			m.baseURLInput.Blur()
		}
		return m, nil

	case "enter":
		if m.connectionFocus == 0 && m.selectedProvider().requiresKey {
			m.connectionFocus = 1
			m.apiKeyInput.Focus()
			m.baseURLInput.Blur()
			return m, nil
		}
		m.step = stepModels
		m.modelCursor = 0
		m.customModelInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	if m.connectionFocus == 0 {
		m.baseURLInput, cmd = m.baseURLInput.Update(msg)
	} else {
		m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
	}
	return m, cmd
}

func (m *WizardModel) updateModels(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := m.selectedProvider()
	totalItems := len(p.models) + 2 // models + custom input + proceed button

	switch msg.String() {
	case "esc":
		m.step = stepConnection
		m.connectionFocus = 0
		m.baseURLInput.Focus()
		m.apiKeyInput.Blur()
		return m, nil

	case "up", "k":
		if m.modelCursor > 0 {
			m.modelCursor--
			if m.modelCursor == len(p.models) {
				m.customModelInput.Focus()
			} else {
				m.customModelInput.Blur()
			}
		}
		return m, nil

	case "down", "j":
		if m.modelCursor < totalItems-1 {
			m.modelCursor++
			if m.modelCursor == len(p.models) {
				m.customModelInput.Focus()
			} else {
				m.customModelInput.Blur()
			}
		}
		return m, nil

	case " ":
		// Toggle checkbox if on model item
		if m.modelCursor < len(p.models) {
			mod := p.models[m.modelCursor]
			m.modelSelections[mod] = !m.modelSelections[mod]
		}
		return m, nil

	case "enter":
		// If on a recommended model, toggle it
		if m.modelCursor < len(p.models) {
			mod := p.models[m.modelCursor]
			m.modelSelections[mod] = !m.modelSelections[mod]
			return m, nil
		}

		// If on custom input, add model
		if m.modelCursor == len(p.models) {
			val := strings.TrimSpace(m.customModelInput.Value())
			if val != "" {
				m.modelSelections[val] = true
				m.customModelInput.SetValue("")
			}
			return m, nil
		}

		// If on proceed button
		m.collectSelectedModels()
		m.step = stepParams
		m.paramFocusIdx = 0
		m.temperatureInput.Focus()
		return m, nil
	}

	if m.modelCursor == len(p.models) {
		var cmd tea.Cmd
		m.customModelInput, cmd = m.customModelInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *WizardModel) collectSelectedModels() {
	var list []string
	p := m.selectedProvider()
	for _, mod := range p.models {
		if m.modelSelections[mod] {
			list = append(list, mod)
		}
	}
	for mod, selected := range m.modelSelections {
		if selected {
			found := false
			for _, item := range list {
				if item == mod {
					found = true
					break
				}
			}
			if !found {
				list = append(list, mod)
			}
		}
	}
	if len(list) == 0 {
		if len(p.models) > 0 {
			list = append(list, p.models[0])
		} else {
			list = append(list, "default")
		}
	}
	m.selectedModels = list
}

func (m *WizardModel) updateParams(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.step = stepModels
		m.modelCursor = len(m.selectedProvider().models) + 1
		m.customModelInput.Blur()
		return m, nil

	case "tab", "shift+tab", "down", "up":
		m.paramFocusIdx = 1 - m.paramFocusIdx
		if m.paramFocusIdx == 0 {
			m.temperatureInput.Focus()
		} else {
			m.temperatureInput.Blur()
		}
		return m, nil

	case " ":
		if m.paramFocusIdx == 1 {
			m.isDefault = !m.isDefault
		}
		return m, nil

	case "enter":
		if m.paramFocusIdx == 0 {
			m.paramFocusIdx = 1
			m.temperatureInput.Blur()
			return m, nil
		}
		m.collectSelectedModels()
		m.step = stepConfirm
		m.confirmCursor = 0
		return m, nil
	}

	if m.paramFocusIdx == 0 {
		var cmd tea.Cmd
		m.temperatureInput, cmd = m.temperatureInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *WizardModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "b":
		m.step = stepParams
		m.paramFocusIdx = 0
		m.temperatureInput.Focus()
		return m, nil

	case "left", "h", "right", "l", "tab":
		m.confirmCursor = 1 - m.confirmCursor
		return m, nil

	case "enter":
		if m.confirmCursor == 0 {
			// Save
			if err := m.saveConfiguration(); err != nil {
				m.err = err
			} else {
				m.step = stepDone
			}
		} else {
			// Cancel
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *WizardModel) saveConfiguration() error {
	p := m.selectedProvider()
	temp, _ := strconv.ParseFloat(m.temperatureInput.Value(), 64)
	if temp == 0 {
		temp = 0.2
	}

	baseURL := strings.TrimSpace(m.baseURLInput.Value())
	apiKey := strings.TrimSpace(m.apiKeyInput.Value())
	models := m.selectedModels
	primaryModel := ""
	if len(models) > 0 {
		primaryModel = models[0]
	}

	// Update Config
	// Update Config — always use endpoints format
	epID := ""
	epName := ""
	provType := p.id

	if m.isCustomEndpoint {
		epID = strings.TrimSpace(m.endpointIDInput.Value())
		if epID == "" {
			epID = fmt.Sprintf("%s-endpoint", p.id)
		}
		epName = strings.TrimSpace(m.endpointNameInput.Value())
		if epName == "" {
			epName = fmt.Sprintf("%s (%s)", p.label, primaryModel)
		}
		if provType == "custom-openai" {
			provType = "openai"
		}
	} else {
		epID = p.id
		epName = p.label
	}

	newEndpoint := config.AIEndpointConfig{
		ID:          epID,
		Name:        epName,
		Provider:    provType,
		BaseURL:     baseURL,
		APIKey:      apiKey,
		Model:       primaryModel,
		Models:      models,
		Temperature: temp,
	}

	// Replace or append
	replaced := false
	for i, ep := range m.rawConfig.AI.Endpoints {
		if ep.ID == epID {
			m.rawConfig.AI.Endpoints[i] = newEndpoint
			replaced = true
			break
		}
	}
	if !replaced {
		m.rawConfig.AI.Endpoints = append(m.rawConfig.AI.Endpoints, newEndpoint)
	}

	if m.isDefault {
		m.rawConfig.DefaultAIProvider = epID
	}

	// Ensure dir exists
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	data, err := yaml.Marshal(m.rawConfig)
	if err != nil {
		return fmt.Errorf("failed to serialize YAML config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config to %s: %w", m.configPath, err)
	}

	m.savedPath = m.configPath
	return nil
}

func (m *WizardModel) View() string {
	w := m.width
	if w <= 0 {
		w = 80
	}
	if w > 90 {
		w = 90
	}

	// Header
	header := wizardHeaderView(w, m.step)

	// Body content
	var body string
	switch m.step {
	case stepProvider:
		body = m.viewProvider()
	case stepTargetType:
		body = m.viewTargetType()
	case stepEndpointID:
		body = m.viewEndpointID()
	case stepConnection:
		body = m.viewConnection()
	case stepModels:
		body = m.viewModels()
	case stepParams:
		body = m.viewParams()
	case stepConfirm:
		body = m.viewConfirm()
	case stepDone:
		body = m.viewDone()
	}

	if m.err != nil {
		body += "\n\n" + statusError.Render("❌ Error: "+m.err.Error())
	}

	// Footer / Key hints
	footer := wizardFooterView(w, m.step)

	content := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(activeBorder).
		Padding(1, 2).
		Width(w).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func wizardHeaderView(width int, step wizardStep) string {
	title := titleStyle.Render(" 🔮 PR-Review AI Configuration Wizard ")

	stepNames := []string{"Provider", "Target", "Details", "Models", "Settings", "Save"}
	stepIdx := 0
	switch step {
	case stepProvider:
		stepIdx = 0
	case stepTargetType:
		stepIdx = 1
	case stepEndpointID, stepConnection:
		stepIdx = 2
	case stepModels:
		stepIdx = 3
	case stepParams:
		stepIdx = 4
	case stepConfirm, stepDone:
		stepIdx = 5
	}

	var breadcrumbs []string
	for i, name := range stepNames {
		if i == stepIdx {
			breadcrumbs = append(breadcrumbs, lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render(fmt.Sprintf("[%d. %s]", i+1, name)))
		} else if i < stepIdx {
			breadcrumbs = append(breadcrumbs, lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render(fmt.Sprintf("✓ %s", name)))
		} else {
			breadcrumbs = append(breadcrumbs, lipgloss.NewStyle().Foreground(subtleColor).Render(fmt.Sprintf("%d. %s", i+1, name)))
		}
	}

	stepsBar := strings.Join(breadcrumbs, lipgloss.NewStyle().Foreground(borderColor).Render(" ❯ "))

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		lipgloss.NewStyle().MarginTop(1).MarginBottom(1).Render(stepsBar),
		lipgloss.NewStyle().Foreground(borderColor).Render(strings.Repeat("─", width-6)),
		"",
	)
}

func wizardFooterView(width int, step wizardStep) string {
	divider := lipgloss.NewStyle().Foreground(borderColor).Render(strings.Repeat("─", width-6))

	var hints string
	switch step {
	case stepProvider, stepTargetType:
		hints = "↑/↓ or j/k: navigate  •  enter: select  •  ctrl+c / q: exit"
	case stepEndpointID, stepConnection:
		hints = "tab / shift+tab: switch input  •  enter: next step  •  esc: back"
	case stepModels:
		hints = "↑/↓: move  •  space: toggle model  •  enter on [Proceed]: next step  •  esc: back"
	case stepParams:
		hints = "tab / shift+tab: switch field  •  space: toggle default  •  enter: next step  •  esc: back"
	case stepConfirm:
		hints = "←/→ / tab: choose action  •  enter: confirm  •  esc / b: back"
	case stepDone:
		hints = "enter / q / esc: exit wizard"
	}

	hintBar := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).MarginTop(1).Render(hints)
	return lipgloss.JoinVertical(lipgloss.Left, divider, hintBar)
}

func (m *WizardModel) viewProvider() string {
	out := lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("Step 1: Select AI Engine / Provider Type") + "\n\n"

	for i, item := range wizardProviders {
		cursor := "  "
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
		descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#777777"))

		if i == m.providerCursor {
			cursor = "❯ "
			style = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(titleBg)
			descStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0")).Background(titleBg)
		}

		line := fmt.Sprintf("%s%-32s", cursor, style.Render(" "+item.label+" "))
		desc := descStyle.Render(" " + item.desc)
		out += line + desc + "\n"
	}

	return out
}

func (m *WizardModel) viewTargetType() string {
	p := m.selectedProvider()
	out := lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("Step 2: Configuration Target for "+p.label) + "\n\n"

	options := []struct {
		title string
		desc  string
	}{
		{
			title: "Standard Provider (ai." + p.id + ")",
			desc:  "Single primary instance in your config. Best for standard local or cloud API use.",
		},
		{
			title: "Custom Named Endpoint (ai.endpoints)",
			desc:  "Multiple distinct endpoints (e.g. gpu1-vllm, remote-ollama, work-gpu).",
		},
	}

	for i, opt := range options {
		cursor := "  "
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
		descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#777777"))

		if i == m.targetTypeCursor {
			cursor = "❯ "
			style = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(titleBg)
			descStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0")).Background(titleBg)
		}

		line := fmt.Sprintf("%s%s\n    %s\n", cursor, style.Render(" "+opt.title+" "), descStyle.Render(opt.desc))
		out += line + "\n"
	}

	return out
}

func (m *WizardModel) viewEndpointID() string {
	out := lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("Step 2b: Custom Endpoint Identity") + "\n\n"

	out += "Define a unique ID and friendly name for this AI endpoint:\n\n"

	lblID := "Endpoint ID (CLI alias, e.g. gpu1-vllm):"
	if m.endpointFocusIdx == 0 {
		lblID = lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("❯ " + lblID)
	} else {
		lblID = "  " + lblID
	}
	out += lblID + "\n  " + m.endpointIDInput.View() + "\n\n"

	lblName := "Display Name (friendly label in TUI):"
	if m.endpointFocusIdx == 1 {
		lblName = lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("❯ " + lblName)
	} else {
		lblName = "  " + lblName
	}
	out += lblName + "\n  " + m.endpointNameInput.View() + "\n"

	return out
}

func (m *WizardModel) viewConnection() string {
	p := m.selectedProvider()
	out := lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("Step 3: Connection & Authentication ("+p.label+")") + "\n\n"

	lblURL := "Base URL / Server Host:"
	if m.connectionFocus == 0 {
		lblURL = lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("❯ " + lblURL)
	} else {
		lblURL = "  " + lblURL
	}
	out += lblURL + "\n  " + m.baseURLInput.View() + "\n\n"

	lblKey := "API Key or Environment Variable Placeholder (e.g. ${ANTHROPIC_API_KEY}):"
	if m.connectionFocus == 1 {
		lblKey = lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("❯ " + lblKey)
	} else {
		lblKey = "  " + lblKey
	}
	out += lblKey + "\n  " + m.apiKeyInput.View() + "\n"

	return out
}

func (m *WizardModel) viewModels() string {
	p := m.selectedProvider()
	out := lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("Step 4: Select & Add Models ("+p.label+")") + "\n\n"
	out += "Check all models you want available in the TUI provider selector [a]:\n\n"

	for i, mod := range p.models {
		cursor := "  "
		checked := "[ ] "
		if m.modelSelections[mod] {
			checked = lipgloss.NewStyle().Foreground(secondaryColor).Bold(true).Render("[✓] ")
		}

		lineStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0"))
		if i == m.modelCursor {
			cursor = "❯ "
			lineStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(titleBg)
		}

		out += fmt.Sprintf("%s%s%s\n", cursor, checked, lineStyle.Render(" "+mod+" "))
	}

	// Custom model input row
	customIdx := len(p.models)
	cursor := "  "
	if m.modelCursor == customIdx {
		cursor = "❯ "
	}
	out += "\n" + cursor + lipgloss.NewStyle().Bold(true).Render("Add custom model: ") + m.customModelInput.View() + "\n"

	// Proceed button row
	proceedIdx := len(p.models) + 1
	btnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Background(lipgloss.Color("#222222")).Padding(0, 2)
	if m.modelCursor == proceedIdx {
		btnStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(primaryColor).Padding(0, 2)
	}
	out += "\n  " + btnStyle.Render("Proceed to Parameters ❯") + "\n"

	return out
}

func (m *WizardModel) viewParams() string {
	out := lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("Step 5: Parameters & Preferences") + "\n\n"

	lblTemp := "Sampling Temperature (0.0 to 1.0, recommended: 0.2):"
	if m.paramFocusIdx == 0 {
		lblTemp = lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("❯ " + lblTemp)
	} else {
		lblTemp = "  " + lblTemp
	}
	out += lblTemp + "\n  " + m.temperatureInput.View() + "\n\n"

	lblDef := "Set as Default AI Provider in config.yaml?"
	if m.paramFocusIdx == 1 {
		lblDef = lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("❯ " + lblDef)
	} else {
		lblDef = "  " + lblDef
	}

	defToggle := "[ No ]"
	if m.isDefault {
		defToggle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(secondaryColor).Render(" [✓ Yes (Default)] ")
	} else {
		defToggle = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Background(lipgloss.Color("#222222")).Render(" [ No ] ")
	}

	out += lblDef + "\n  " + defToggle + " (press Space to toggle)\n"

	return out
}

func (m *WizardModel) viewConfirm() string {
	p := m.selectedProvider()
	out := lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("Step 6: Review & Save Configuration") + "\n\n"

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("• Target File:      %s\n", lipgloss.NewStyle().Bold(true).Render(m.configPath)))
	summary.WriteString(fmt.Sprintf("• Provider Type:    %s\n", p.label))

	if m.isCustomEndpoint {
		summary.WriteString(fmt.Sprintf("• Target Mode:      Custom Endpoint (ID: %s, Name: %s)\n", m.endpointIDInput.Value(), m.endpointNameInput.Value()))
	} else {
		summary.WriteString(fmt.Sprintf("• Target Mode:      Standard Provider (ai.%s)\n", p.id))
	}

	url := m.baseURLInput.Value()
	if url == "" {
		url = "(default cloud endpoint)"
	}
	summary.WriteString(fmt.Sprintf("• Base URL:         %s\n", url))

	key := m.apiKeyInput.Value()
	if key == "" {
		key = "(none / unauthenticated)"
	} else if !strings.HasPrefix(key, "${") {
		key = maskSecret(key)
	}
	summary.WriteString(fmt.Sprintf("• API Key:          %s\n", key))

	modelsStr := strings.Join(m.selectedModels, ", ")
	if modelsStr == "" {
		modelsStr = "(none selected)"
	}
	summary.WriteString(fmt.Sprintf("• Models (%d):       %s\n", len(m.selectedModels), modelsStr))
	summary.WriteString(fmt.Sprintf("• Temperature:      %s\n", m.temperatureInput.Value()))

	defStr := "No"
	if m.isDefault {
		defStr = "Yes"
	}
	summary.WriteString(fmt.Sprintf("• Make Default:     %s\n", defStr))

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Render(summary.String())

	out += card + "\n\n"

	btnSave := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Background(lipgloss.Color("#222222")).Padding(0, 2).Render("Save & Apply")
	btnCancel := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Background(lipgloss.Color("#222222")).Padding(0, 2).Render("Cancel")

	if m.confirmCursor == 0 {
		btnSave = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(secondaryColor).Padding(0, 2).Render("💾 Save & Apply")
	} else {
		btnCancel = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(accentColor).Padding(0, 2).Render("❌ Cancel")
	}

	out += "  " + btnSave + "    " + btnCancel + "\n"

	return out
}

func (m *WizardModel) viewDone() string {
	out := statusSuccess.Render("✓ Successfully updated configuration!") + "\n\n"

	out += fmt.Sprintf("Saved changes to: %s\n\n", lipgloss.NewStyle().Bold(true).Render(m.savedPath))

	out += "You can now run pr-review using your new configuration:\n\n"
	if m.isCustomEndpoint {
		out += lipgloss.NewStyle().Foreground(primaryColor).Render(fmt.Sprintf("  pr-review --provider %s\n", m.endpointIDInput.Value()))
	} else {
		out += lipgloss.NewStyle().Foreground(primaryColor).Render(fmt.Sprintf("  pr-review --provider %s\n", m.selectedProvider().id))
	}
	out += lipgloss.NewStyle().Foreground(secondaryColor).Render("  pr-review               # launches interactive TUI dashboard\n\n")

	out += lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("Press Enter or 'q' to exit.")

	return out
}

func maskSecret(s string) string {
	if len(s) <= 8 {
		return "********"
	}
	return s[:4] + "..." + s[len(s)-4:]
}

// RunWizard launches the Bubble Tea TUI configuration wizard
func RunWizard(configPath string) error {
	model, err := NewWizardModel(configPath)
	if err != nil {
		return fmt.Errorf("failed to initialize wizard: %w", err)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run wizard TUI: %w", err)
	}
	return nil
}
