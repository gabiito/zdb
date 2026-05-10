package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PasswordPromptSubmitMsg is emitted when the user confirms the password.
// Empty passwords are allowed — the engine decides whether they're valid.
type PasswordPromptSubmitMsg struct {
	Password string
}

// PasswordPromptCancelMsg is emitted on Esc.
type PasswordPromptCancelMsg struct{}

// PasswordPromptModel is a one-field modal asking the user to type a password
// at connect time. Used for connections saved without a stored secret (no
// keyring, no env var, DSN contains a `{password}` placeholder).
type PasswordPromptModel struct {
	connName  string
	pwInput   textinput.Model
	width     int
	height    int
	connecting bool
	errMsg    string
}

// NewPasswordPromptModel builds a prompt for the named connection.
func NewPasswordPromptModel(connName string, width, height int) PasswordPromptModel {
	pw := textinput.New()
	pw.Placeholder = "(empty for trust auth)"
	pw.CharLimit = 256
	pw.Width = 50
	pw.EchoMode = textinput.EchoPassword
	pw.EchoCharacter = '•'
	pw.Focus()

	return PasswordPromptModel{
		connName: connName,
		pwInput:  pw,
		width:    width,
		height:   height,
	}
}

// SetConnecting toggles the in-flight state while the App attempts the
// connection. While connecting, only Esc is honored.
func (m *PasswordPromptModel) SetConnecting(v bool) { m.connecting = v }

// SetError displays an error returned by the connect attempt so the user can
// retry without leaving the modal.
func (m *PasswordPromptModel) SetError(s string) { m.errMsg = s; m.connecting = false }

// Init satisfies tea.Model.
func (m PasswordPromptModel) Init() tea.Cmd { return textinput.Blink }

// Update handles Enter and Esc. Anything else flows into the textinput.
func (m PasswordPromptModel) Update(msg tea.Msg) (PasswordPromptModel, tea.Cmd) {
	if m.connecting {
		if k, ok := msg.(tea.KeyMsg); ok && k.String() == "esc" {
			return m, func() tea.Msg { return PasswordPromptCancelMsg{} }
		}
		return m, nil
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			return m, func() tea.Msg { return PasswordPromptCancelMsg{} }
		case "enter":
			return m, func() tea.Msg {
				return PasswordPromptSubmitMsg{Password: m.pwInput.Value()}
			}
		}
	}
	var cmd tea.Cmd
	m.pwInput, cmd = m.pwInput.Update(msg)
	return m, cmd
}

// View renders the bordered prompt.
func (m PasswordPromptModel) View() string {
	boxW := m.width - 8
	if boxW < 50 {
		boxW = 50
	}
	if boxW > 80 {
		boxW = 80
	}

	body := StyleTitle.Render("Password for "+m.connName) + "\n\n" +
		m.pwInput.View() + "\n"

	if m.errMsg != "" {
		body += "\n" + StyleError.Render(m.errMsg) + "\n"
	}
	if m.connecting {
		body += "\n" + lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true).
			Render("⏳ Connecting...") + "\n"
	}

	hint := "Enter connect · Esc cancel"
	if m.connecting {
		hint = "Esc cancels"
	}
	body += "\n" + StyleHelp.Render(hint)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("114")).
		Padding(1, 2).
		Width(boxW).
		Render(body)
}
