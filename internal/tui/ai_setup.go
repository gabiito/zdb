package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AISetupSubmitMsg is emitted when the AI setup wizard saves a config.
// The App is responsible for: writing the profile to the user's
// config.toml (replacing any existing entry of the same Name), persisting
// the API key to the OS keyring (when supplied), re-initializing the
// provider with the active profile, and proceeding to the Ask Panel.
type AISetupSubmitMsg struct {
	Name           string
	BaseURL        string
	Model          string
	APIKey         string // raw value; empty when the preset doesn't need a key (Ollama trust mode)
	TimeoutSeconds int
	IsEdit         bool // true when editing an existing profile (Name is locked)
}

// AISetupCancelMsg is emitted when the user dismisses the wizard.
type AISetupCancelMsg struct{}

// AISetupModel is the first-run wizard for the AI provider configuration.
// It walks the user through preset selection (OpenAI / Ollama / Groq /
// Custom) and edits to base URL / model / API key, then hands the values
// off to the App for persistence.
type AISetupModel struct {
	presetIdx     int
	presets       []aiPreset
	nameInput     textinput.Model
	baseURLInput  textinput.Model
	modelSelector ModelSelectorModel
	apiKeyInput   textinput.Model
	focused       int // 0=preset 1=name 2=baseURL 3=model 4=apiKey
	width, height int
	errMsg        string
	isEdit        bool // true when editing an existing profile (name is locked)
}

type aiPreset struct {
	Name     string
	BaseURL  string
	Model    string   // default model (first entry of Models)
	Models   []string // common models for this provider, surfaced in the cycler
	NeedsKey bool
}

var aiPresets = []aiPreset{
	{
		Name:    "OpenAI",
		BaseURL: "https://api.openai.com/v1",
		Model:   "gpt-4o-mini",
		Models: []string{
			"gpt-4o-mini", "gpt-4o", "gpt-4-turbo", "gpt-3.5-turbo",
		},
		NeedsKey: true,
	},
	{
		Name:    "Gemini",
		BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai",
		Model:   "gemini-2.5-flash",
		Models: []string{
			"gemini-2.5-flash", "gemini-2.5-pro", "gemini-2.5-flash-lite",
			"gemini-2.0-flash", "gemini-1.5-flash", "gemini-1.5-pro",
		},
		NeedsKey: true,
	},
	{
		Name:    "Ollama",
		BaseURL: "http://localhost:11434/v1",
		Model:   "llama3",
		Models: []string{
			"llama3", "llama3.1", "llama3.2", "mistral",
		},
		NeedsKey: false,
	},
	{
		Name:    "Groq",
		BaseURL: "https://api.groq.com/openai/v1",
		Model:   "llama3-8b-8192",
		Models: []string{
			"llama3-8b-8192", "llama3-70b-8192", "mixtral-8x7b-32768",
		},
		NeedsKey: true,
	},
	{
		Name:     "Custom",
		BaseURL:  "",
		Model:    "",
		Models:   nil, // forces the "Other…" path so the user types the model
		NeedsKey: true,
	},
}

// NewAISetupModel builds a fresh wizard for adding a new AI profile.
// Defaults to the OpenAI preset; the user can cycle presets with ←/→
// while focused on the preset row.
func NewAISetupModel(width, height int) AISetupModel {
	preset := aiPresets[0]

	name := textinput.New()
	name.CharLimit = 40
	name.Width = 50
	name.Placeholder = "default"
	name.SetValue(strings.ToLower(preset.Name))

	base := textinput.New()
	base.CharLimit = 256
	base.Width = 50
	base.SetValue(preset.BaseURL)

	model := NewModelSelector(preset.Models, preset.Model)

	key := textinput.New()
	key.CharLimit = 256
	key.Width = 50
	key.EchoMode = textinput.EchoPassword
	key.EchoCharacter = '•'
	key.Placeholder = "(stored in OS keyring)"

	return AISetupModel{
		presetIdx:     0,
		presets:       aiPresets,
		nameInput:     name,
		baseURLInput:  base,
		modelSelector: model,
		apiKeyInput:   key,
		focused:       0,
		width:         width,
		height:        height,
	}
}

