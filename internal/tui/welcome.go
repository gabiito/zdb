package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// WelcomeAddConnectionMsg is emitted when the user presses 'n' on the welcome
// screen to start adding their first connection.
type WelcomeAddConnectionMsg struct{}

// WelcomeQuitMsg is emitted when the user presses 'q' on the welcome screen.
type WelcomeQuitMsg struct{}

// WelcomeModel renders the first-run welcome screen shown when the config has
// no connections yet.
type WelcomeModel struct {
	width  int
	height int
}

// NewWelcomeModel builds a WelcomeModel sized for the terminal.
func NewWelcomeModel(width, height int) WelcomeModel {
	return WelcomeModel{width: width, height: height}
}

// Init implements tea.Model.
func (m WelcomeModel) Init() tea.Cmd { return nil }

// Update handles the two keys the welcome screen accepts.
func (m WelcomeModel) Update(msg tea.Msg) (WelcomeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "n", "N", "enter":
			return m, func() tea.Msg { return WelcomeAddConnectionMsg{} }
		case "q":
			return m, func() tea.Msg { return WelcomeQuitMsg{} }
		}
	}
	return m, nil
}

// View renders the welcome panel.
func (m WelcomeModel) View() string {
	boxW := m.width - 8
	if boxW < 50 {
		boxW = 50
	}
	if boxW > 80 {
		boxW = 80
	}

	title := lipgloss.NewStyle().
		Foreground(CtpGreen).
		Bold(true).
		Render("Welcome to zDB")

	subtitle := StyleDim.Render(
		"You don't have any database connections configured yet.",
	)

	keyN := lipgloss.NewStyle().
		Foreground(CtpSapphire).
		Bold(true).
		Render("[n]")
	keyQ := lipgloss.NewStyle().
		Foreground(CtpSapphire).
		Bold(true).
		Render("[q]")

	actions := keyN + " add a new connection\n" +
		keyQ + " quit"

	body := title + "\n\n" + subtitle + "\n\n" + actions

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(CtpGreen).
		Padding(1, 3).
		Width(boxW).
		Align(lipgloss.Center).
		Render(body)
}
