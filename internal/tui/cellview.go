package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
)

// CellViewModel is a scrollable viewport for viewing large cell values.
type CellViewModel struct {
	viewport viewport.Model
	active   bool
}

// NewCellViewModel creates a CellViewModel showing the given content.
func NewCellViewModel(content string, width, height int) CellViewModel {
	vp := viewport.New(width-4, height-6)
	vp.SetContent(content)
	return CellViewModel{viewport: vp, active: true}
}

// Init implements tea.Model.
func (m CellViewModel) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m CellViewModel) Update(msg tea.Msg) (CellViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.active = false
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// IsActive returns whether the viewport is still open.
func (m CellViewModel) IsActive() bool { return m.active }

// View renders the cell viewport.
func (m CellViewModel) View() string {
	return StyleActiveBorder.
		Width(m.viewport.Width + 4).
		Height(m.viewport.Height + 4).
		Render(
			StyleTitle.Render("Cell View") + "\n" +
				StyleHelp.Render("↑/↓: scroll · q/Esc: back") + "\n" +
				m.viewport.View(),
		)
}
