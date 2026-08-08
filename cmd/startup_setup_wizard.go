package cmd

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

type startupSetupStep int

const (
	startupSelectModelStep startupSetupStep = iota
	startupInputAPIKeyStep
	startupDoneStep
	startupCancelledStep
)

type startupSetupResult struct {
	ModelID  models.ModelID
	Provider models.ModelProvider
	APIKey   string
}

type startupSetupModel struct {
	step startupSetupStep

	models      []models.Model
	selectedIdx int

	selectedModel models.Model
	requiresKey   bool
	apiKeyEnvName string
	apiKeyInput   textinput.Model
	apiKey        string
	errMsg        string
}

func newStartupSetupModel() startupSetupModel {
	return startupSetupModel{
		step:          startupSelectModelStep,
		models:        getStartupWizardModels(),
		selectedIdx:   0,
		selectedModel: models.Model{},
	}
}

func (m startupSetupModel) Init() tea.Cmd {
	return nil
}

func (m startupSetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.step {
		case startupSelectModelStep:
			return m.updateSelectStep(msg)
		case startupInputAPIKeyStep:
			return m.updateAPIKeyStep(msg)
		}
	}

	if m.step == startupInputAPIKeyStep {
		var cmd tea.Cmd
		m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m startupSetupModel) updateSelectStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc", "q":
		m.step = startupCancelledStep
		return m, tea.Quit
	case "up", "k":
		if len(m.models) == 0 {
			return m, nil
		}
		if m.selectedIdx > 0 {
			m.selectedIdx--
		} else {
			m.selectedIdx = len(m.models) - 1
		}
		return m, nil
	case "down", "j":
		if len(m.models) == 0 {
			return m, nil
		}
		if m.selectedIdx < len(m.models)-1 {
			m.selectedIdx++
		} else {
			m.selectedIdx = 0
		}
		return m, nil
	case "enter":
		if len(m.models) == 0 {
			m.errMsg = "No supported models available for interactive setup"
			return m, nil
		}

		selected := m.models[m.selectedIdx]
		m.selectedModel = selected
		m.requiresKey = providerNeedsAPIKey(selected.Provider)
		m.apiKeyEnvName = providerAPIKeyName(selected.Provider)
		m.errMsg = ""

		if !m.requiresKey {
			m.step = startupDoneStep
			return m, tea.Quit
		}

		ti := textinput.New()
		ti.Placeholder = fmt.Sprintf("Input %s", m.apiKeyEnvName)
		ti.Prompt = "> "
		ti.EchoMode = textinput.EchoPassword
		ti.EchoCharacter = '*'
		ti.Focus()
		m.apiKeyInput = ti
		m.step = startupInputAPIKeyStep
		return m, textinput.Blink
	}

	return m, nil
}

func (m startupSetupModel) updateAPIKeyStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.step = startupCancelledStep
		return m, tea.Quit
	case "esc":
		m.step = startupSelectModelStep
		m.errMsg = ""
		return m, nil
	case "enter":
		input := strings.TrimSpace(m.apiKeyInput.Value())
		if input == "" {
			m.errMsg = fmt.Sprintf("%s cannot be empty", m.apiKeyEnvName)
			return m, nil
		}

		m.apiKey = input
		m.errMsg = ""
		m.step = startupDoneStep
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
	return m, cmd
}

func (m startupSetupModel) View() string {
	if m.step == startupInputAPIKeyStep {
		return m.renderAPIKeyInput()
	}

	return m.renderModelSelection()
}

func (m startupSetupModel) renderModelSelection() string {
	var b strings.Builder
	b.WriteString("OpenCode setup required: no available model/provider configured\n\n")
	b.WriteString("Select a model (up/down, enter confirm, esc cancel):\n\n")

	if len(m.models) == 0 {
		b.WriteString("  No models available\n")
		return b.String()
	}

	for i, model := range m.models {
		cursor := "  "
		if i == m.selectedIdx {
			cursor = "> "
		}
		b.WriteString(fmt.Sprintf("%s%s [%s]\n", cursor, model.Name, model.Provider))
	}

	if m.errMsg != "" {
		b.WriteString("\n")
		b.WriteString("Error: ")
		b.WriteString(m.errMsg)
		b.WriteString("\n")
	}

	return b.String()
}

