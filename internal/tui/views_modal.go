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

// ConnItem wraps a connection name for display in the copy-source picker.
type ConnItem struct {
	Name string
}

func (i ConnItem) Title() string       { return i.Name }
func (i ConnItem) Description() string { return "select to browse its views" }
func (i ConnItem) FilterValue() string { return i.Name }

// RunViewMsg is emitted when the user picks a view to execute.
type RunViewMsg struct{ SQL string }

// DeleteViewMsg is emitted when the user requests deletion (capital D in the list).
type DeleteViewMsg struct{ Name string }

// CloseViewsMsg is emitted on Esc.
type CloseViewsMsg struct{}

// CopyViewSelectedMsg is emitted when the user picks a source view in the
// copy-view flow (mode == modePickView). The App handles it by entering
// copy-mode and opening the SQL editor prefilled with the source SQL.
type CopyViewSelectedMsg struct {
	SourceConn string
	ViewName   string
	SQL        string
}

// ViewsListMode is the internal state of the views modal.
type ViewsListMode int

const (
	modeViews    ViewsListMode = iota // default: show current connection's views
	modePickConn                      // pick a source connection for copy-view
	modePickView                      // pick a source view from the chosen connection
)

// ViewsListModel renders the saved-views modal.
// In modeViews it shows the current connection's views.
// In modePickConn it shows a picker of all other connections.
// In modePickView it shows the chosen connection's views.
type ViewsListModel struct {
	list   list.Model
	width  int
	height int

	// mode tracks which step of the flow we are in.
	mode ViewsListMode

	// pickedConn is the connection name chosen in modePickConn.
	pickedConn string
}

// NewViewsListModel builds a fresh modal sized for the terminal.
func NewViewsListModel(items []ViewItem, width, height int) ViewsListModel {
	listItems := make([]list.Item, len(items))
	for i, v := range items {
		listItems[i] = v
	}

	l := buildList(listItems, width, height, "Saved views")

	return ViewsListModel{list: l, width: width, height: height}
}

// buildList constructs a list.Model sized to fit inside the modal box.
func buildList(items []list.Item, width, height int, title string) list.Model {
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

	l := list.New(items, delegate, boxW-4, boxH-6)
	l.Title = title
	l.Styles.Title = StyleTitle
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	if len(items) == 0 {
		l.SetShowFilter(false)
	}
	return l
}

// SetConnItems replaces the list content with a slice of ConnItems (modePickConn).
func (m *ViewsListModel) SetConnItems(names []string) {
	items := make([]list.Item, len(names))
	for i, n := range names {
		items[i] = ConnItem{Name: n}
	}
	m.list.SetItems(items)
	m.list.Title = "Copy from — pick a connection"
	m.list.SetShowFilter(len(names) > 0)
}

// SetViewItemsForConn replaces the list content with ViewItems for the
// chosen source connection (modePickView).
func (m *ViewsListModel) SetViewItemsForConn(connName string, items []ViewItem) {
	m.pickedConn = connName
	listItems := make([]list.Item, len(items))
	for i, v := range items {
		listItems[i] = v
	}
	m.list.SetItems(listItems)
	m.list.Title = "Copy from " + connName + " — pick a view"
	m.list.SetShowFilter(len(listItems) > 0)
}

