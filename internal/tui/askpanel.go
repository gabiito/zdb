package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
)

// AskPanelModel is the AI Ask panel.
type AskPanelModel struct {
	input      textinput.Model
	preview    string
	aiEnabled  bool
	active     bool
	hasPreview bool
	width      int
	height     int
}

// NewAskPanelModel creates an AskPanelModel.
func NewAskPanelModel(aiEnabled bool, width, height int) AskPanelModel {
	ti := textinput.New()
	ti.Placeholder = "Ask a question about your data..."
	ti.Width = width - 8
	if aiEnabled {
		ti.Focus()
	}

	return AskPanelModel{
		input:     ti,
		aiEnabled: aiEnabled,
		active:    true,
		width:     width,
		height:    height,
	}
}

// IsActive returns whether the panel is still active.
func (m AskPanelModel) IsActive() bool { return m.active }

// SetPreview sets the AI-generated SQL preview.
func (m *AskPanelModel) SetPreview(sql string) {
	m.preview = sql
	m.hasPreview = true
}

// Init implements tea.Model.
func (m AskPanelModel) Init() tea.Cmd {
	if m.aiEnabled {
		return textinput.Blink
	}
	return nil
}

// Update implements tea.Model.
func (m AskPanelModel) Update(msg tea.Msg) (AskPanelModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.active = false
			return m, nil
		case "enter":
			if m.aiEnabled && !m.hasPreview {
				question := m.input.Value()
				if question != "" {
					return m, func() tea.Msg {
						return AskSubmitMsg{Question: question}
					}
				}
			}
		case "y", "ctrl+enter":
			if m.hasPreview && m.preview != "" {
				sql := m.preview
				return m, func() tea.Msg {
					return SqlExecuteMsg{SQL: sql}
				}
			}
		}
	}

	if m.aiEnabled && !m.hasPreview {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the Ask panel.
func (m AskPanelModel) View() string {
	if !m.aiEnabled {
		return StyleActiveBorder.
			Width(m.width - 2).
			Render(
				StyleTitle.Render("AI Ask") + "\n\n" +
					StyleDim.Render("configure AI to enable") + "\n\n" +
					StyleHelp.Render("Esc: close"),
			)
	}

	var content string
	if m.hasPreview {
		content = StyleTitle.Render("AI Ask — SQL Preview") + "\n\n" +
			m.preview + "\n\n" +
			StyleHelp.Render("y/Ctrl+Enter: execute · Esc: cancel")
	} else {
		content = StyleTitle.Render("AI Ask") + "\n\n" +
			m.input.View() + "\n\n" +
			StyleHelp.Render("Enter: submit · Esc: cancel")
	}

	return StyleActiveBorder.
		Width(m.width - 2).
		Render(content)
}

// AskSubmitMsg is emitted when the user submits an AI question.
type AskSubmitMsg struct {
	Question string
}