// NewAISetupModelEdit builds a wizard pre-filled to edit an existing
// profile. The Name field is locked (renaming would require keyring
// migration, which we don't support yet); other fields are editable.
func NewAISetupModelEdit(name, baseURL, model string, width, height int) AISetupModel {
	m := NewAISetupModel(width, height)
	m.isEdit = true
	m.nameInput.SetValue(name)
	m.baseURLInput.SetValue(baseURL)
	m.apiKeyInput.Placeholder = "leave empty to keep current key"
	// Try to match the preset by URL so the model selector shows the
	// right list. Falls back to "Custom" when the URL is unfamiliar.
	for i, p := range m.presets {
		if p.BaseURL == baseURL {
			m.presetIdx = i
			break
		}
	}
	if m.presetIdx == 0 && baseURL != m.presets[0].BaseURL {
		for i, p := range m.presets {
			if p.Name == "Custom" {
				m.presetIdx = i
				break
			}
		}
	}
	// Reseed the model selector against the matched preset's option list
	// so a known model is highlighted and an unknown one shows up under
	// "Other…" with the value pre-filled.
	m.modelSelector.SetOptions(m.presets[m.presetIdx].Models, model)
	m.focused = 2 // start on Base URL — the most common field to tweak
	m.refocus()
	return m
}

// SetError displays an error message in the wizard (typically a save
// failure surfaced by the App).
func (m *AISetupModel) SetError(s string) { m.errMsg = s }

// Init satisfies tea.Model.
func (m AISetupModel) Init() tea.Cmd { return textinput.Blink }

// Update handles preset cycling, field navigation, submit, and cancel.
func (m AISetupModel) Update(msg tea.Msg) (AISetupModel, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			return m, func() tea.Msg { return AISetupCancelMsg{} }
		case "tab":
			m.focused = (m.focused + 1) % 5
			// In edit mode, skip the locked name field (idx 1).
			if m.isEdit && m.focused == 1 {
				m.focused = 2
			}
			m.refocus()
			return m, nil
		case "shift+tab":
			m.focused = (m.focused + 4) % 5
			if m.isEdit && m.focused == 1 {
				m.focused = 0
			}
			m.refocus()
			return m, nil
		case "left":
			if m.focused == 0 {
				m.presetIdx = (m.presetIdx - 1 + len(m.presets)) % len(m.presets)
				m.applyPreset()
				return m, nil
			}
		case "right":
			if m.focused == 0 {
				m.presetIdx = (m.presetIdx + 1) % len(m.presets)
				m.applyPreset()
				return m, nil
			}
		case "enter":
			return m.submit()
		}
	}

	var cmd tea.Cmd
	switch m.focused {
	case 1:
		if !m.isEdit {
			m.nameInput, cmd = m.nameInput.Update(msg)
		}
	case 2:
		m.baseURLInput, cmd = m.baseURLInput.Update(msg)
	case 3:
		m.modelSelector, cmd = m.modelSelector.Update(msg)
	case 4:
		m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
	}
	return m, cmd
}

// applyPreset rewrites the base URL textinput, reseeds the model
// selector with the preset's model list, and updates the default
// profile name (when not in edit mode). Custom is the one preset that
// keeps the user's URL/model as-is so cycling through doesn't blow
// away their typed values.
func (m *AISetupModel) applyPreset() {
	p := m.presets[m.presetIdx]
	if p.Name == "Custom" {
		// For the Custom preset, force the model selector into "Other…"
		// so the user types it manually.
		m.modelSelector.SetOptions(nil, m.modelSelector.Value())
		return
	}
	m.baseURLInput.SetValue(p.BaseURL)
	m.modelSelector.SetOptions(p.Models, p.Model)
	if !m.isEdit {
		m.nameInput.SetValue(strings.ToLower(p.Name))
	}
}

func (m *AISetupModel) refocus() {
	m.nameInput.Blur()
	m.baseURLInput.Blur()
	m.modelSelector.Blur()
	m.apiKeyInput.Blur()
	switch m.focused {
	case 1:
		if !m.isEdit {
			m.nameInput.Focus()
		}
	case 2:
		m.baseURLInput.Focus()
	case 3:
		m.modelSelector.Focus()
	case 4:
		m.apiKeyInput.Focus()
	}
}

