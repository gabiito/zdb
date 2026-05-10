package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gabiito/zdb/internal/config"
)

// AIProfileActivateMsg is emitted when the user picks a profile to set
// as the active one. The App re-initializes the provider with the new
// selection and persists ActiveAI.
type AIProfileActivateMsg struct{ Name string }

// AIProfileAddMsg is emitted to open the wizard for adding a new profile.
type AIProfileAddMsg struct{}

// AIProfileEditMsg is emitted to open the wizard pre-filled with the
// selected profile's values.
type AIProfileEditMsg struct{ Name string }

// AIProfileDeleteMsg is emitted to delete the selected profile (after
// confirmation handled by the App).
type AIProfileDeleteMsg struct{ Name string }

// AIProfileOpenAnalyticsMsg is emitted when the user wants to jump from
// the profile list to the analytics view.
type AIProfileOpenAnalyticsMsg struct{}

// AIProfileListCloseMsg is emitted on Esc.
type AIProfileListCloseMsg struct{}

// AIProfileListModel renders the list of configured AI profiles with
// the active one highlighted. From here the user can activate, add,
// edit, delete, or open the analytics view.
type AIProfileListModel struct {
	profiles []config.AIProfile
	active   string
	cursor   int
	width    int
	height   int
}

// NewAIProfileListModel builds a fresh list view for the given profiles.
func NewAIProfileListModel(profiles []config.AIProfile, active string, width, height int) AIProfileListModel {
	return AIProfileListModel{
		profiles: profiles,
		active:   active,
		width:    width,
		height:   height,
	}
}

// Init satisfies tea.Model.
func (m AIProfileListModel) Init() tea.Cmd { return nil }

// Update handles cursor moves + actions.
func (m AIProfileListModel) Update(msg tea.Msg) (AIProfileListModel, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			return m, func() tea.Msg { return AIProfileListCloseMsg{} }
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.profiles)-1 {
				m.cursor++
			}
		case "enter":
			if m.cursor < len(m.profiles) {
				name := m.profiles[m.cursor].Name
				return m, func() tea.Msg { return AIProfileActivateMsg{Name: name} }
			}
		case "a":
			return m, func() tea.Msg { return AIProfileAddMsg{} }
		case "e":
			if m.cursor < len(m.profiles) {
				name := m.profiles[m.cursor].Name
				return m, func() tea.Msg { return AIProfileEditMsg{Name: name} }
			}
		case "d":
			if m.cursor < len(m.profiles) {
				name := m.profiles[m.cursor].Name
				return m, func() tea.Msg { return AIProfileDeleteMsg{Name: name} }
			}
		case "s", "g":
			return m, func() tea.Msg { return AIProfileOpenAnalyticsMsg{} }
		}
	}
	return m, nil
}

// View renders the bordered profile list.
func (m AIProfileListModel) View() string {
	boxW := m.width - 8
	if boxW < 60 {
		boxW = 60
	}
	if boxW > 100 {
		boxW = 100
	}

	body := StyleTitle.Render("AI profiles") + "\n\n"

	if len(m.profiles) == 0 {
		body += StyleDim.Render("(no profiles yet)") + "\n\n" +
			StyleHelp.Render("a add · Esc close")
	} else {
		for i, p := range m.profiles {
			marker := "  "
			cursor := "  "
			if p.Name == m.active {
				marker = lipgloss.NewStyle().Foreground(CtpGreen).Bold(true).Render("●")
				marker = " " + marker
			}
			if i == m.cursor {
				cursor = lipgloss.NewStyle().Foreground(CtpPink).Bold(true).Render("▸ ")
			}
			line := fmt.Sprintf("%s%s%-15s %s · %s", cursor, marker, truncate(p.Name, 15), truncate(p.BaseURL, 40), p.Model)
			if i == m.cursor {
				line = lipgloss.NewStyle().Foreground(CtpText).Render(line)
			} else {
				line = StyleDim.Render(line)
			}
			body += line + "\n"
		}
		body += "\n" + StyleHelp.Render(
			"Enter activate · a add · e edit · d delete · g analytics · Esc close",
		)
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(CtpMauve).
		Padding(1, 2).
		Width(boxW).
		Render(body)
}

// truncate returns s clipped to n runes (with an ellipsis when clipped).
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return s[:n-1] + "…"
}