// Update routes keys: Enter to run, D to delete, C to start copy-view flow,
// Esc to close/go-back, otherwise forwarded to the list.
func (m ViewsListModel) Update(msg tea.Msg) (ViewsListModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			switch m.mode {
			case modeViews:
				return m, func() tea.Msg { return CloseViewsMsg{} }
			case modePickConn:
				// Back to modeViews — App must reload view items.
				m.mode = modeViews
				return m, func() tea.Msg { return CloseViewsMsg{} }
			case modePickView:
				// Back to modePickConn.
				m.mode = modePickConn
				return m, func() tea.Msg { return CloseViewsMsg{} }
			}
		case "enter":
			switch m.mode {
			case modeViews:
				if item, ok := m.list.SelectedItem().(ViewItem); ok {
					return m, func() tea.Msg { return RunViewMsg{SQL: item.SQL} }
				}
			case modePickConn:
				if item, ok := m.list.SelectedItem().(ConnItem); ok {
					connName := item.Name
					m.mode = modePickView
					m.pickedConn = connName
					return m, func() tea.Msg { return PickConnSelectedMsg{ConnName: connName} }
				}
			case modePickView:
				if item, ok := m.list.SelectedItem().(ViewItem); ok {
					conn := m.pickedConn
					name := item.Name
					sql := item.SQL
					return m, func() tea.Msg {
						return CopyViewSelectedMsg{SourceConn: conn, ViewName: name, SQL: sql}
					}
				}
			}
		case "D":
			if m.mode == modeViews {
				if item, ok := m.list.SelectedItem().(ViewItem); ok {
					return m, func() tea.Msg { return DeleteViewMsg{Name: item.Name} }
				}
			}
		case "C":
			if m.mode == modeViews {
				// Transition to connection-picker mode. The App handles
				// pickConnModeMsg to populate the list with other connections.
				m.mode = modePickConn
				return m, func() tea.Msg { return EnterPickConnMsg{} }
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
	switch m.mode {
	case modeViews:
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
					StyleHelp.Render("Enter run · D delete · C copy from another connection · Esc close"),
			)
	case modePickConn:
		if len(m.list.Items()) == 0 {
			body = StyleDim.Render("(no other connections — add more connections first)")
		}
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(CtpSapphire).
			Padding(1, 2).
			Width(boxW).
			Render(
				body + "\n\n" +
					StyleHelp.Render("Enter pick · Esc back"),
			)
	case modePickView:
		if len(m.list.Items()) == 0 {
			body = StyleDim.Render("(no views saved in this connection)")
		}
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(CtpSapphire).
			Padding(1, 2).
			Width(boxW).
			Render(
				body + "\n\n" +
					StyleHelp.Render("Enter copy · Esc back"),
			)
	}
	return body
}

// ---------------------------------------------------------------------------
// Messages used between ViewsListModel and App
// ---------------------------------------------------------------------------

// EnterPickConnMsg is sent when the user presses C in modeViews. The App
// populates the connection list and calls SetConnItems on the ViewsListModel.
type EnterPickConnMsg struct{}

// PickConnSelectedMsg is sent when a connection is picked in modePickConn.
// The App loads that connection's views and calls SetViewItemsForConn.
type PickConnSelectedMsg struct{ ConnName string }

// ---------------------------------------------------------------------------
// SaveViewModel
// ---------------------------------------------------------------------------

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
	errMsg string // inline validation error (e.g. name collision)
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

// SetError displays an inline error message and keeps the modal open.
// Call after detecting a name collision to prevent the submit from closing
// the modal.
func (m *SaveViewModel) SetError(s string) { m.errMsg = s }

// HasError reports whether there is a pending inline error.
func (m SaveViewModel) HasError() bool { return m.errMsg != "" }

// SetPrefilledName pre-populates the name input field. Used by the copy-view
// flow to carry the source view's name into the save prompt.
func (m *SaveViewModel) SetPrefilledName(name string) {
	m.input.SetValue(name)
	m.input.CursorEnd()
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
		default:
			// Clear any inline error as soon as the user starts typing a new name.
			m.errMsg = ""
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

	body := StyleTitle.Render("Save view") + "\n\n" +
		StyleDim.Render("SQL:") + "\n" +
		HighlightSQL(preview) + "\n\n" +
		StyleDim.Render("Name:") + " " + m.input.View() + "\n"

	if m.errMsg != "" {
		body += "\n" + StyleError.Render(m.errMsg) + "\n"
	}

	body += "\n" + StyleHelp.Render("Enter save · Esc cancel")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(CtpMauve).
		Padding(1, 2).
		Width(boxW).
		Render(body)
}
