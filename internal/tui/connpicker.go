package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gabiito/zdb/internal/config"
)

// ConnectMsg is emitted when the user selects a connection.
type ConnectMsg struct {
	Conn config.Connection
}

// connItem wraps a config.Connection for the bubbles/list widget.
type connItem struct {
	conn config.Connection
}

func (c connItem) Title() string       { return c.conn.Name }
func (c connItem) Description() string { return c.conn.Engine + " · " + redactDSNForDisplay(c.conn.DSN) }
func (c connItem) FilterValue() string { return c.conn.Name }

// ConnPickerModel renders the connection picker.
type ConnPickerModel struct {
	list   list.Model
	width  int
	height int
}

// NewConnPickerModel creates a ConnPickerModel from a config.
func NewConnPickerModel(connections []config.Connection, width, height int) ConnPickerModel {
	items := make([]list.Item, len(connections))
	for i, c := range connections {
		items[i] = connItem{conn: c}
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(CtpPink).
		BorderForeground(CtpPink)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(CtpOverlay1).
		BorderForeground(CtpPink)

	l := list.New(items, delegate, listWidthFor(width), height-4)
	DisableListQuit(&l)
	l.Title = "zDB — select connection"
	l.Styles.Title = StyleTitle

	return ConnPickerModel{list: l, width: width, height: height}
}

// listWidthFor returns the width to give the list widget, leaving room
// for the right-hand logo panel when the terminal is wide enough.
func listWidthFor(total int) int {
	if total < LogoMinWidth {
		return total
	}
	return total - LogoPanelWidth
}

// Selected returns the currently highlighted connection, if any. The second
// return value is false when the list is empty.
func (m ConnPickerModel) Selected() (config.Connection, bool) {
	item, ok := m.list.SelectedItem().(connItem)
	if !ok {
		return config.Connection{}, false
	}
	return item.conn, true
}

// Init implements tea.Model.
func (m ConnPickerModel) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m ConnPickerModel) Update(msg tea.Msg) (ConnPickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			item, ok := m.list.SelectedItem().(connItem)
			if ok {
				return m, func() tea.Msg {
					return ConnectMsg{Conn: item.conn}
				}
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(listWidthFor(msg.Width), msg.Height-4)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m ConnPickerModel) View() string {
	listView := m.list.View()
	if m.width < LogoMinWidth {
		return listView
	}
	logo := RenderLogo(LogoPanelWidth, m.height-4)
	return lipgloss.JoinHorizontal(lipgloss.Top, listView, logo)
}

// redactDSNForDisplay shows only the host portion of a DSN.
func redactDSNForDisplay(dsn string) string {
	if len(dsn) > 30 {
		return dsn[:27] + "..."
	}
	return dsn
}
