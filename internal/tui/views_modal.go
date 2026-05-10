package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ViewItem wraps a saved view for display in the bubbles/list.
type ViewItem struct {
	Name string
	SQL  string
}

func (i ViewItem) Title() string { return i.Name }
func (i ViewItem) Description() string {
	s := strings.ReplaceAll(i.SQL, "\n", " ")
	if len(s) > 80 {
		s = s[:77] + "…"
	}
	return s
}
func (i ViewItem) FilterValue() string { return i.Name }

// RunViewMsg is emitted when the user picks a view to execute.
type RunViewMsg struct{ SQL string }

// DeleteViewMsg is emitted when the user requests deletion (capital D in the list).
type DeleteViewMsg struct{ Name string }

// CloseViewsMsg is emitted on Esc.
type CloseViewsMsg struct{}

// ViewsListModel renders the saved-views modal.
type ViewsListModel struct {
	list   list.Model
	width  int
	height int
}

// NewViewsListModel builds a fresh modal sized for the terminal.
func NewViewsListModel(items []ViewItem, width, height int) ViewsListModel {
	listItems := make([]list.Item, len(items))
	for i, v := range items {
		listItems[i] = v
	}

	boxW := width - 8
	if boxW < 50 {
		boxW = 50
	}
	if boxW > 100 {
		boxW = 100
	}
	boxH := height - 8
	if boxH < 12 {
		boxH = 12
	}
	if boxH > 30 {
		boxH = 30
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(CtpMauve)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(CtpMauve)

	l := list.New(listItems, delegate, boxW-4, boxH-6)
	l.Title = "Saved views"
	l.Styles.Title = StyleTitle
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	if len(items) == 0 {
		l.SetShowFilter(false)
	}

	return ViewsListModel{list: l, width: width, height: height}
}

// Update routes keys: Enter to run, D to delete, Esc to close, otherwise list.
func (m ViewsListModel) Update(msg tea.Msg) (ViewsListModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			return m, func() tea.Msg { return CloseViewsMsg{} }
		case "enter":
			if item, ok := m.list.SelectedItem().(ViewItem); ok {
				return m, func() tea.Msg { return RunViewMsg{SQL: item.SQL} }
			}
		case "D":
			if item, ok := m.list.SelectedItem().(ViewItem); ok {
				return m, func() tea.Msg { return DeleteViewMsg{Name: item.Name} }
			}
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the modal box.
func (m ViewsListModel) View() string {
	boxW := m.width - 8
	if boxW < 50 {
		boxW = 50
	}
	if boxW > 100 {
		boxW = 100
	}

	body := m.list.View()
	if len(m.list.Items()) == 0 {
		body = StyleDim.Render("(no saved views — press W on a query result to save one)")
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(CtpMauve).
		Padding(1, 2).
		Width(boxW).
		Render(
			body + "\n\n" +
				StyleHelp.Render("Enter run · D delete · Esc close"),
		)
}

// SaveViewSubmitMsg fires when the user confirms a name for the view.
type SaveViewSubmitMsg struct {
	Name string
	SQL  string
}

// SaveViewCancelMsg fires when the user cancels.
type SaveViewCancelMsg struct{}

// SaveViewModel is the textinput modal for naming a view.
type SaveViewModel struct {
	input  textinput.Model
	sql    string
	width  int
	height int
}

// NewSaveViewModel builds the save dialog with the given SQL preview.
func NewSaveViewModel(sql string, width, height int) SaveViewModel {
	ti := textinput.New()
	ti.Placeholder = "name your view"
	ti.Focus()
	ti.CharLimit = 80
	ti.Width = 50
	return SaveViewModel{input: ti, sql: sql, width: width, height: height}
}

// Init satisfies tea.Model.
func (m SaveViewModel) Init() tea.Cmd { return textinput.Blink }

// Update handles Enter (submit) / Esc (cancel) and forwards typing.
func (m SaveViewModel) Update(msg tea.Msg) (SaveViewModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			return m, func() tea.Msg { return SaveViewCancelMsg{} }
		case "enter":
			name := strings.TrimSpace(m.input.Value())
			if name == "" {
				return m, nil
			}
			return m, func() tea.Msg {
				return SaveViewSubmitMsg{Name: name, SQL: m.sql}
			}
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View renders the bordered prompt with a syntax-highlighted SQL preview.
func (m SaveViewModel) View() string {
	boxW := m.width - 8
	if boxW < 50 {
		boxW = 50
	}
	if boxW > 100 {
		boxW = 100
	}

	preview := m.sql
	if len(preview) > 240 {
		preview = preview[:237] + "..."
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(CtpMauve).
		Padding(1, 2).
		Width(boxW).
		Render(
			StyleTitle.Render("Save view") + "\n\n" +
				StyleDim.Render("SQL:") + "\n" +
				HighlightSQL(preview) + "\n\n" +
				StyleDim.Render("Name:") + " " + m.input.View() + "\n\n" +
				StyleHelp.Render("Enter save · Esc cancel"),
		)
}
