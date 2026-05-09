package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// ConfirmModel renders a confirmation banner.
// Red style for delete, yellow for mutating SQL, neutral otherwise.
type ConfirmModel struct {
	prompt string
	red    bool
}

// NewConfirmModel creates a ConfirmModel.
func NewConfirmModel(prompt string, red bool) ConfirmModel {
	return ConfirmModel{prompt: prompt, red: red}
}

// Init implements tea.Model.
func (m ConfirmModel) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y":
			return m, func() tea.Msg { return ConfirmYesMsg{} }
		default:
			return m, func() tea.Msg { return ConfirmNoMsg{} }
		}
	}
	return m, nil
}

// View renders the confirmation banner.
func (m ConfirmModel) View() string {
	style := StyleBannerYellow
	if m.red {
		style = StyleBannerRed
	}
	return style.Render(m.prompt + " [y/N]")
}

// NoticeModel renders a non-blocking info banner.
type NoticeModel struct {
	text string
}

// NewNoticeModel creates a NoticeModel.
func NewNoticeModel(text string) NoticeModel {
	return NoticeModel{text: text}
}

// View renders the notice.
func (m NoticeModel) View() string {
	return StyleBannerYellow.Render(m.text)
}
