package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// JoinChoiceMsg is emitted when the user picks how to start a new join while
// a join chain is already active. Exactly one of the booleans is true.
type JoinChoiceMsg struct {
	Add     bool
	Replace bool
	Cancel  bool
}

// JoinChoiceModel is a tiny prompt shown when the user presses J on a
// data viewer that's already showing a JOIN result.
type JoinChoiceModel struct {
	chainTables []string // table names in the current chain, in order
	width       int
	height      int
}

// NewJoinChoiceModel builds the prompt with the current chain summary.
func NewJoinChoiceModel(chainTables []string, width, height int) JoinChoiceModel {
	return JoinChoiceModel{chainTables: chainTables, width: width, height: height}
}

// Update handles a/r/Esc.
func (m JoinChoiceModel) Update(msg tea.Msg) (JoinChoiceModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "a", "A":
			return m, func() tea.Msg { return JoinChoiceMsg{Add: true} }
		case "r", "R":
			return m, func() tea.Msg { return JoinChoiceMsg{Replace: true} }
		case "esc":
			return m, func() tea.Msg { return JoinChoiceMsg{Cancel: true} }
		}
	}
	return m, nil
}

// View renders the bordered prompt with the chain context.
func (m JoinChoiceModel) View() string {
	chain := strings.Join(m.chainTables, " ⟶ ")
	if chain == "" {
		chain = "(none)"
	}

	boxW := m.width - 8
	if boxW < 50 {
		boxW = 50
	}
	if boxW > 90 {
		boxW = 90
	}

	body := StyleTitle.Render("Continue join?") + "\n\n" +
		StyleDim.Render("Currently joined: ") + chain + "\n\n" +
		"  " + lipgloss.NewStyle().Foreground(lipgloss.Color("114")).Bold(true).Render("A") +
		"  " + StyleDim.Render("Add another JOIN onto the chain") + "\n" +
		"  " + lipgloss.NewStyle().Foreground(lipgloss.Color("173")).Bold(true).Render("R") +
		"  " + StyleDim.Render("Replace — start a new JOIN from scratch") + "\n" +
		"  " + StyleDim.Render("Esc cancel")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("33")).
		Padding(1, 2).
		Width(boxW).
		Render(body)
}