func (m AISetupModel) submit() (AISetupModel, tea.Cmd) {
	name := strings.TrimSpace(m.nameInput.Value())
	baseURL := strings.TrimSpace(m.baseURLInput.Value())
	model := strings.TrimSpace(m.modelSelector.Value())
	apiKey := m.apiKeyInput.Value()

	if name == "" {
		m.errMsg = "name required"
		m.focused = 1
		m.refocus()
		return m, nil
	}
	if baseURL == "" {
		m.errMsg = "base URL required"
		m.focused = 2
		m.refocus()
		return m, nil
	}
	if model == "" {
		m.errMsg = "model required"
		m.focused = 3
		m.refocus()
		return m, nil
	}
	// On EDIT, the API key is allowed to be empty (means "keep current").
	// On ADD, presets that need a key require one — except for trust-mode
	// providers like local Ollama.
	preset := m.presets[m.presetIdx]
	if !m.isEdit && preset.NeedsKey && apiKey == "" {
		m.errMsg = "API key required (leave empty only for trust setups like local Ollama)"
		m.focused = 4
		m.refocus()
		return m, nil
	}

	return m, func() tea.Msg {
		return AISetupSubmitMsg{
			Name:           name,
			BaseURL:        baseURL,
			Model:          model,
			APIKey:         apiKey,
			TimeoutSeconds: 30,
			IsEdit:         m.isEdit,
		}
	}
}

// View renders the wizard with the preset row at the top and the three
// edit fields below.
func (m AISetupModel) View() string {
	boxW := m.width - 8
	if boxW < 60 {
		boxW = 60
	}
	if boxW > 100 {
		boxW = 100
	}

	// Preset row
	presetRow := make([]string, len(m.presets))
	for i, p := range m.presets {
		switch {
		case i == m.presetIdx && m.focused == 0:
			presetRow[i] = lipgloss.NewStyle().
				Foreground(CtpPink).
				Bold(true).
				Render("[ " + p.Name + " ]")
		case i == m.presetIdx:
			presetRow[i] = lipgloss.NewStyle().
				Foreground(CtpSubtext0).
				Render("[ " + p.Name + " ]")
		default:
			presetRow[i] = StyleDim.Render("  " + p.Name + "  ")
		}
	}

	label := func(text string, idx int) string {
		if m.focused == idx {
			return lipgloss.NewStyle().Foreground(CtpPink).Bold(true).Render("▸ " + text)
		}
		return StyleDim.Render("  " + text)
	}

	title := "Configure AI provider"
	if m.isEdit {
		title = "Edit AI profile: " + m.nameInput.Value()
	}

	nameField := m.nameInput.View()
	if m.isEdit {
		nameField = StyleDim.Render("(name locked — delete and re-add to rename)")
	}

	body := StyleTitle.Render(title) + "\n\n" +
		label("Provider", 0) + "\n" +
		strings.Join(presetRow, "  ") + "\n" +
		StyleDim.Render("    ←/→ to choose") + "\n\n" +
		label("Profile name", 1) + "\n" + nameField + "\n\n" +
		label("Base URL", 2) + "\n" + m.baseURLInput.View() + "\n\n" +
		label("Model", 3) + "\n" + m.modelSelector.View() + "\n" +
		StyleDim.Render("    ←/→ to choose · Other… to type a custom id") + "\n\n" +
		label("API Key", 4) + "\n" + m.apiKeyInput.View() + "\n" +
		StyleDim.Render("    saved to your OS keyring; leave empty for Ollama trust mode") + "\n"

	if m.errMsg != "" {
		body += "\n" + StyleError.Render(m.errMsg) + "\n"
	}

	body += "\n" + StyleHelp.Render("Tab next · Shift+Tab prev · ←/→ preset · Enter save · Esc cancel")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(CtpGreen).
		Padding(1, 2).
		Width(boxW).
		Render(body)
}