func (m startupSetupModel) renderAPIKeyInput() string {
	var b strings.Builder
	b.WriteString("OpenCode setup required\n\n")
	b.WriteString(fmt.Sprintf("Selected model: %s [%s]\n", m.selectedModel.Name, m.selectedModel.Provider))
	b.WriteString(fmt.Sprintf("Input %s (enter confirm, esc back):\n\n", m.apiKeyEnvName))
	b.WriteString(m.apiKeyInput.View())
	b.WriteString("\n")

	if m.errMsg != "" {
		b.WriteString("\nError: ")
		b.WriteString(m.errMsg)
		b.WriteString("\n")
	}

	return b.String()
}

func runStartupSetupWizard() (startupSetupResult, error) {
	if !hasTTY() {
		return runStartupSetupTextMode()
	}

	m := newStartupSetupModel()
	program := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stdout),
	)

	finalModel, err := program.Run()
	if err != nil {
		if shouldFallbackToTextMode(err) {
			return runStartupSetupTextMode()
		}
		return startupSetupResult{}, err
	}

	resultModel, ok := finalModel.(startupSetupModel)
	if !ok {
		return startupSetupResult{}, fmt.Errorf("unexpected setup model type")
	}

	if resultModel.step == startupCancelledStep {
		return startupSetupResult{}, fmt.Errorf("interactive setup canceled")
	}
	if resultModel.step != startupDoneStep {
		return startupSetupResult{}, fmt.Errorf("interactive setup did not complete")
	}

	return startupSetupResult{
		ModelID:  resultModel.selectedModel.ID,
		Provider: resultModel.selectedModel.Provider,
		APIKey:   resultModel.apiKey,
	}, nil
}

func shouldFallbackToTextMode(err error) bool {
	errText := strings.ToLower(err.Error())
	return strings.Contains(errText, "could not open a new tty") || strings.Contains(errText, "/dev/tty")
}

func hasTTY() bool {
	stdinInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	stdoutInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	stdinTTY := (stdinInfo.Mode() & os.ModeCharDevice) != 0
	stdoutTTY := (stdoutInfo.Mode() & os.ModeCharDevice) != 0
	return stdinTTY && stdoutTTY
}

