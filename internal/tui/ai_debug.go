package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AIDebugRetryMsg is emitted when the user wants to send the failed
// query plus their hint back to the AI for a corrected attempt.
type AIDebugRetryMsg struct {
	Question    string // the original natural-language question
	PreviousSQL string // the SQL the AI generated last time
	Error       string // the DB error from running PreviousSQL
	Hint        string // user-supplied additional context
}

// AIDebugCancelMsg is emitted when the user dismisses the debug panel.
type AIDebugCancelMsg struct{}

// AIDebugEditMsg is emitted when the user wants to take over manually
// from the failed SQL — the App opens the SQL editor pre-filled with it.
type AIDebugEditMsg struct {
	SQL string
}

// AIDebugModel is the panel shown when an AI-driven SQL execution fails.
// It surfaces the failure context (question / SQL / error) and lets the
// user collaborate with the AI by typing a hint that's bundled into a
// retry prompt — closing the loop on the "fix it together" workflow.
type AIDebugModel struct {
	question string
	sql      string
	errMsg   string
	hint     textinput.Model
	width    int
	height   int
	pending  bool // true while the retry request is in flight
}

// NewAIDebugModel builds a fresh debug panel for the given failure.
func NewAIDebugModel(question, sql, errMsg string, width, height int) AIDebugModel {
	hint := textinput.New()
	hint.Placeholder = "Add a hint for the AI (optional) — Enter to retry · Esc to cancel"
	hint.CharLimit = 512
	hint.Width = max(40, width-12)
	hint.Focus()

	return AIDebugModel{
		question: question,
		sql:      sql,
		errMsg:   errMsg,
		hint:     hint,
		width:    width,
		height:   height,
	}
}

// SetPending marks the retry as in-flight so the user knows their hint
// has been submitted and we're waiting on the AI.
func (m *AIDebugModel) SetPending(p bool) {
	m.pending = p
	if p {
		m.hint.Blur()
	} else {
		m.hint.Focus()
	}
}

// Init satisfies tea.Model.
func (m AIDebugModel) Init() tea.Cmd { return textinput.Blink }

// Update handles Enter (retry), Ctrl+E (manual edit handoff), and Esc.
func (m AIDebugModel) Update(msg tea.Msg) (AIDebugModel, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			return m, func() tea.Msg { return AIDebugCancelMsg{} }
		case "enter":
			if m.pending {
				return m, nil
			}
			return m, func() tea.Msg {
				return AIDebugRetryMsg{
					Question:    m.question,
					PreviousSQL: m.sql,
					Error:       m.errMsg,
					Hint:        m.hint.Value(),
				}
			}
		case "ctrl+e":
			return m, func() tea.Msg { return AIDebugEditMsg{SQL: m.sql} }
		}
	}
	if m.pending {
		return m, nil
	}
	var cmd tea.Cmd
	m.hint, cmd = m.hint.Update(msg)
	return m, cmd
}

// View renders the bordered debug panel.
func (m AIDebugModel) View() string {
	boxW := m.width - 8
	if boxW < 60 {
		boxW = 60
	}
	if boxW > 110 {
		boxW = 110
	}

	question := strings.TrimSpace(m.question)
	if question == "" {
		question = StyleDim.Render("(no question recorded)")
	}

	body := StyleTitle.Render("AI debug — query failed") + "\n\n" +
		StyleDim.Render("Question") + "\n" +
		question + "\n\n" +
		StyleDim.Render("Generated SQL") + "\n" +
		HighlightSQL(m.sql) + "\n\n" +
		StyleDim.Render("Error") + "\n" +
		StyleError.Render(m.errMsg) + "\n\n" +
		StyleDim.Render("Your hint (optional)") + "\n" +
		m.hint.View() + "\n"

	if m.pending {
		body += "\n" + lipgloss.NewStyle().
			Foreground(CtpPeach).
			Bold(true).
			Render("⏳ Asking the AI to fix it…") + "\n"
	}

	body += "\n" + StyleHelp.Render("Enter retry with hint · Ctrl+e edit SQL manually · Esc cancel")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(CtpRed).
		Padding(1, 2).
		Width(boxW).
		Render(body)
}
