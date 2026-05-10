package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ModelSelectorModel is a single-line cycler over a list of model
// identifiers, with a sentinel "Other…" option at the end that reveals
// an inline textinput for free-form custom values. Useful where the
// space of valid choices is small and well-known but still open-ended.
type ModelSelectorModel struct {
	options     []string // last entry is always "Other…"
	selected    int
	customInput textinput.Model
	focused     bool
}

// NewModelSelector builds a selector seeded with presetModels (which
// must NOT include "Other…" — the constructor appends it). If current
// matches one of the preset entries, that one is pre-selected; if not
// (and current is non-empty), it becomes the custom value behind the
// "Other…" sentinel.
func NewModelSelector(presetModels []string, current string) ModelSelectorModel {
	opts := make([]string, 0, len(presetModels)+1)
	opts = append(opts, presetModels...)
	opts = append(opts, "Other…")

	custom := textinput.New()
	custom.CharLimit = 64
	custom.Width = 50
	custom.Placeholder = "type custom model id"

	sel := 0
	matched := false
	for i, o := range presetModels {
		if o == current {
			sel = i
			matched = true
			break
		}
	}
	if !matched && strings.TrimSpace(current) != "" {
		sel = len(opts) - 1
		custom.SetValue(current)
	}

	return ModelSelectorModel{
		options:     opts,
		selected:    sel,
		customInput: custom,
	}
}

// Value returns the active model identifier — either the highlighted
// preset entry or the custom textinput's content when "Other…" is on.
func (m ModelSelectorModel) Value() string {
	if m.IsCustom() {
		return strings.TrimSpace(m.customInput.Value())
	}
	return m.options[m.selected]
}

// IsCustom reports whether the "Other…" sentinel is currently selected.
func (m ModelSelectorModel) IsCustom() bool {
	return len(m.options) > 0 && m.selected == len(m.options)-1
}

// SetOptions replaces the preset list and tries to keep the user's
// previous choice (matched by string). Called by the parent form when
// the provider preset changes.
func (m *ModelSelectorModel) SetOptions(presetModels []string, current string) {
	prev := m.Value()
	if current == "" {
		current = prev
	}
	opts := make([]string, 0, len(presetModels)+1)
	opts = append(opts, presetModels...)
	opts = append(opts, "Other…")
	m.options = opts

	m.selected = 0
	for i, o := range presetModels {
		if o == current {
			m.selected = i
			m.customInput.SetValue("")
			m.refocusCustom()
			return
		}
	}
	if strings.TrimSpace(current) != "" {
		m.selected = len(m.options) - 1
		m.customInput.SetValue(current)
	}
	m.refocusCustom()
}

// Focus marks the selector as the active form field.
func (m *ModelSelectorModel) Focus() {
	m.focused = true
	m.refocusCustom()
}

// Blur removes focus from the selector and any embedded textinput.
func (m *ModelSelectorModel) Blur() {
	m.focused = false
	m.customInput.Blur()
}

// Update handles ←/→ cycling and forwards typing to the embedded
// textinput while "Other…" is selected.
func (m ModelSelectorModel) Update(msg tea.Msg) (ModelSelectorModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "left", "h":
			m.selected = (m.selected - 1 + len(m.options)) % len(m.options)
			m.refocusCustom()
			return m, nil
		case "right", "l":
			m.selected = (m.selected + 1) % len(m.options)
			m.refocusCustom()
			return m, nil
		}
	}
	if m.IsCustom() {
		var cmd tea.Cmd
		m.customInput, cmd = m.customInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *ModelSelectorModel) refocusCustom() {
	if m.focused && m.IsCustom() {
		m.customInput.Focus()
	} else {
		m.customInput.Blur()
	}
}

// View renders a one-line `‹ option ›  (n/N)` plus, when "Other…" is on
// AND the selector is focused, the embedded textinput on the next line.
func (m ModelSelectorModel) View() string {
	if len(m.options) == 0 {
		return StyleDim.Render("(no models)")
	}
	label := m.options[m.selected]
	if m.focused {
		label = lipgloss.NewStyle().
			Foreground(CtpPink).
			Bold(true).
			Render("‹ " + label + " ›")
	} else {
		label = lipgloss.NewStyle().
			Foreground(CtpSubtext0).
			Render("[ " + label + " ]")
	}
	counter := StyleDim.Render(fmt.Sprintf(" (%d/%d)", m.selected+1, len(m.options)))

	out := label + counter
	if m.IsCustom() {
		out += "\n" + m.customInput.View()
	}
	return out
}