func runStartupSetupTextMode() (startupSetupResult, error) {
	availableModels := getStartupWizardModels()
	if len(availableModels) == 0 {
		return startupSetupResult{}, fmt.Errorf("no supported models available for startup setup")
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Fprintln(os.Stdout, "OpenCode setup required: no available model/provider configured")
	fmt.Fprintln(os.Stdout, "Select a model by number:")
	for i, model := range availableModels {
		fmt.Fprintf(os.Stdout, "  %d) %s [%s]\n", i+1, model.Name, model.Provider)
	}

	selectedModel, err := promptModelSelection(reader, availableModels)
	if err != nil {
		return startupSetupResult{}, err
	}

	result := startupSetupResult{
		ModelID:  selectedModel.ID,
		Provider: selectedModel.Provider,
	}

	if providerNeedsAPIKey(selectedModel.Provider) {
		apiKeyEnvName := providerAPIKeyName(selectedModel.Provider)
		fmt.Fprintf(os.Stdout, "Input %s: ", apiKeyEnvName)
		apiKey, readErr := reader.ReadString('\n')
		if readErr != nil {
			return startupSetupResult{}, fmt.Errorf("failed to read %s: %w", apiKeyEnvName, readErr)
		}
		result.APIKey = strings.TrimSpace(apiKey)
		if result.APIKey == "" {
			return startupSetupResult{}, fmt.Errorf("%s cannot be empty", apiKeyEnvName)
		}
	}

	return result, nil
}

func promptModelSelection(reader *bufio.Reader, availableModels []models.Model) (models.Model, error) {
	for attempt := 0; attempt < 3; attempt++ {
		fmt.Fprint(os.Stdout, "Model number: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return models.Model{}, fmt.Errorf("failed to read model selection: %w", err)
		}

		input = strings.TrimSpace(input)
		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(availableModels) {
			fmt.Fprintln(os.Stdout, "Invalid selection, please input a valid number.")
			continue
		}

		return availableModels[idx-1], nil
	}

	return models.Model{}, fmt.Errorf("too many invalid model selections")
}

func applyStartupSetupResult(result startupSetupResult) error {
	if providerNeedsAPIKey(result.Provider) {
		if err := config.UpdateProviderAPIKey(result.Provider, result.APIKey); err != nil {
			return err
		}
	}

	agents := []config.AgentName{
		config.AgentCoder,
		config.AgentSummarizer,
		config.AgentTask,
		config.AgentTitle,
	}
	for _, agentName := range agents {
		if err := config.UpdateAgentModel(agentName, result.ModelID); err != nil {
			return fmt.Errorf("failed to set model for %s: %w", agentName, err)
		}
	}

	return nil
}

func shouldLaunchStartupSetup(err error) bool {
	if err == nil {
		return false
	}

	errText := strings.ToLower(err.Error())
	triggers := []string{
		"agent coder not found",
		"agent title not found",
		"agent summarizer not found",
		"provider",
		"not enabled",
		"not supported",
		"no valid provider",
	}
	for _, trigger := range triggers {
		if strings.Contains(errText, trigger) {
			return true
		}
	}
	return false
}

func getStartupWizardModels() []models.Model {
	allowed := map[models.ModelProvider]bool{
		models.ProviderAnthropic:  true,
		models.ProviderOpenAI:     true,
		models.ProviderDeepSeek:   true,
		models.ProviderGemini:     true,
		models.ProviderGROQ:       true,
		models.ProviderOpenRouter: true,
		models.ProviderXAI:        true,
		models.ProviderCopilot:    true,
	}

	available := make([]models.Model, 0, len(models.SupportedModels))
	for _, model := range models.SupportedModels {
		if allowed[model.Provider] {
			available = append(available, model)
		}
	}

	slices.SortFunc(available, func(a, b models.Model) int {
		rankA := models.ProviderPopularity[a.Provider]
		rankB := models.ProviderPopularity[b.Provider]
		if rankA == 0 {
			rankA = 999
		}
		if rankB == 0 {
			rankB = 999
		}
		if rankA != rankB {
			return rankA - rankB
		}
		return strings.Compare(a.Name, b.Name)
	})

	return available
}

func providerNeedsAPIKey(provider models.ModelProvider) bool {
	switch provider {
	case models.ProviderAnthropic,
		models.ProviderOpenAI,
		models.ProviderDeepSeek,
		models.ProviderGemini,
		models.ProviderGROQ,
		models.ProviderOpenRouter,
		models.ProviderXAI,
		models.ProviderCopilot:
		return true
	default:
		return false
	}
}

func providerAPIKeyName(provider models.ModelProvider) string {
	switch provider {
	case models.ProviderAnthropic:
		return "ANTHROPIC_API_KEY"
	case models.ProviderOpenAI:
		return "OPENAI_API_KEY"
	case models.ProviderDeepSeek:
		return "DEEPSEEK_API_KEY"
	case models.ProviderGemini:
		return "GEMINI_API_KEY"
	case models.ProviderGROQ:
		return "GROQ_API_KEY"
	case models.ProviderOpenRouter:
		return "OPENROUTER_API_KEY"
	case models.ProviderXAI:
		return "XAI_API_KEY"
	case models.ProviderCopilot:
		return "GITHUB_TOKEN"
	default:
		return "API_KEY"
	}
}
