package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gabiito/zdb/internal/config"
)

// EditConnectionSubmitMsg is emitted when the edit form passes local
// validation. The Original field carries the connection as it was BEFORE the
// edit so the App can locate the entry to replace and (if the name changed)
// migrate the keyring secret. PasswordChanged is true when the user typed
// something into the password field — when false, the existing keyring entry
// is preserved as-is.
type EditConnectionSubmitMsg struct {
	Original        config.Connection
	Updated         config.Connection
	Password        string
	PasswordChanged bool
}

// EditConnectionCancelMsg is emitted on Esc.
type EditConnectionCancelMsg struct{}

// EditConnectionModel is the form for editing an existing connection. It
// pre-fills name, engine, and DSN from the original connection. The password
// field is always blank — the user types a new password only when they want
// to rotate it.
type EditConnectionModel struct {
	original      config.Connection
	nameInput     textinput.Model
	engine        EngineSelectorModel
	dsnInput      textinput.Model
	passwordInput textinput.Model
	focused       int
	width         int
	height        int
	errMsg        string
	testing       bool
}

// SetTesting toggles the in-progress flag while the App tests the edited
// connection asynchronously.
func (m *EditConnectionModel) SetTesting(v bool) { m.testing = v }

// SetTestError displays an async test failure inside the modal.
func (m *EditConnectionModel) SetTestError(s string) {
	m.errMsg = "test failed: " + s
	m.testing = false
}

// SetError displays an arbitrary error message.
func (m *EditConnectionModel) SetError(s string) {
	m.errMsg = s
	m.testing = false
}

// NewEditConnectionModel builds an edit form pre-filled from the given
// connection. The DSN shown is the stored DSN, which may contain a
// `{password}` placeholder — that's fine, the form treats it as a template.
func NewEditConnectionModel(original config.Connection, width, height int) EditConnectionModel {
	name := textinput.New()
	name.CharLimit = 60
	name.Width = 50
	name.SetValue(original.Name)
	name.Focus()

	engine := NewEngineSelector(original.Engine)

	dsn := textinput.New()
	dsn.CharLimit = 1024
	dsn.Width = 50
	dsn.SetValue(original.DSN)

	password := textinput.New()
	password.Placeholder = "leave empty to keep the current password"
	password.CharLimit = 256
	password.Width = 50
	password.EchoMode = textinput.EchoPassword
	password.EchoCharacter = '•'

	return EditConnectionModel{
		original:      original,
		nameInput:     name,
		engine:        engine,
		dsnInput:      dsn,
		passwordInput: password,
		focused:       0,
		width:         width,
		height:        height,
	}
}

// Init satisfies tea.Model.
func (m EditConnectionModel) Init() tea.Cmd { return textinput.Blink }

// Update handles Tab cycling, Enter submission, and Esc cancellation.
func (m EditConnectionModel) Update(msg tea.Msg) (EditConnectionModel, tea.Cmd) {
	if m.testing {
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
			return m, func() tea.Msg { return EditConnectionCancelMsg{} }
		}
		return m, nil
	}
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			return m, func() tea.Msg { return EditConnectionCancelMsg{} }
		case "tab":
			m.focused = (m.focused + 1) % 4
			m.refocus()
			return m, nil
		case "shift+tab":
			m.focused = (m.focused + 3) % 4
			m.refocus()
			return m, nil
		case "enter":
			name := strings.TrimSpace(m.nameInput.Value())
			engine := m.engine.Value()
			dsn := strings.TrimSpace(m.dsnInput.Value())
			password := m.passwordInput.Value()
			if name == "" {
				m.errMsg = "name required"
				m.focused = 0
				m.refocus()
				return m, nil
			}
			if dsn == "" {
				m.errMsg = "DSN required"
				m.focused = 2
				m.refocus()
				return m, nil
			}
			updated := config.Connection{
				Name:       name,
				Engine:     engine,
				DSN:        dsn,
				KeyringKey: m.original.KeyringKey,
				DSNEnv:     m.original.DSNEnv,
			}
			return m, func() tea.Msg {
				return EditConnectionSubmitMsg{
					Original:        m.original,
					Updated:         updated,
					Password:        password,
					PasswordChanged: password != "",
				}
			}
		}
	}

	var cmd tea.Cmd
	switch m.focused {
	case 0:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case 1:
		m.engine, cmd = m.engine.Update(msg)
	case 2:
		m.dsnInput, cmd = m.dsnInput.Update(msg)
	case 3:
		m.passwordInput, cmd = m.passwordInput.Update(msg)
	}
	return m, cmd
}

func (m *EditConnectionModel) refocus() {
	m.nameInput.Blur()
	m.engine.Blur()
	m.dsnInput.Blur()
	m.passwordInput.Blur()
	switch m.focused {
	case 0:
		m.nameInput.Focus()
	case 1:
		m.engine.Focus()
	case 2:
		m.dsnInput.Focus()
	case 3:
		m.passwordInput.Focus()
	}
}

// View renders the bordered form.
func (m EditConnectionModel) View() string {
	boxW := m.width - 8
	if boxW < 60 {
		boxW = 60
	}
	if boxW > 100 {
		boxW = 100
	}

	label := func(text string, idx int) string {
		if m.focused == idx {
			return lipgloss.NewStyle().Foreground(lipgloss.Color("114")).Bold(true).Render("▸ " + text)
		}
		return StyleDim.Render("  " + text)
	}

	body := StyleTitle.Render("Edit connection: "+m.original.Name) + "\n\n" +
		label("Name", 0) + "\n" + m.nameInput.View() + "\n\n" +
		label("Engine", 1) + "\n" + m.engine.View() + "\n" +
		StyleDim.Render("    ←/→ to choose") + "\n\n" +
		label("DSN", 2) + "\n" + m.dsnInput.View() + "\n\n" +
		label("New password (optional)", 3) + "\n" + m.passwordInput.View() + "\n" +
		StyleDim.Render("    leave empty to keep the existing password in the keyring") + "\n"

	if m.errMsg != "" {
		body += "\n" + StyleError.Render(m.errMsg) + "\n"
	}
	if m.testing {
		body += "\n" + lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true).
			Render("⏳ Testing connection...") + "\n"
	}

	hint := "Tab next · Shift+Tab prev · Enter test+save · Esc cancel"
	if m.testing {
		hint = "Esc cancels the test"
	}
	body += "\n" + StyleHelp.Render(hint)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("114")).
		Padding(1, 2).
		Width(boxW).
		Render(body)
}
