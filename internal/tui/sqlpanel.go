package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textarea"
)

// SqlPanelModel is the raw SQL input panel.
type SqlPanelModel struct {
	textarea textarea.Model
	active   bool
	width    int
	height   int
}

// NewSqlPanelModel creates a SqlPanelModel.
func NewSqlPanelModel(width, height int) SqlPanelModel {
	ta := textarea.New()
	ta.Placeholder = "Enter SQL..."
	ta.Focus()
	ta.SetWidth(width - 4)
	ta.SetHeight(height / 3)

	return SqlPanelModel{
		textarea: ta,
		active:   true,
		width:    width,
		height:   height,
	}
}

// IsActive returns whether the panel is in an active state (not closing).
func (m SqlPanelModel) IsActive() bool { return m.active }

// Init implements tea.Model.
func (m SqlPanelModel) Init() tea.Cmd { return textarea.Blink }

// Update implements tea.Model.
func (m SqlPanelModel) Update(msg tea.Msg) (SqlPanelModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.active = false
			return m, nil
		case "ctrl+enter", "f5":
			// Execute SQL (handled by App)
			sql := m.textarea.Value()
			return m, func() tea.Msg {
				return SqlExecuteMsg{SQL: sql}
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textarea.SetWidth(msg.Width - 4)
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

// View renders the SQL panel.
func (m SqlPanelModel) View() string {
	return StyleActiveBorder.
		Width(m.width - 2).
		Render(
			StyleTitle.Render("SQL Panel") + "\n" +
				StyleHelp.Render("Ctrl+Enter: execute · Esc: close") + "\n\n" +
				m.textarea.View(),
		)
}

// Value returns the current SQL text.
func (m SqlPanelModel) Value() string { return m.textarea.Value() }

// SqlExecuteMsg is emitted when the user executes SQL.
type SqlExecuteMsg struct {
	SQL string
}
