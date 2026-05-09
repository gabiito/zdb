package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gabiito/db-viewer/internal/config"
)

// AddConnectionSubmitMsg is emitted when the form passes validation.
// Password, when non-empty, holds the literal password the user typed in
// the dedicated field — the App is expected to URL-encode it when building
// the DSN to test, and to store it in the keyring after a successful test.
// The Connection.DSN in that case is treated as a template (no password).
type AddConnectionSubmitMsg struct {
	Connection config.Connection
	Password   string
}

// AddConnectionCancelMsg is emitted on Esc.
type AddConnectionCancelMsg struct{}

// AddConnectionModel is a 4-field form: Name, Engine, DSN, Password (optional).
type AddConnectionModel struct {
	nameInput     textinput.Model
	engineInput   textinput.Model
	dsnInput      textinput.Model
	passwordInput textinput.Model
	focused       int // 0 name, 1 engine, 2 dsn, 3 password
	width         int
	height        int
	errMsg        string
	testing       bool
}

// SetTesting toggles the in-progress flag (App calls this while the connection
// is being verified asynchronously).
func (m *AddConnectionModel) SetTesting(v bool) { m.testing = v }

// SetTestError displays an async test failure inside the modal.
func (m *AddConnectionModel) SetTestError(s string) { m.errMsg = "test failed: " + s; m.testing = false }

// SetError displays an arbitrary error message (e.g. keyring failure after
// a successful connection test) without the "test failed" prefix.
func (m *AddConnectionModel) SetError(s string) { m.errMsg = s; m.testing = false }

// NewAddConnectionModel builds a fresh form sized for the terminal.
func NewAddConnectionModel(width, height int) AddConnectionModel {
	name := textinput.New()
	name.Placeholder = "name (e.g., demo-sqlite)"
	name.CharLimit = 60
	name.Width = 50
	name.Focus()

	engine := textinput.New()
	engine.Placeholder = "sqlite | postgres | mysql"
	engine.CharLimit = 16
	engine.Width = 50

	dsn := textinput.New()
	dsn.Placeholder = "/path/file.db · postgres://user@host/db · user@tcp(host:3306)/db"
	dsn.CharLimit = 1024
	dsn.Width = 50

	password := textinput.New()
	password.Placeholder = "leave empty if DSN already has it (URL-encoded)"
	password.CharLimit = 256
	password.Width = 50
	password.EchoMode = textinput.EchoPassword
	password.EchoCharacter = '•'

	return AddConnectionModel{
		nameInput:     name,
		engineInput:   engine,
		dsnInput:      dsn,
		passwordInput: password,
		focused:       0,
		width:         width,
		height:        height,
	}
}

// Init satisfies tea.Model.
func (m AddConnectionModel) Init() tea.Cmd { return textinput.Blink }

// Update handles Tab cycling and Enter submission.
func (m AddConnectionModel) Update(msg tea.Msg) (AddConnectionModel, tea.Cmd) {
	// While the App is testing the connection, only allow Esc.
	if m.testing {
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
			return m, func() tea.Msg { return AddConnectionCancelMsg{} }
		}
		return m, nil
	}
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			return m, func() tea.Msg { return AddConnectionCancelMsg{} }
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
			engine := strings.TrimSpace(m.engineInput.Value())
			dsn := strings.TrimSpace(m.dsnInput.Value())
			password := m.passwordInput.Value() // do NOT trim — passwords may begin/end with whitespace
			if name == "" {
				m.errMsg = "name required"
				m.focused = 0
				m.refocus()
				return m, nil
			}
			if engine != "sqlite" && engine != "postgres" && engine != "mysql" {
				m.errMsg = "engine must be one of: sqlite, postgres, mysql"
				m.focused = 1
				m.refocus()
				return m, nil
			}
			if dsn == "" {
				m.errMsg = "DSN required"
				m.focused = 2
				m.refocus()
				return m, nil
			}
			return m, func() tea.Msg {
				return AddConnectionSubmitMsg{
					Connection: config.Connection{Name: name, Engine: engine, DSN: dsn},
					Password:   password,
				}
			}
		}
	}

	var cmd tea.Cmd
	switch m.focused {
	case 0:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case 1:
		m.engineInput, cmd = m.engineInput.Update(msg)
	case 2:
		m.dsnInput, cmd = m.dsnInput.Update(msg)
	case 3:
		m.passwordInput, cmd = m.passwordInput.Update(msg)
	}
	return m, cmd
}

func (m *AddConnectionModel) refocus() {
	m.nameInput.Blur()
	m.engineInput.Blur()
	m.dsnInput.Blur()
	m.passwordInput.Blur()
	switch m.focused {
	case 0:
		m.nameInput.Focus()
	case 1:
		m.engineInput.Focus()
	case 2:
		m.dsnInput.Focus()
	case 3:
		m.passwordInput.Focus()
	}
}

// View renders the bordered form.
func (m AddConnectionModel) View() string {
	boxW := m.width - 8
	if boxW < 60 {
		boxW = 60
	}
	if boxW > 100 {
		boxW = 100
	}

	label := func(text string, idx int) string {
		if m.focused == idx {
			return lipgloss.NewStyle().Foreground(lipgloss.Color("114")).Bold(true).Render("▸ "+text)
		}
		return StyleDim.Render("  " + text)
	}

	body := StyleTitle.Render("Add database connection") + "\n\n" +
		label("Name", 0) + "\n" + m.nameInput.View() + "\n\n" +
		label("Engine", 1) + "\n" + m.engineInput.View() + "\n" +
		StyleDim.Render("    one of: sqlite, postgres, mysql") + "\n\n" +
		label("DSN", 2) + "\n" + m.dsnInput.View() + "\n\n" +
		label("Password (optional)", 3) + "\n" + m.passwordInput.View() + "\n" +
		StyleDim.Render("    fill this if your password contains @ or other special chars\n    when filled, the DSN is treated as a template (without password)") + "\n"

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
