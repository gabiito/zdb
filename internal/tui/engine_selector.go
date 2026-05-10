package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// EngineSelectorModel is a horizontal radio-style picker for the three
// supported engines. It replaces the free-text engine field in the connection
// forms — invalid engines simply cannot be entered, and the form gets a
// nicer visual at no cost.
type EngineSelectorModel struct {
	options  []string
	selected int
	focused  bool
}

// NewEngineSelector builds a selector pre-pointed at `initial` when it
// matches one of the engines; otherwise defaults to sqlite (index 0).
func NewEngineSelector(initial string) EngineSelectorModel {
	opts := []string{"sqlite", "postgres", "mysql"}
	sel := 0
	for i, o := range opts {
		if o == initial {
			sel = i
			break
		}
	}
	return EngineSelectorModel{options: opts, selected: sel}
}

// Value returns the currently selected engine string.
func (m EngineSelectorModel) Value() string { return m.options[m.selected] }

// Focus marks the selector as keyboard-active.
func (m *EngineSelectorModel) Focus() { m.focused = true }

// Blur removes focus.
func (m *EngineSelectorModel) Blur() { m.focused = false }

// Update handles arrow keys (and h/l) to cycle the selection. Returns no
// commands. The form decides how to interleave this with Tab/Shift+Tab/Enter.
func (m EngineSelectorModel) Update(msg tea.Msg) (EngineSelectorModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "left", "h":
			m.selected = (m.selected - 1 + len(m.options)) % len(m.options)
		case "right", "l":
			m.selected = (m.selected + 1) % len(m.options)
		}
	}
	return m, nil
}

// View renders the three options inline. The selected one is wrapped in
// brackets; when focused, it also gets a bright color.
func (m EngineSelectorModel) View() string {
	parts := make([]string, len(m.options))
	for i, opt := range m.options {
		switch {
		case i == m.selected && m.focused:
			parts[i] = lipgloss.NewStyle().
				Foreground(CtpGreen).
				Bold(true).
				Render("[ " + opt + " ]")
		case i == m.selected:
			parts[i] = lipgloss.NewStyle().
				Foreground(CtpSubtext0).
				Render("[ " + opt + " ]")
		default:
			parts[i] = StyleDim.Render("  " + opt + "  ")
		}
	}
	return strings.Join(parts, "  ")
}
